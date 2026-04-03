package printer

import (
	"epos-proxy/logger"
	"strconv"
	"strings"

	"github.com/google/gousb"
)

// printerId shouldnot be vid or pid, because multiple printers can share those.
// Instead, we encode a combination of serial number (preferred), USB path (fallback),
// and CUPS name (optional) into a single string ID that can be used to reliably identify printers across sessions.
// fallback of serial number, vid and pid is same for same brand and model
func PathToString(desc *gousb.DeviceDesc) string {
	parts := make([]string, 0, len(desc.Path)+1)

	// Add bus first
	parts = append(parts, strconv.Itoa(desc.Bus))

	// Convert each path element
	for _, p := range desc.Path {
		parts = append(parts, strconv.Itoa(p))
	}
	logger.Debugf("parts: %s", strings.Join(parts, "."))
	return strings.Join(parts, ".")
}

func DetectPrinterType(data string) PrinterType {
	s := strings.ToLower(data)

	if strings.Contains(s, "pdf") {
		return TypePDF
	}

	thermalKeywords := []string{
		"thermal", "epos", "pos", "receipt", "tm",
		"epson", "star", "80mm", "58mm", "roll",
	}

	for _, k := range thermalKeywords {
		if strings.Contains(s, k) {
			return TypeEPOS
		}
	}

	return TypeANY
}
