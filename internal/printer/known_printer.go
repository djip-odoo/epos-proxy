package printer

import (
	"fmt"
	"strings"
	"sync"

	"epos-proxy/internal/config"
	"epos-proxy/internal/logger"

	"github.com/google/gousb"
)

// Type is what a printer produces, which decides how job data is framed.
type Type string

const (
	TypeReceipt Type = "receipt"
	TypeLabel   Type = "label"
)

type registryEntry struct {
	typ  Type
	name string
}

var (
	registryMu      sync.RWMutex
	printerRegistry map[string]registryEntry
)

func initKnownPrinterRegistry(printers []config.KnownPrinter) {
	registryMu.Lock()
	defer registryMu.Unlock()

	reg := make(map[string]registryEntry, len(printers))
	for _, p := range printers {
		pt := Type(p.Type)
		if pt == "" {
			pt = TypeReceipt
		} else if pt != TypeLabel && pt != TypeReceipt {
			logger.Warnf("Assigning Default type for known printer %s (%s:%s) with invalid type %s", p.Name, p.VID, p.PID, p.Type)
			pt = TypeReceipt
		}
		reg[vidPidKey(p.VID, p.PID)] = registryEntry{typ: pt, name: p.Name}
	}
	printerRegistry = reg
}

func vidPidKey(vid, pid string) string {
	return strings.ToLower(fmt.Sprintf("%s:%s", vid, pid))
}

func lookup(vidPid string) (registryEntry, bool) {
	registryMu.RLock()
	entry, ok := printerRegistry[strings.ToLower(vidPid)]
	registryMu.RUnlock()
	return entry, ok
}

func isKnownPrinter(desc *gousb.DeviceDesc) bool {
	vidPid := vidPidKey(fmt.Sprintf("%04x", uint16(desc.Vendor)), fmt.Sprintf("%04x", uint16(desc.Product)))
	_, ok := lookup(vidPid)
	return ok
}

func getPrinterType(vidPid string) Type {
	if entry, ok := lookup(vidPid); ok {
		return entry.typ
	}
	return TypeReceipt
}

func getKnownPrinterName(vidPid string) string {
	if entry, ok := lookup(vidPid); ok {
		return entry.name
	}
	return ""
}
