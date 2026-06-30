package config

import "log"

func InitConfig() *Manager {
	cfg, err := NewManager()
	if err != nil {
		log.Printf("config load error: %v", err)
		return nil
	}

	if err := cfg.Load(); err != nil {
		log.Printf("config load warning: %v", err)
	}

	return cfg
}
