package escpos

import (
	"encoding/xml"
	"epos-proxy/config"
	"fmt"
	"strings"
)

type XmlRawItem struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content string     `xml:",chardata"`
}

type XmlEPOSPrint struct {
	XMLName xml.Name     `xml:"epos-print"`
	Items   []XmlRawItem `xml:",any"`
}

func ParseXML(body []byte, info config.KnownPrinterInfo) ([]byte, error) {
	if info.Protocol == "ESCPOS" {
		return ParseXMLToESCPOS(body)
	}
	if info.Protocol == "ESCPOS_COMPAT" {
		return ParseXMLToRasterImage(body, info)
	}
	return nil, fmt.Errorf("unsupported protocol: %s", info.Protocol)
}

func attrMap(attrs []xml.Attr) map[string]string {
	m := make(map[string]string, len(attrs))
	for _, a := range attrs {
		m[strings.ToLower(a.Name.Local)] = a.Value
	}
	return m
}

func parseInt(s string, def int) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return def
	}
	return n
}
