package printer

import (
	"fmt"
	"strings"

	"github.com/google/gousb"
)

var printerVidPid = map[string]struct{}{
	"2aaf:6015": {}, // Essae thermal
	"04b8:0e32": {}, // Epson thermal
	"04b8:0202": {}, // Epson thermal
	"04b8:0203": {}, // Epson thermal
	"2d84:c7c8": {}, // Zhuhai Poskey
	"4b43:3830": {}, // Caysn
	"0483:5720": {}, // STMicroelectronics
}

func isPrinterDevice(device *gousb.Device) (bool, PrinterProtocol) {
	deviceID, usbPrinterClass, _ := getPrinterDeviceID(device)
	isPrinter := usbPrinterClass || strings.Contains(deviceID["CLS"], "PRINTER")

	vidPid := fmt.Sprintf("%04x:%04x", uint16(device.Desc.Vendor), uint16(device.Desc.Product))
	if _, ok := printerVidPid[vidPid]; ok {
		isPrinter = true
	}

	if deviceID.hasCommand("ESCPOS") {
		return isPrinter, ProtocolESCPOS
	}

	if deviceID.hasCommand("TSPL") {
		return isPrinter, ProtocolTSPL
	}

	return isPrinter, ProtocolUnknown
}
