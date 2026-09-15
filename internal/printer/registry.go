package printer

import (
	"sync"

	"epos-proxy/internal/config"
)

// DriverFactory is a function that creates a Driver given the application configuration.
type DriverFactory func(cfg *config.Manager) Driver

type driverEntry struct {
	name    string
	factory DriverFactory
}

var (
	registryMu      sync.RWMutex
	driverFactories []driverEntry
)

// RegisterDriver registers a driver factory by name. If a driver with the same
// name is already registered, its factory is updated.
func RegisterDriver(name string, factory DriverFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()

	for i, entry := range driverFactories {
		if entry.name == name {
			driverFactories[i].factory = factory
			return
		}
	}
	driverFactories = append(driverFactories, driverEntry{name: name, factory: factory})
}

// creates a Manager populated with all registered drivers.
func RegistryManager(cfg *config.Manager) *Manager {
	registryMu.RLock()
	defer registryMu.RUnlock()

	mgr := NewManager()
	for _, entry := range driverFactories {
		if d := entry.factory(cfg); d != nil {
			mgr.RegisterDriver(d)
		}
	}
	return mgr
}

// ResetDriversForTesting resets the registered drivers list (used in unit tests).
func ResetDriversForTesting() {
	registryMu.Lock()
	defer registryMu.Unlock()
	driverFactories = nil
}
