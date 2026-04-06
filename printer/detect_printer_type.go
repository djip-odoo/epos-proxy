package printer

import (
	"epos-proxy/logger"
	"strings"
)

var eposKeywords = []string{
	"tm-t", "tm t", "tm20", "tm82", "tm88", "tm-m", "epson tm",
	"srp-3", "srp-350", "srp-330", "srp-275", "bixolon",
	"rp58", "rp80", "rpp200", "rpp300", "rongta rp",
	"xp-58", "xp-80", "xp-q200", "ct-s", "tvs rp", "tvs msp",
	"sunmi", "hoin", "zjiang", "gprinter gp-5890",
	"receipt", "thermal pos", "epos",
}

func DetectPrinterType(printerName string) PrinterType {
	if printerName == "" {
		logger.Infof("Set Unknown for Name: %s", printerName)
		return TypeUNKNOWN
	}

	s := strings.ToLower(strings.TrimSpace(printerName))
	return detectByName(s)
}

func detectByName(s string) PrinterType {
	for _, k := range eposKeywords {
		if strings.Contains(s, k) {
			return TypeEPOS
		}
	}
	return TypeUNKNOWN
}
