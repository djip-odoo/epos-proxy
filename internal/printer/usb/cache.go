package usb

import (
	"sort"
	"strings"
	"sync"

	"epos-proxy/internal/printer"
)

type printerCache struct {
	mu             sync.RWMutex
	lastSnapshot   string
	cachedPrinters []printer.Device
}

var usbCache = &printerCache{}

func (c *printerCache) HasChanged(keys []string) bool {
	snap := buildSnapshot(keys)
	c.mu.RLock()
	defer c.mu.RUnlock()
	return snap != c.lastSnapshot
}

func (c *printerCache) Get() []printer.Device {
	c.mu.RLock()
	defer c.mu.RUnlock()

	printers := make([]printer.Device, len(c.cachedPrinters))
	copy(printers, c.cachedPrinters)

	return printers
}

func (c *printerCache) Update(keys []string, printers []printer.Device) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastSnapshot = buildSnapshot(keys)
	c.cachedPrinters = printers
}

func buildSnapshot(keys []string) string {
	sorted := make([]string, len(keys))
	copy(sorted, keys)
	sort.Strings(sorted)
	return strings.Join(sorted, "|")
}

func (c *printerCache) HasUnavailable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, p := range c.cachedPrinters {
		if p.ErrorMsg != "" {
			return true
		}
	}
	return false
}
