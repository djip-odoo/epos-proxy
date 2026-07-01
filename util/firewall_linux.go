package util

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"epos-proxy/logger"
)

var ErrAuthCancelled = errors.New("authentication cancelled")

func allowApplicationOS() error {
	return nil
}

func allowPortOS(port int) error {
	logger.Infof("Allowing port %d through Linux firewall", port)

	if _, err := exec.LookPath("ufw"); err != nil {
		return fmt.Errorf(
			"UFW is not installed. Install UFW and try enabling network printing again",
		)
	}

	return runElevatedLinux([]string{
		"ufw",
		"allow",
		fmt.Sprintf("%d/tcp", port),
	})
}

func blockPortOS(port int) error {
	logger.Infof("Blocking port %d through Linux firewall", port)

	if _, err := exec.LookPath("ufw"); err != nil {
		return fmt.Errorf(
			"UFW is not installed. Install UFW and try enabling network printing again",
		)
	}

	return runElevatedLinux([]string{
		"ufw",
		"delete",
		"allow",
		fmt.Sprintf("%d/tcp", port),
	})
}

func runElevatedLinux(args []string) error {
	if _, err := exec.LookPath("pkexec"); err == nil {
		return runWithPkexec(args)
	}

	if _, err := exec.LookPath("kdesudo"); err == nil {
		return runWithKdesudo(args)
	}

	return fmt.Errorf("no authorization agent found (pkexec or kdesudo)")
}

func runWithPkexec(args []string) error {
	cmdArgs := append([]string{"pkexec"}, args...)
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	outStr := strings.ToLower(string(output))
	if strings.Contains(outStr, "error executing command as another user:") {
		if strings.Contains(outStr, "no such file") ||
			strings.Contains(outStr, "command not found") ||
			strings.Contains(outStr, "cannot find") {
			return fmt.Errorf("command %q not found", args[0])
		}
		return ErrAuthCancelled
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 126 {
			return ErrAuthCancelled
		}
	}

	return fmt.Errorf("pkexec failed: %w\n%s", err, string(output))
}

func runWithKdesudo(args []string) error {
	cmdArgs := append([]string{"kdesudo", "--"}, args...)
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	outStr := strings.ToLower(string(output))
	if strings.Contains(outStr, "cancel") ||
		strings.Contains(outStr, "dismissed") {
		return ErrAuthCancelled
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 126 {
			return ErrAuthCancelled
		}
	}

	return fmt.Errorf("kdesudo failed: %w\n%s", err, string(output))
}
