package util

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"epos-proxy/logger"
)

var ErrAuthCancelled = errors.New("authentication cancelled")

func setFirewallRule(port, oldPort int) error {
	return allowPortOS(port, oldPort)
}

func allowPortOS(port, oldPort int) error {
	logger.Infof("Updating UFW rules: old=%d new=%d", oldPort, port)

	if _, err := exec.LookPath("ufw"); err != nil {
		return fmt.Errorf("UFW is not installed. Install UFW or configure the firewall manually")
	}

	var commands []string

	if oldPort > 0 && oldPort != port {
		commands = append(commands,
			fmt.Sprintf("ufw delete allow %d/tcp || true", oldPort),
		)
	}

	commands = append(commands,
		fmt.Sprintf("ufw allow %d/tcp", port),
	)

	return runElevatedLinux(commands)
}

func runElevatedLinux(commands []string) error {
	if _, err := exec.LookPath("pkexec"); err == nil {
		return runWithPkexec(commands)
	}

	if _, err := exec.LookPath("kdesudo"); err == nil {
		return runWithKdesudo(commands)
	}

	return fmt.Errorf("no authorization agent found (pkexec or kdesudo)")
}

func runWithPkexec(commands []string) error {
	script := strings.Join(commands, " ; ")
	logger.Infof("Executing UFW commands with pkexec: %s", script)

	cmd := exec.Command("pkexec", "sh", "-c", script)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	outStr := strings.ToLower(string(output))

	if exitErr, ok := err.(*exec.ExitError); ok {
		switch exitErr.ExitCode() {
		case 126:
			return ErrAuthCancelled
		case 127:
			return fmt.Errorf("pkexec failed to execute shell")
		}
	}

	if strings.Contains(outStr, "authentication") ||
		strings.Contains(outStr, "cancel") ||
		strings.Contains(outStr, "dismissed") {
		return ErrAuthCancelled
	}

	return fmt.Errorf("pkexec failed: %w\n%s", err, output)
}

func runWithKdesudo(commands []string) error {
	script := strings.Join(commands, " ; ")
	logger.Infof("Executing UFW commands with kdesudo: %s", script)

	cmd := exec.Command("kdesudo", "--", "sh", "-c", script)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	outStr := strings.ToLower(string(output))

	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 126 {
		return ErrAuthCancelled
	}

	if strings.Contains(outStr, "authentication") ||
		strings.Contains(outStr, "cancel") ||
		strings.Contains(outStr, "dismissed") {
		return ErrAuthCancelled
	}

	return fmt.Errorf("kdesudo failed: %w\n%s", err, output)
}

func checkIfRuleExistOS() (bool, error) {
	// we dont want to check as it required UAC prompt which might annoys the user
	return false, nil
}
