package config

func (cm *Manager) UpdateFirewallPreference(prompt bool, accepted bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.Data.FirewallPromptCompleted = prompt
	cm.Data.FirewallAccepted = accepted
	if prompt {
		cm.Data.OldPort = cm.Data.Port
	}
	return cm.saveLocked()
}

func (cm *Manager) IsFirewallAccepted() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.FirewallAccepted
}

func (cm *Manager) IsFirewallPromptCompleted() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.FirewallPromptCompleted
}
