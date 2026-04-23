package printer

import (
	"epos-proxy/logger"
	"sort"
	"strings"

	"github.com/google/gousb"
)

var (
	lastSnapshot      string
	cachedPrinters    []LibUsbPrinter
	cachedUnavailable []UnavailableInfo
)

func buildSnapshot(descs map[string]gousb.DeviceDesc) string {
	keys := make([]string, 0, len(descs))
	for k := range descs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, "|")
}

func isDataChanged(descs map[string]gousb.DeviceDesc) bool {
	currentSnapshot := buildSnapshot(descs)
	logger.Infof("%v, %v", lastSnapshot, currentSnapshot)
	if lastSnapshot == currentSnapshot {
		return false
	}
	lastSnapshot = currentSnapshot
	return true
}
