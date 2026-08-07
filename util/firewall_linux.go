package util

import (
	"epos-proxy/logger"
	"fmt"
	"os/exec"
)

func allowPortOS(port int) error {
	logger.Infof("Attempting to allow port %d through Linux firewall", port)

	// Try UFW first
	if _, err := exec.LookPath("ufw"); err == nil {
		cmdStr := fmt.Sprintf("ufw allow %d/tcp", port)
		err := runElevatedLinux(cmdStr)
		if err != nil {
			logger.Errorf("UFW allow failed: %v", err)
			// fallback to firewalld
		} else {
			return nil
		}
	}

	// Try firewalld
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		cmdStr := fmt.Sprintf("firewall-cmd --add-port=%d/tcp --permanent && firewall-cmd --reload", port)
		err := runElevatedLinux(cmdStr)
		if err != nil {
			return fmt.Errorf("firewall-cmd failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no supported firewall found (ufw or firewalld)")
}

func blockPortOS(port int) error {
	logger.Infof("Attempting to block port %d through Linux firewall", port)

	if _, err := exec.LookPath("ufw"); err == nil {
		cmdStr := fmt.Sprintf("ufw delete allow %d/tcp", port)
		err := runElevatedLinux(cmdStr)
		if err != nil {
			logger.Errorf("UFW block failed: %v", err)
		} else {
			return nil
		}
	}

	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		cmdStr := fmt.Sprintf("firewall-cmd --remove-port=%d/tcp --permanent && firewall-cmd --reload", port)
		err := runElevatedLinux(cmdStr)
		if err != nil {
			return fmt.Errorf("firewall-cmd failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no supported firewall found (ufw or firewalld)")
}

func runElevatedLinux(command string) error {
	// Try pkexec
	if _, err := exec.LookPath("pkexec"); err == nil {
		cmd := exec.Command("pkexec", "sh", "-c", command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("pkexec error: %w, output: %s", err, string(out))
		}
		return nil
	}

	// Try kdesudo
	if _, err := exec.LookPath("kdesudo"); err == nil {
		cmd := exec.Command("kdesudo", "--", "sh", "-c", command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("kdesudo error: %w, output: %s", err, string(out))
		}
		return nil
	}

	return fmt.Errorf("no polkit authorization agent found (e.g. pkexec)")
}
