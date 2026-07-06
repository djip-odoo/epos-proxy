//go:build android

package printer

import (
	"encoding/json"
	"fmt"
	"epos-proxy/logger"
)

type androidUsbPrinterInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	VidPid string `json:"vidPid"`
	Serial string `json:"serial"`
}

func ListUSBPrinters() (*Printers, error) {
	logger.Debugf("ListUSBPrinters (Android): Starting USB printer detection")
	resJson := callJavaPrinter("listUSBPrinters", "")
	if resJson == "" {
		return &Printers{
			Available:   make([]Info, 0),
			Unavailable: make([]UnavailableInfo, 0),
		}, nil
	}

	var rawPrinters []androidUsbPrinterInfo
	err := json.Unmarshal([]byte(resJson), &rawPrinters)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Android USB printers JSON: %w", err)
	}

	result := &Printers{
		Available:   make([]Info, 0),
		Unavailable: make([]UnavailableInfo, 0),
	}

	for _, p := range rawPrinters {
		libUsbPrinter := &LibUsbPrinter{
			Serial: p.Serial,
			Path:   p.Path,
			Name:   p.Name,
			VidPid: p.VidPid,
		}
		id, err := encodePrinterID(libUsbPrinter)
		if err != nil {
			logger.Errorf("failed to encode Android printer ID: %v", err)
			continue
		}
		result.Available = append(result.Available, Info{
			Id:   id,
			Name: p.Name,
		})
	}

	return result, nil
}
