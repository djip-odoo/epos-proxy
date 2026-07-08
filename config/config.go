package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const AppName = "EposProxy"

const (
	PortRangeStart = 4545
	PortRangeEnd   = 4555
)

type AppConfig struct {
	Port                    int      `json:"port"`
	LANPrinters             []string `json:"lan_printers,omitempty"`
	FirewallPromptCompleted bool     `json:"firewall_prompt_completed"`
	FirewallAccepted        bool     `json:"firewall_accepted"`
	OldPort                 int      `json:"old_port"`
	OS                      string   `json:"os"`
	Arch                    string   `json:"arch"`
	SupportMode             bool     `json:"support_mode"`
}

func defaults() AppConfig {
	return AppConfig{
		Port:                    0,
		OldPort:                 0,
		FirewallPromptCompleted: false,
		FirewallAccepted:        false,
		OS:                      runtime.GOOS,
		Arch:                    runtime.GOARCH,
		SupportMode:             false,
	}
}

type Manager struct {
	mu   sync.RWMutex
	path string
	Data AppConfig
}

func NewManager() (*Manager, error) {
	var base string
	var err error
	if runtime.GOOS == "android" {
		base = "/data/data/com.wails.app/files"
	} else {
		base, err = os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("cannot locate user config dir: %w", err)
		}
	}

	dir := filepath.Join(base, AppName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create config dir: %w", err)
	}

	return &Manager{
		path: filepath.Join(dir, "config.json"),
		Data: defaults(),
	}, nil
}

func (cm *Manager) HasArgs(args ...string) bool {
	for _, arg := range args {
		for _, osArg := range os.Args[1:] {
			if osArg == arg {
				return true
			}
		}
	}

	return false
}

func (cm *Manager) ConfigDirectory() string {
	var base string
	var err error
	if runtime.GOOS == "android" {
		base = "/data/data/com.wails.app/files"
	} else {
		base, err = os.UserConfigDir()
		if err != nil {
			return ""
		}
	}
	return filepath.Join(base, AppName)
}

func (cm *Manager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(cm.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("config read error: %w", err)
	}

	if err := json.Unmarshal(data, &cm.Data); err != nil {
		return fmt.Errorf("config parse error: %w", err)
	}
	return nil
}

func (cm *Manager) Save() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.saveLocked()
}

func (cm *Manager) saveLocked() error {
	data, err := json.MarshalIndent(cm.Data, "", "  ")
	if err != nil {
		return fmt.Errorf("config marshal error: %w", err)
	}
	if err := os.WriteFile(cm.path, data, 0644); err != nil {
		return fmt.Errorf("config write error: %w", err)
	}
	return nil
}

func (cm *Manager) Path() string { return cm.path }

func (cm *Manager) SetSupportMode(enabled bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.SupportMode = enabled
	return cm.saveLocked()
}
