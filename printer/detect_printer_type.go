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

var labelKeywords = []string{
	"zd2", "zd4", "zd6", "gk42", "gk420", "gx43", "zt2", "zt4", "zt41",
	"zebra", "te244", "te200", "te210", "ttp-244", "ttp-247",
	"ql-7", "ql-8", "ql-11", "brother ql", "brother td",
	"dymo labelwriter", "godex", "honeywell pc", "honeywell pm",
	"sato", "munbyn", "labelworks", "colorworks", "label printer",
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
	"LABEL": {labelKeywords, TypeLABEL},
	"PDF":   {pdfKeywords, TypePDF},
}

func DetectPrinterType(printerName string, activeFilter string) PrinterType {
	if printerName == "" {
		return TypeUNKNOWN
	}

	s := strings.ToLower(strings.TrimSpace(printerName))
	return detectByName(s, activeFilter)
}

func detectByName(s string, activeFilter string) PrinterType {
	if entry, ok := keywordMap[activeFilter]; ok {
		for _, k := range entry.keywords {
			if strings.Contains(s, k) {
				return entry.pType
			}
		}
	}

	return TypeUNKNOWN
}
