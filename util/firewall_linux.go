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
	logger.Infof("Allowing port %d through Linux firewall", port)

	portTCP := fmt.Sprintf("%d/tcp", port)
	return runFirewallCommand(
		[]string{"ufw", "allow", portTCP},
		[]string{"firewall-cmd", fmt.Sprintf("--add-port=%s", portTCP), "--permanent"},
		[]string{"firewall-cmd", "--reload"},
	)
}

func blockPortOS(port int) error {
	logger.Infof("Blocking port %d through Linux firewall", port)

	portTCP := fmt.Sprintf("%d/tcp", port)
	return runFirewallCommand(
		[]string{"ufw", "delete", "allow", portTCP},
		[]string{"firewall-cmd", fmt.Sprintf("--remove-port=%s", portTCP), "--permanent"},
		[]string{"firewall-cmd", "--reload"},
	)
}

func runFirewallCommand(ufwArgs, firewalldArgs, firewalldReloadArgs []string) error {
	if _, err := exec.LookPath("ufw"); err == nil {
		if err := runElevatedLinux(ufwArgs); err == nil {
			return nil
		} else if errors.Is(err, ErrAuthCancelled) {
			return err
		} else {
			logger.Warnf("UFW failed, falling back to firewalld: %v", err)
		}
	}

	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		if err := runElevatedLinux(firewalldArgs); err != nil {
			if errors.Is(err, ErrAuthCancelled) {
				return err
			}
			return fmt.Errorf("firewalld failed: %w", err)
		}
		if err := runElevatedLinux(firewalldReloadArgs); err != nil {
			if errors.Is(err, ErrAuthCancelled) {
				return err
			}
			return fmt.Errorf("firewalld reload failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no supported firewall manager found (tried ufw, firewalld); " +
		"if using iptables directly, add a rule manually: iptables -A INPUT -p tcp --dport <port> -j ACCEPT")
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
		if strings.Contains(outStr, "no such file") || strings.Contains(outStr, "command not found") || strings.Contains(outStr, "cannot find") {
			return fmt.Errorf("pkexec: command not found %q — is %s installed?", args[0], args[0])
		}
		return ErrAuthCancelled
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 126 {
			return ErrAuthCancelled
		}
	}

	return fmt.Errorf(
		"pkexec failed: %w\n%s",
		err,
		string(output),
	)
}

func runWithKdesudo(args []string) error {
	cmdArgs := append([]string{"kdesudo", "--"}, args...)
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	outStr := strings.ToLower(string(output))
	if strings.Contains(outStr, "cancel") || strings.Contains(outStr, "dismissed") {
		return ErrAuthCancelled
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 126 {
			return ErrAuthCancelled
		}
	}

	return fmt.Errorf(
		"kdesudo failed: %w\n%s",
		err,
		string(output),
	)
}
