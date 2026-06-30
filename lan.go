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
	ip := "127.0.0.1"
	if a.config.IsLANAccessEnabled() {
		if lanIP, err := util.GetLocalIP(); err == nil {
			ip = lanIP
		} else {
			logger.Errorf("Failed to get local IP: %v", err)
		}
	}

	return LANSettings{
		Enabled: a.config.IsLANAccessEnabled(),
		IP:      ip,
		Port:    a.config.GetPort(),
	}
}

func (a *App) EnableLANAccess() error {
	logger.Infof("Enabling LAN Access")
	port := a.config.GetPort()

	err := util.AllowPortThroughFirewall(port)
	if err != nil {
		logger.Errorf("Failed to allow port through firewall: %v", err)
		return fmt.Errorf("firewall error: %v", err)
	}

	a.config.SetLANAccess(true)
	if err := a.config.Save(); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		return fmt.Errorf("config error: %v", err)
	}

	a.RestartServer()
	return nil
}

func (a *App) DisableLANAccess() error {
	logger.Infof("Disabling LAN Access")
	port := a.config.GetPort()

	err := util.BlockPortThroughFirewall(port)
	if err != nil {
		logger.Errorf("Failed to block port through firewall: %v", err)
	}

	a.config.SetLANAccess(false)
	if err := a.config.Save(); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		return fmt.Errorf("config error: %v", err)
	}

	a.RestartServer()
	return nil
}
