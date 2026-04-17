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

func detectPrinterType(vidPid string, Type PrinterType) PrinterType {
	if Type != TypeANY {
		return Type
	}

	if vidPid != "" {
		if t, ok := _detectByVidPid(vidPid); ok {
			logger.Debugf("Detected by VID:PID (%s)", vidPid)
			return t
		}
	}

	return TypeANY
}

func _detectByVidPid(vidPid string) (PrinterType, bool) {
	if t, ok := vidPidTypeMap[strings.ToLower(vidPid)]; ok {
		return t, true
	}

	logger.Debugf("Set any for VID:PID (%s)", vidPid)
	return TypeANY, false
}

//  -----------------  LIBUSB ----------------------

var onlyAlphabetsRegex = regexp.MustCompile(`[^A-Z]+`)
var CMD_KEY = []string{"CMD:", "COMMAND SET:", "COMMANDSET:", "COMMAND:", "COMMANDS:"}
var PDF_CMDS = map[string]struct{}{"PCL": {}, "PCLC": {}, "PCLXL": {}, "POSTSCRIPT": {}}
var EPOS_CMDS = map[string]struct{}{"ESCPOS": {}, "TSPL": {}, "ZPL": {}}

func libUsbDetectPrinterType(dev *gousb.Device) PrinterType {
	id, err := _getPrinterDeviceID(dev)
	if err != nil {
		logger.Errorf("failed to detect printer type: %v", err)
		return TypeANY
	}

	cmds := _extractCMD(id)
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

	logger.Debugf("CMD: %v, ID: %s", cmds, id)
	return TypeANY
}

func _extractCMD(id string) []string {
	if id == "" {
		return nil
	}

	idUpper := strings.ToUpper(id)
	parts := strings.Split(idUpper, ";")

	var result []string
	seen := make(map[string]bool)

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		for _, c := range CMD_KEY {
			if strings.HasPrefix(p, c) {

				idx := strings.IndexByte(p, ':')
				if idx == -1 {
					continue
				}

				raw := strings.TrimSpace(p[idx+1:])
				for _, c := range strings.Split(raw, ",") {
					n := onlyAlphabetsRegex.ReplaceAllString(strings.ToUpper(strings.TrimSpace(c)), "")
					if n != "" && !seen[n] {
						seen[n] = true
						result = append(result, n)
					}
				}
			}
		}
	}

	return result
}

func _getPrinterDeviceID(dev *gousb.Device) (string, error) {
	buf := make([]byte, 1024)
	desc := dev.Desc
	for _, cfg := range desc.Configs {
		for _, iFace := range cfg.Interfaces {
			for _, alt := range iFace.AltSettings {

				if alt.Class != gousb.ClassPrinter {
					continue
				}

				n, err := dev.Control(
					0xA1,
					0,
					0,
					uint16(iFace.Number),
					buf,
				)

				if err == nil && n > 2 {
					length := int(buf[0])<<8 | int(buf[1])
					if length > n-2 {
						length = n - 2
					}
					return string(buf[2 : 2+length]), nil
				}
			}
		}
	}

	return "", fmt.Errorf("device id not found")
}
