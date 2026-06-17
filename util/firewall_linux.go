package util

import (
	"errors"
	"fmt"
	"os/exec"

	"epos-proxy/logger"
)

var ErrAuthCancelled = errors.New("authentication cancelled")

func allowPortOS(port int) error {
	logger.Infof("Allowing port %d through Linux firewall", port)

	return runFirewallCommand(
		fmt.Sprintf("ufw allow %d/tcp", port),
		fmt.Sprintf("firewall-cmd --add-port=%d/tcp --permanent && firewall-cmd --reload", port),
	)
}

func blockPortOS(port int) error {
	logger.Infof("Blocking port %d through Linux firewall", port)

	return runFirewallCommand(
		fmt.Sprintf("ufw delete allow %d/tcp", port),
		fmt.Sprintf("firewall-cmd --remove-port=%d/tcp --permanent && firewall-cmd --reload", port),
	)
}

func runFirewallCommand(ufwCmd, firewalldCmd string) error {
	// Try UFW first
	if _, err := exec.LookPath("ufw"); err == nil {
		if err := runElevatedLinux(ufwCmd); err == nil {
			return nil
		} else if errors.Is(err, ErrAuthCancelled) {
			return err
		} else {
			logger.Warnf("UFW failed, falling back to firewalld: %v", err)
		}
	}

	// Try firewalld
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		if err := runElevatedLinux(firewalldCmd); err == nil {
			return nil
		} else if errors.Is(err, ErrAuthCancelled) {
			return err
		} else {
			return fmt.Errorf("firewalld failed: %w", err)
		}
	}

	return fmt.Errorf("no supported firewall found (ufw or firewalld)")
}

func runElevatedLinux(command string) error {
	// pkexec (GNOME, most distros)
	if _, err := exec.LookPath("pkexec"); err == nil {
		return runWithPkexec(command)
	}

	// KDE
	if _, err := exec.LookPath("kdesudo"); err == nil {
		return runWithKdesudo(command)
	}

	return fmt.Errorf("no authorization agent found (pkexec or kdesudo)")
}

func runWithPkexec(command string) error {
	cmd := exec.Command("pkexec", "sh", "-c", command)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		switch exitErr.ExitCode() {
		case 126, 127:
			return ErrAuthCancelled
		}
	}

	return fmt.Errorf(
		"pkexec failed: %w\n%s",
		err,
		string(output),
	)
}

func runWithKdesudo(command string) error {
	cmd := exec.Command("kdesudo", "--", "sh", "-c", command)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		switch exitErr.ExitCode() {
		case 126, 127:
			return ErrAuthCancelled
		}
	}

	return fmt.Errorf(
		"kdesudo failed: %w\n%s",
		err,
		string(output),
	)
}
