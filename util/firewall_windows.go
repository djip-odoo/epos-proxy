package util

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"epos-proxy/logger"
)

var ErrAuthCancelled = errors.New("authentication cancelled")

func allowPortOS(port int) error {
	ruleName := fmt.Sprintf("EPOS Proxy LAN Port %d", port)

	logger.Infof("Attempting to allow port %d through Windows firewall", port)

	args := fmt.Sprintf(
		`advfirewall firewall add rule name="%s" dir=in action=allow protocol=TCP localport=%d profile=any`,
		ruleName,
		port,
	)

	return runElevated("netsh.exe", args)
}

func blockPortOS(port int) error {
	ruleName := fmt.Sprintf("EPOS Proxy LAN Port %d", port)

	logger.Infof("Attempting to remove firewall rule for port %d", port)

	args := fmt.Sprintf(
		`advfirewall firewall delete rule name="%s"`,
		ruleName,
	)

	return runElevated("netsh.exe", args)
}

func runElevated(exe string, args string) error {
	logger.Infof("Running elevated command: %s %s", exe, args)

	psCmd := fmt.Sprintf(`
$proc = Start-Process -FilePath '%s' ` +
		`-ArgumentList '%s' ` +
		`-Verb RunAs ` +
		`-Wait ` +
		`-PassThru;
$proc.ExitCode
`,
		escapePS(exe),
		escapePS(args),
	)

	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		psCmd,
	)

	output, err := cmd.CombinedOutput()

	if err != nil {
		out := strings.TrimSpace(string(output))

		// User pressed "No" on UAC
		if strings.Contains(out, "1223") ||
			strings.Contains(out, "The operation was canceled by the user") {
			return ErrAuthCancelled
		}

		return fmt.Errorf(
			"failed to execute elevated command: %w\n%s",
			err,
			out,
		)
	}

	exitCode := strings.TrimSpace(string(output))

	if exitCode != "" && exitCode != "0" {
		return fmt.Errorf(
			"elevated process exited with code %s",
			exitCode,
		)
	}

	logger.Info("Elevated command completed successfully")
	return nil
}

func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
