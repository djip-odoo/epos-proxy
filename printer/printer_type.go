package printer

import (
	"fmt"
	"strings"

	"github.com/google/gousb"
)

var escPosVidPid = map[string]struct{}{
	"2aaf:6015": {}, // Essae thermal
	"04b8:0e32": {}, // Epson thermal
	"04b8:0202": {}, // Epson thermal
	"04b8:0203": {}, // Epson thermal
	"2d84:c7c8": {}, // Zhuhai Poskey
	"4b43:3830": {}, // Caysn
}

func isPrinterDevice(device *gousb.Device) (bool, PrinterProtocol) {
	deviceID, usbPrinterClass, _ := getPrinterDeviceID(device)
	isPrinter := usbPrinterClass || strings.Contains(deviceID["CLS"], "PRINTER")
	isEscPos := deviceID["CMD"] == "ESCPOS"

	vidPid := fmt.Sprintf("%04x:%04x", uint16(device.Desc.Vendor), uint16(device.Desc.Product))
	if _, ok := escPosVidPid[vidPid]; ok {
		isEscPos = true
	}

	if isEscPos {
		return isPrinter, ProtocolESCPOS
	}

	return isPrinter, ProtocolUnknown
}
