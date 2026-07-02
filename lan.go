package main

import (
	"fmt"

	"epos-proxy/logger"
	"epos-proxy/util"
)

type LANSettings struct {
	Enabled bool   `json:"enabled"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
}

func (a *App) GetLANSettings() LANSettings {
	return LANSettings{
		Enabled: a.config.IsFirewallAccepted(),
		IP:      util.GetLocalIP(),
		Port:    a.config.GetPort(),
	}
}

func (a *App) ConfigureFirewall() error {
	logger.Infof("Configuring firewall")
	err := util.SetFirewallRule(a.config.GetPort(), a.config.GetOldPort())
	if err != nil {
		if err == util.ErrAuthCancelled {
			logger.Warnf("Firewall configuration cancelled by user")
			if err := a.config.UpdateFirewallPreference(true, false); err != nil {
				logger.Errorf("Failed to save config: %v", err)
				return fmt.Errorf("config error: %v", err)
			}
			return err
		}
		logger.Errorf("Failed to configure firewall: %v", err)
		return err
	}

	logger.Infof("Firewall configured successfully")
	if err := a.config.UpdateFirewallPreference(true, true); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		return fmt.Errorf("config error: %v", err)
	}

	return nil
}

func (a *App) SkipFirewallPrompt() error {
	logger.Infof("User skipped firewall prompt ('Not Now')")
	if err := a.config.UpdateFirewallPreference(true, false); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		return fmt.Errorf("config error: %v", err)
	}
	return nil
}
