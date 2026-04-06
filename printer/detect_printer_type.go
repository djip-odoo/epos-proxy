package printer

import "strings"

var eposKeywords = []string{
	"tm-t", "tm t", "tm20", "tm82", "tm88", "tm-m", "epson tm",
	"srp-3", "srp-350", "srp-330", "srp-275", "bixolon",
	"rp58", "rp80", "rpp200", "rpp300", "rongta rp",
	"xp-58", "xp-80", "xp-q200", "ct-s", "tvs rp", "tvs msp",
	"sunmi", "hoin", "zjiang", "gprinter gp-5890",
	"receipt", "thermal pos", "epos",
}


var pdfKeywords = []string{
	"pdf", "laserjet", "deskjet", "officejet", "smart tank", "ink tank",
	"lbp", "imageclass", "pixma", "ecotank", "workforce",
	"hl-l", "dcp-l", "mfc-l", "brother hl", "brother dcp",
	"ml-", "xpress", "phaser", "workcentre", "imageprograf",
}

var keywordMap = map[string]struct {
	keywords []string
	pType    PrinterType
}{
	"EPOS":  {eposKeywords, TypeEPOS},
	"PDF":   {pdfKeywords, TypePDF},
}

func DetectPrinterType(printerName string, pType PrinterType) PrinterType {
	if printerName == "" {
		return TypeUNKNOWN
	}
	if pType != TypeUNKNOWN {
		return  pType
	}

	s := strings.ToLower(strings.TrimSpace(printerName))
	return detectByName(s)
}

func detectByName(s string) PrinterType {
	for _, entry := range keywordMap {
		for _, k := range entry.keywords {
			if strings.Contains(s, k) {
				return entry.pType
			}
		}
	}
	return TypeUNKNOWN
}
