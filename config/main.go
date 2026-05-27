package config

import (
	"log"
	"sync"
)

var (
	manager *Manager
	once    sync.Once
)

func Get() *Manager {
	once.Do(func() {
		cfg, err := NewManager()
		if err != nil {
			log.Printf("config init error: %v", err)
			return
		}

		if err := cfg.Load(); err != nil {
			log.Printf("config load warning: %v", err)
		}

		manager = cfg
	})

	return manager
}
