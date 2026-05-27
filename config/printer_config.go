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
	ID    string `json:"id"`
	Width int    `json:"width"`
}

var (
	printerConfigPath string
	printerConfigOnce sync.Once

	printerConfigs   map[string]int
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

func AddPrinterIfNotExist(id string) error {
	if err := ensurePrinterConfigsLoaded(); err != nil {
		return err
	}

	printerConfigsMu.Lock()
	defer printerConfigsMu.Unlock()

	if _, exists := printerConfigs[id]; exists {
		return nil
	}

	printerConfigs[id] = DefaultPrinterWidth

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

	printerConfigs[id] = width

	return savePrinterConfigsLocked()
}

func GetPrinterWidth(id string) (int) {
	if err := ensurePrinterConfigsLoaded(); err != nil {
		logger.Warnf("Could not load printer configs: %v", err)
		return DefaultPrinterWidth
	}

	printerConfigsMu.RLock()
	defer printerConfigsMu.RUnlock()

	width, exists := printerConfigs[id]
	if !exists {
		logger.Warnf("Printer %s not found, using default width %d", id, DefaultPrinterWidth)
		return DefaultPrinterWidth
	}

	return width
}

func loadPrinterConfigsLocked() error {
	data, err := os.ReadFile(printerConfigPath)
	if os.IsNotExist(err) {
		printerConfigs = make(map[string]int)
		return nil
	}
	if err != nil {
		return err
	}

	var configs []PrinterWidthConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return err
	}

	printerConfigs = make(map[string]int, len(configs))

	for _, cfg := range configs {
		if cfg.ID == "" || cfg.Width <= 0 {
			continue
		}

		printerConfigs[cfg.ID] = cfg.Width
	}

	return nil
}

func savePrinterConfigsLocked() error {
	configs := make([]PrinterWidthConfig, 0, len(printerConfigs))

	for id, width := range printerConfigs {
		configs = append(configs, PrinterWidthConfig{
			ID:    id,
			Width: width,
		})
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(printerConfigPath), 0755); err != nil {
		return err
	}

	tmpPath := printerConfigPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, printerConfigPath)
}