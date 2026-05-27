package config

import (
	"encoding/json"
	"epos-proxy/logger"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const DefaultPrinterWidth = 576

var ErrPrinterNotFound = errors.New("printer not found")

type PrinterWidthConfig struct {
	ID       string `json:"id"`
	Width    int    `json:"width"`
	Protocol string `json:"protocol"`
}

var (
	printerConfigPath string
	printerConfigOnce sync.Once

	printerConfigs   map[string]PrinterWidthConfig
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

func AddPrinterIfNotExist(id string, protocol string) error {
	if err := ensurePrinterConfigsLoaded(); err != nil {
		return err
	}

	printerConfigsMu.Lock()
	defer printerConfigsMu.Unlock()

	if _, exists := printerConfigs[id]; exists {
		return nil
	}

	printerConfigs[id] = PrinterWidthConfig{
		ID:       id,
		Width:    DefaultPrinterWidth,
		Protocol: protocol,
	}

	return savePrinterConfigsLocked()
}

func SetPrinterWidth(id string, width int) error {
	if width <= 0 {
		return errors.New("printer width must be greater than 0")
	}

	if err := ensurePrinterConfigsLoaded(); err != nil {
		return err
	}

	printerConfigsMu.Lock()
	defer printerConfigsMu.Unlock()

	cfg := printerConfigs[id]
	cfg.Width = width
	cfg.ID = id

	if cfg.Protocol == "" {
		cfg.Protocol = "ESCPOS"
	}

	printerConfigs[id] = cfg
	logger.Infof("Printer %s width updated to %d with protocol %s", id, width, cfg.Protocol)

	return savePrinterConfigsLocked()
}

func GetPrinterWidth(id string) int {
	if err := ensurePrinterConfigsLoaded(); err != nil {
		logger.Warnf("Could not load printer configs: %v", err)
		return DefaultPrinterWidth
	}

	printerConfigsMu.RLock()
	defer printerConfigsMu.RUnlock()

	cfg, exists := printerConfigs[id]
	if !exists {
		logger.Warnf("Printer %s not found, using default width %d", id, DefaultPrinterWidth)
		return DefaultPrinterWidth
	}

	return cfg.Width
}

func loadPrinterConfigsLocked() error {
	data, err := os.ReadFile(printerConfigPath)
	if os.IsNotExist(err) {
		printerConfigs = make(map[string]PrinterWidthConfig)
		return savePrinterConfigsLocked()
	}
	if err != nil {
		return err
	}

	var configs []PrinterWidthConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		printerConfigs = make(map[string]PrinterWidthConfig)

		_ = os.Remove(printerConfigPath)

		return savePrinterConfigsLocked()
	}

	printerConfigs = make(map[string]PrinterWidthConfig, len(configs))

	for _, cfg := range configs {
		if cfg.ID == "" || cfg.Width <= 0 {
			continue
		}

		printerConfigs[cfg.ID] = cfg
	}

	return nil
}

func savePrinterConfigsLocked() error {
	configs := make([]PrinterWidthConfig, 0, len(printerConfigs))

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
