package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const DefaultPrinterWidth = 576
const DefaultPrinterBottomPadding = 120

var ErrPrinterNotFound = errors.New("printer not found")

type PrinterSettingConfig struct {
	ID            string            `json:"id"`
	Width         int               `json:"width"`
	BottomPadding int               `json:"bottom_padding"`
	Protocol      string            `json:"protocol"`
	VidPid        string            `json:"vid_pid,omitempty"`
	DeviceID      map[string]string `json:"device_id,omitempty"`
}

var (
	printerConfigPath string
	printerConfigOnce sync.Once

	printerConfigs   map[string]PrinterSettingConfig
	printerConfigsMu sync.RWMutex
)

func initPrinterConfigPath() {
	printerConfigOnce.Do(func() {
		base, err := os.UserConfigDir()
		if err != nil {
			return
		}

		printerConfigPath = filepath.Join(base, AppName, "printer.json")
	})
}

func ensurePrinterConfigsLoaded() error {
	initPrinterConfigPath()

	printerConfigsMu.RLock()
	loaded := printerConfigs != nil
	printerConfigsMu.RUnlock()

	if loaded {
		return nil
	}

	printerConfigsMu.Lock()
	defer printerConfigsMu.Unlock()

	// Double-check after acquiring write lock.
	if printerConfigs != nil {
		return nil
	}

	return loadPrinterConfigsLocked()
}

func AddPrinterIfNotExist(id string, protocol string, vidpid string, deviceid map[string]string) error {
	if err := ensurePrinterConfigsLoaded(); err != nil {
		return err
	}

	printerConfigsMu.Lock()
	defer printerConfigsMu.Unlock()

	defaultCfg := PrinterSettingConfig{
		ID:            id,
		Width:         DefaultPrinterWidth,
		BottomPadding: DefaultPrinterBottomPadding,
		Protocol:      protocol,
		VidPid:        vidpid,
		DeviceID:      deviceid,
	}

	existing, exists := printerConfigs[id]
	if exists {
		changed := false

		if existing.Protocol != protocol {
			existing.Protocol = protocol
			changed = true
		}

		if existing.Width == 0 {
			existing.Width = DefaultPrinterWidth
			changed = true
		}

		if existing.BottomPadding == 0 {
			existing.BottomPadding = DefaultPrinterBottomPadding
			changed = true
		}

		if existing.VidPid != vidpid && vidpid != "" {
			existing.VidPid = vidpid
			changed = true
		}

		if deviceid != nil {
			existing.DeviceID = deviceid
			changed = true
		}

		if !changed {
			return nil
		}

		printerConfigs[id] = existing

		return savePrinterConfigsLocked()
	}

	printerConfigs[id] = defaultCfg

	return savePrinterConfigsLocked()
}

func SetPrinterSetting(id string, width int, bottomPadding int, protocol string) error {
	if width <= 0 {
		return errors.New("printer width must be greater than 0")
	}
	if bottomPadding < 0 {
		return errors.New("printer bottom padding must be greater than or equal to 0")
	}

	if err := ensurePrinterConfigsLoaded(); err != nil {
		return err
	}

	printerConfigsMu.Lock()
	defer printerConfigsMu.Unlock()

	cfg := printerConfigs[id]
	cfg.Width = width
	cfg.BottomPadding = bottomPadding
	cfg.Protocol = protocol
	cfg.ID = id

	printerConfigs[id] = cfg

	return savePrinterConfigsLocked()
}

func GetPrinterSetting(id string) PrinterSettingConfig {
	if err := ensurePrinterConfigsLoaded(); err != nil {
		return PrinterSettingConfig{Width: DefaultPrinterWidth, BottomPadding: DefaultPrinterBottomPadding, Protocol: "ESCPOS"}
	}

	printerConfigsMu.RLock()
	defer printerConfigsMu.RUnlock()

	cfg, exists := printerConfigs[id]
	if !exists {
		return PrinterSettingConfig{Width: DefaultPrinterWidth, BottomPadding: DefaultPrinterBottomPadding, Protocol: "ESCPOS"}
	}

	return cfg
}

func loadPrinterConfigsLocked() error {
	data, err := os.ReadFile(printerConfigPath)
	if os.IsNotExist(err) {
		printerConfigs = make(map[string]PrinterSettingConfig)
		return savePrinterConfigsLocked()
	}
	if err != nil {
		return err
	}

	var configs []PrinterSettingConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		printerConfigs = make(map[string]PrinterSettingConfig)

		_ = os.Remove(printerConfigPath)

		return savePrinterConfigsLocked()
	}

	printerConfigs = make(map[string]PrinterSettingConfig, len(configs))

	for _, cfg := range configs {
		if cfg.ID == "" || cfg.Width <= 0 {
			continue
		}

		printerConfigs[cfg.ID] = cfg
	}

	return nil
}

func savePrinterConfigsLocked() error {
	configs := make([]PrinterSettingConfig, 0, len(printerConfigs))

	for _, cfg := range printerConfigs {
		configs = append(configs, cfg)
	}

	data, err := json.Marshal(configs)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(printerConfigPath), 0755); err != nil {
		return err
	}

	tmpPath := printerConfigPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err == nil {
		if err := os.Rename(tmpPath, printerConfigPath); err == nil {
			return nil
		}
	}

	return os.WriteFile(printerConfigPath, data, 0644)
}
