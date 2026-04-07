package printer

import (
	"epos-proxy/logger"
	"strings"
)

var vidPidTypeMap = map[string]PrinterType{
	"2aaf:6015": TypeEPOS, // Essae thermal
	"04b8:0e32": TypeEPOS, // Epson thermal
}

var eposKeywords = []string{
	"tm-t", "tm t", "tm20", "tm82", "tm88", "tm-m", "epson tm",
	"srp-3", "srp-350", "srp-330", "srp-275", "bixolon",
	"rp58", "rp80", "rpp200", "rpp300", "rongta rp",
	"xp-58", "xp-80", "xp-q200", "ct-s", "tvs rp", "tvs msp",
	"sunmi", "hoin", "zjiang", "gprinter gp-5890",
	"receipt", "thermal pos", "epos",
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
