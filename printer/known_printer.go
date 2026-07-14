package printer

import (
	"fmt"

	"github.com/google/gousb"
)

// Some thermal printers do not expose the standard USB printer class (0x07)
// and instead use vendor-specific interfaces. These known VID:PID pairs are
// treated as printers even when printer-class detection fails.
var knownPrinterVIDPID = map[string]struct{}{
	"2aaf:6015": {}, // Essae thermal
	"04b8:0e32": {}, // Epson thermal
	"04b8:0202": {}, // Epson thermal
	"04b8:0203": {}, // Epson thermal
	"2d84:c7c8": {}, // Zhuhai Poskey
	"4b43:3830": {}, // Caysn
}

func isKnownPrinter(desc *gousb.DeviceDesc) bool {
	vidPid := fmt.Sprintf("%04x:%04x", uint16(desc.Vendor), uint16(desc.Product))
	_, ok := knownPrinterVIDPID[vidPid]
	return ok
}
