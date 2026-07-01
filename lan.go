package main

import (
	"fmt"
	"runtime"

	"epos-proxy/logger"
	"epos-proxy/util"
)

type LANSettings struct {
	Enabled bool   `json:"enabled"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
}

func (a *App) GetLANSettings() LANSettings {
	ip := "127.0.0.1"
	if lanIP, err := util.GetLocalIP(); err == nil {
		ip = lanIP
	} else {
		logger.Errorf("Failed to get local network IP: %v", err)
	}
	return LANSettings{
		Enabled: a.config.IsFirewallAccepted(),
		IP:      ip,
		Port:    a.config.GetPort(),
	}
}

func (a *App) ConfigureFirewall() error {
	logger.Infof("Configuring firewall")
	port := a.config.GetPort()

	var err error
	switch runtime.GOOS {
	case "windows":
		err = util.AllowApplicationThroughFirewall()
	case "linux":
		err = util.AllowPortThroughFirewall(port)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err != nil {
		if err == util.ErrAuthCancelled {
			logger.Warnf("Firewall configuration cancelled by user")
			a.config.SetFirewallPromptCompleted(true)
			a.config.SetFirewallAccepted(false)
			return err
		}
		logger.Errorf("Failed to configure firewall: %v", err)
		return err
	}

	logger.Infof("Firewall configured successfully")
	a.config.SetFirewallPromptCompleted(true)
	a.config.SetFirewallAccepted(true)
	return nil
}

func (a *App) SkipFirewallPrompt() error {
	logger.Infof("User skipped firewall prompt ('Not Now')")
	a.config.SetFirewallPromptCompleted(true)
	a.config.SetFirewallAccepted(false)
	return nil
}

func (a *App) IsFirewallPromptCompleted() bool {
	return a.config.IsFirewallPromptCompleted()
}
