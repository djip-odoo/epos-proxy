package config

func (cm *Manager) SetLANAccess(enabled bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.Data.LANAccessEnabled = enabled
	return cm.saveLocked()
}

func (cm *Manager) IsLANAccessEnabled() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.Data.LANAccessEnabled
}

func (cm *Manager) GetPort() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.Data.Port
}
