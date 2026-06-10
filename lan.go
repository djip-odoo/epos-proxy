package main

import (
	"epos-proxy/server"
)

// GetLANSettings returns the current LAN settings and firewall status.
func (a *App) GetLANSettings() server.LANSettings {
	return a.service.GetLANSettings()
}

// EnableLANAccess prompts for firewall elevation, adds the rule, updates config and restarts server.
func (a *App) EnableLANAccess() error {
	return a.service.EnableLANAccess()
}

// DisableLANAccess prompts for firewall elevation, removes the rule, updates config and restarts server.
func (a *App) DisableLANAccess() error {
	return a.service.DisableLANAccess()
}

// GetLocalIPAddress is a helper to just get the IP
func (a *App) GetLocalIPAddress() string {
	return a.service.GetLocalIPAddress()
}
