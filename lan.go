package main

import (
	"epos-proxy/logger"
	"epos-proxy/util"
	"fmt"
)

type LANSettings struct {
	Enabled bool   `json:"enabled"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
}

func (a *App) GetLANSettings() LANSettings {
	ip := "127.0.0.1"
	if a.config.Data.LANAccessEnabled {
		if lanIp, err := util.GetLocalIP(); err == nil {
			ip = lanIp
		} else {
			logger.Errorf("Failed to get local IP: %v", err)
		}
	}

	port := a.config.Data.Port

	return LANSettings{
		Enabled: a.config.Data.LANAccessEnabled,
		IP:      ip,
		Port:    port,
	}
}

func (a *App) EnableLANAccess() error {
	logger.Infof("Enabling LAN Access")
	port := a.config.Data.Port

	err := util.AllowPortThroughFirewall(port)
	if err != nil {
		logger.Errorf("Failed to allow port through firewall: %v", err)
		return fmt.Errorf("firewall error: %v", err)
	}

	a.config.Data.LANAccessEnabled = true
	if err := a.config.Save(); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		return fmt.Errorf("config error: %v", err)
	}

	a.RestartServer()
	return nil
}

func (a *App) DisableLANAccess() error {
	logger.Infof("Disabling LAN Access")
	port := a.config.Data.Port

	err := util.BlockPortThroughFirewall(port)
	if err != nil {
		logger.Errorf("Failed to block port through firewall: %v", err)
	}

	a.config.Data.LANAccessEnabled = false
	if err := a.config.Save(); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		return fmt.Errorf("config error: %v", err)
	}

	a.RestartServer()
	return nil
}
