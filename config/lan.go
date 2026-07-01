package config

import "runtime"

func (cm *Manager) IsFirewallPromptCompleted() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.Data.FirewallPromptCompleted
}

func (cm *Manager) SetFirewallPromptCompleted(completed bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.Data.FirewallPromptCompleted = completed
	return cm.saveLocked()
}

func (cm *Manager) IsFirewallAccepted() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.Data.FirewallAccepted
}

func (cm *Manager) SetFirewallAccepted(accepted bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.Data.FirewallAccepted = accepted
	return cm.saveLocked()
}

func (cm *Manager) GetPort() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.Data.Port
}

func (cm *Manager) CheckPortChange(port int) error {
	if runtime.GOOS == "linux" && cm.IsFirewallAccepted() && cm.GetPort() > 0 && cm.GetPort() != port {
		if err := cm.SetFirewallAccepted(false); err != nil {
			return err
		}
		if err := cm.SetFirewallPromptCompleted(false); err != nil {
			return err
		}
	}
	return nil
}
