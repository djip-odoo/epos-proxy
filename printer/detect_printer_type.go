package printer

import (
	"epos-proxy/logger"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/gousb"
)

var vidPidTypeMap = map[string]PrinterType{
	"2aaf:6015": TypeTHERMAL, // Essae thermal
	"04b8:0e32": TypeTHERMAL, // Epson thermal
	"2d84:c7c8": TypeTHERMAL, // Zhuhai Poskey Technology
	"4b43:3830": TypeTHERMAL, // Caysn CN811-UWB
}

var onlyAlphabetsRegex = regexp.MustCompile(`[^A-Z]+`)
var PDF_CMDS = map[string]struct{}{"PCL": {}, "PCLC": {}, "PCLXL": {}, "POSTSCRIPT": {}}
var EPOS_CMDS = map[string]struct{}{"ESCPOS": {}, "TSPL": {}, "ZPL": {}}

func isPrinterDevice(device *gousb.Device) (PrinterType, bool) {
	deviceID, isPrinter, _ := getPrinterDeviceID(device)

	printerType := _libUsbDetectPrinterType(deviceID)
	if isPrinter || strings.Contains(strings.ToUpper(deviceID["CLS"]), "PRINTER") {
		return printerType, true
	}

	if t := _detectByVidPid(fmt.Sprintf("%04X:%04X", uint16(device.Desc.Vendor), uint16(device.Desc.Product))); t != TypeANY {
		return t, true
	}

	if printerType != TypeANY {
		return printerType, true
	}

	return TypeANY, false
}

func _libUsbDetectPrinterType(deviceId DeviceID) PrinterType {
	cmds := _extractCMD(deviceId)
	for _, c := range cmds {
		if _, ok := PDF_CMDS[c]; ok {
			return TypeOFFICE
		}
	}

	for _, c := range cmds {
		if _, ok := EPOS_CMDS[c]; ok {
			return TypeTHERMAL
		}
	}

	logger.Debugf("CMD: %v, ID: %v", cmds, deviceId)
	return TypeANY
}

func _extractCMD(id DeviceID) []string {
	var result []string
	seen := make(map[string]bool)

	raw := id["CMD"]
	if raw == "" {
		return result
	}

	for _, c := range strings.Split(raw, ",") {
		n := onlyAlphabetsRegex.ReplaceAllString(
			strings.ToUpper(strings.TrimSpace(c)),
			"",
		)

		if n != "" && !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}

	return result
}

func _detectByVidPid(vidPid string) PrinterType {
	if t, ok := vidPidTypeMap[strings.ToLower(vidPid)]; ok {
		return t
	}

	logger.Debugf("Set any for VID:PID (%s)", vidPid)
	return TypeANY
}
