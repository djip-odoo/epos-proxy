package usb

import (
	"fmt"
	"strings"

	"epos-proxy/internal/printer"

	"github.com/google/gousb"
)

// Some thermal printers do not expose the standard USB printer class (0x07)
// and instead use vendor-specific interfaces. These known VID:PID pairs are
// treated as printers even when printer-class detection fails.
var printerRegistry = map[string]printer.Type{
	// Receipt printers
	"2aaf:6015": printer.TypeReceipt, // Essae thermal
	"04b8:0e32": printer.TypeReceipt, // Epson thermal
	"04b8:0202": printer.TypeReceipt, // Epson thermal
	"04b8:0203": printer.TypeReceipt, // Epson thermal
	"04b8:0e27": printer.TypeReceipt, // Epson TM-T83III
	"2d84:c7c8": printer.TypeReceipt, // Zhuhai Poskey
	"4b43:3830": printer.TypeReceipt, // Caysn
	"0483:5720": printer.TypeReceipt, // STMicroelectronics

	// Label printers
	"0a5f:0187": printer.TypeLabel, // Zebra ZD421
	"195f:0001": printer.TypeLabel, // Godex G500
}

func isKnownPrinter(desc *gousb.DeviceDesc) bool {
	vidPid := strings.ToLower(
		fmt.Sprintf("%04x:%04x", uint16(desc.Vendor), uint16(desc.Product)),
	)
	_, ok := printerRegistry[vidPid]
	return ok
}

func isKnownVidPid(vidPid string) bool {
	_, ok := printerRegistry[strings.ToLower(vidPid)]
	return ok
}

func getPrinterType(vidPid string) printer.Type {
	if printerType, ok := printerRegistry[strings.ToLower(vidPid)]; ok {
		return printerType
	}
	return printer.TypeReceipt
}
