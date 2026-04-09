package printer

import (
	"epos-proxy/logger"
	"fmt"
	"strings"

	"github.com/google/gousb"
)

var vidPidTypeMap = map[string]PrinterType{
	"2aaf:6015": TypeEPOS, // Essae thermal
	"04b8:0e32": TypeEPOS, // Epson thermal
	"2d84:c7c8": TypeEPOS, // Zhuhai Poskey Technology
	"4b43:3830": TypeEPOS, // Caysn CN811-UWB
}

var eposKeywords = []string{
	// Epson
	"tm-t", "tm-m", "tm-p", "tm-u", "tm-l", "m30",

	// Bixolon
	"srp-3", "srp-2", "srp-e", "bixolon",

	// Star Micronics
	"tsp100", "tsp600", "tsp700", "mc-print", "star line",

	// Budget/Android Specialized
	"xp-58", "xp-80", "xp-q", "rp58", "rp80", "rpp200", "rpp300",
	"sunmi", "zjiang", "hoin", "gprinter", "rongta",

	// Citizen
	"ct-s2", "ct-s3", "ct-s6", "ct-s8",
}

func DetectPrinterType(printerName string, vidPid string) PrinterType {
	if vidPid != "" {
		if t, ok := detectByVidPid(vidPid); ok {
			logger.Debugf("Detected by VID:PID (%s)", vidPid)
			return t
		}
	}

	if printerName == "" {
		logger.Infof("Set Unknown for Name: %s", printerName)
		return TypeUNKNOWN
	}

	s := strings.ToLower(strings.TrimSpace(printerName))
	return detectByName(s)
}

func detectByName(name string) PrinterType {
	for _, k := range eposKeywords {
		if strings.Contains(name, k) {
			logger.Debugf("Detected by name, %s", name)
			return TypeEPOS
		}
	}
	logger.Infof("Set Unknown for Name: %s", name)
	return TypeUNKNOWN
}

func detectByVidPid(vidPid string) (PrinterType, bool) {
	if t, ok := vidPidTypeMap[strings.ToLower(vidPid)]; ok {
		return t, true
	}

	logger.Infof("Set Unknown for VID:PID (%s)", vidPid)
	return TypeUNKNOWN, false
}

func LibUsbDetectPrinterType(dev *gousb.Device) (PrinterType, error) {
	id, err := getPrinterDeviceID(dev)
	if err != nil {
		return "UNKNOWN", err
	}

	idUpper := strings.ToUpper(id)
	switch {
	case strings.Contains(idUpper, "ESC/POS"),
		strings.Contains(idUpper, "ESCPOS"),
		strings.Contains(idUpper, "ESC/P"):
		return TypeEPOS, nil

	case strings.Contains(idUpper, "PDF"),
		strings.Contains(idUpper, "PCL"),
		strings.Contains(idUpper, "PCLXL"),
		strings.Contains(idUpper, "POSTSCRIPT"),
		strings.Contains(idUpper, "PS"):
		return TypePDF, nil

	default:
		logger.Infof("CMD: %s, ID: %s", idUpper, id)
		return TypeUNKNOWN, nil
	}
}

func getPrinterDeviceID(dev *gousb.Device) (string, error) {
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
