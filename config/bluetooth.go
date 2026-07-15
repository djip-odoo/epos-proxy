package config

func (cm *Manager) AddBluetoothPrinter(mac, name, connType string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, existing := range cm.Data.BluetoothPrinters {
		if existing.MAC == mac {
			cm.Data.BluetoothPrinters[i].Name = name
			cm.Data.BluetoothPrinters[i].Type = connType
			return cm.saveLocked()
		}
	}
	cm.Data.BluetoothPrinters = append(cm.Data.BluetoothPrinters, BluetoothPrinterConfig{
		MAC:  mac,
		Name: name,
		Type: connType,
	})
	return cm.saveLocked()
}

func (cm *Manager) RemoveBluetoothPrinter(mac string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, existing := range cm.Data.BluetoothPrinters {
		if existing.MAC == mac {
			cm.Data.BluetoothPrinters = append(cm.Data.BluetoothPrinters[:i], cm.Data.BluetoothPrinters[i+1:]...)
			return cm.saveLocked()
		}
	}
	return nil // Not found, nothing to remove
}

func (cm *Manager) GetBluetoothPrinters() []BluetoothPrinterConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.Data.BluetoothPrinters == nil {
		return []BluetoothPrinterConfig{}
	}
	result := make([]BluetoothPrinterConfig, len(cm.Data.BluetoothPrinters))
	copy(result, cm.Data.BluetoothPrinters)
	return result
}
