package util

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"epos-proxy/logger"
)

var ErrAuthCancelled = errors.New("authentication cancelled")

const firewallRuleName = "EPOS Proxy LAN Access"

func allowApplicationOS() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	logger.Infof("Attempting to allow executable %s through Windows firewall", exePath)

	// Best effort delete first to avoid duplicate rules
	deleteArgs := fmt.Sprintf(`advfirewall firewall delete rule name="%s"`, firewallRuleName)
	_ = runElevated("netsh.exe", deleteArgs)

	addArgs := fmt.Sprintf(
		`advfirewall firewall add rule name="%s" dir=in action=allow program="%s" enable=yes profile=private,domain`,
		firewallRuleName,
		exePath,
	)
	return runElevated("netsh.exe", addArgs)
}

func allowPortOS(port int) error {
	return fmt.Errorf("allowPortOS should not be called on Windows; use allowApplicationOS instead")
}

func blockPortOS(port int) error {
	return fmt.Errorf("blockPortOS should not be called on Windows; use blockApplicationOS instead")
}

func runElevated(exe, args string) error {
	logger.Infof("Running elevated command: %s %s", exe, args)

	psCmd := fmt.Sprintf(`
$proc = Start-Process -FilePath '%s' -ArgumentList '%s' -Verb RunAs -Wait -PassThru
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

		if strings.Contains(out, "1223") ||
			strings.Contains(out, "The operation was canceled by the user") {
			return ErrAuthCancelled
		}

		return fmt.Errorf("failed to execute elevated command: %w\n%s", err, out)
	}

	exitCode := strings.TrimSpace(string(output))
	if exitCode != "" && exitCode != "0" {
		return fmt.Errorf("elevated process exited with code %s", exitCode)
	}

	logger.Info("Elevated command completed successfully")
	return nil
}

func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
