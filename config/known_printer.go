package config

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/gousb"
)

const DefaultPrinterWidth = 576
const DefaultPrinterBottomPadding = 120

//go:embed known_printer.json
var knownPrinterJSONData []byte

type KnownPrinterInfo struct {
	Type          string `json:"type"`
	Protocol      string `json:"protocol"`
	Width         int    `json:"width"`
	BottomPadding int    `json:"bottom_padding"`
}

var (
	knownPrinters   map[string]KnownPrinterInfo
	knownPrintersMu sync.RWMutex
	lastModTime     time.Time
)


func getKnownPrinters() map[string]KnownPrinterInfo {
	filePath, err := ConfigDirectory("known_printer.json")
	if err != nil {
		log.Printf("[config] warning: could not get known_printer.json path: %v", err)
	}

	knownPrintersMu.RLock()
	if knownPrinters != nil {
		if filePath != "" {
			if stat, err := os.Stat(filePath); err == nil && !stat.ModTime().After(lastModTime) {
				m := knownPrinters
				knownPrintersMu.RUnlock()
				return m
			}
		} else {
			m := knownPrinters
			knownPrintersMu.RUnlock()
			return m
		}
	}
	knownPrintersMu.RUnlock()

	knownPrintersMu.Lock()
	defer knownPrintersMu.Unlock()

	knownPrinters = reloadKnownPrintersLocked(filePath)
	return knownPrinters
}

func reloadKnownPrintersLocked(filePath string) map[string]KnownPrinterInfo {
	result := make(map[string]KnownPrinterInfo)
	var data []byte

	if filePath != "" {
		if stat, err := os.Stat(filePath); err == nil && stat.Size() > 0 {
			if fileData, err := os.ReadFile(filePath); err == nil {
				data = fileData
				lastModTime = stat.ModTime()
			}
		} else {
			// Save default embedded JSON to user config directory so user can edit it in production
			data = knownPrinterJSONData
			if err := os.WriteFile(filePath, data, 0644); err == nil {
				if stat, err := os.Stat(filePath); err == nil {
					lastModTime = stat.ModTime()
				}
			}
		}
	}

	if len(data) == 0 {
		data = knownPrinterJSONData
	}

	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("[config] error parsing %s: %v, falling back to embedded defaults", filePath, err)
		_ = json.Unmarshal(knownPrinterJSONData, &result)
	}

	return result
}

func IsKnownPrinter(desc *gousb.DeviceDesc) bool {
	printers := getKnownPrinters()
	vidPid := strings.ToLower(fmt.Sprintf("%04x:%04x", uint16(desc.Vendor), uint16(desc.Product)))
	_, ok := printers[vidPid]
	return ok
}

func GetPrinterType(vidPid string) string {
	printers := getKnownPrinters()
	if info, ok := printers[strings.ToLower(vidPid)]; ok {
		return info.Type
	}
	return "receipt"
}

func GetKnownPrinterInfo(vidPid string) KnownPrinterInfo {
	printers := getKnownPrinters()
	if info, ok := printers[strings.ToLower(vidPid)]; ok {
		return info
	}
	return KnownPrinterInfo{
		Type:          "receipt",
		Protocol:      "ESCPOS",
		Width:         DefaultPrinterWidth,
		BottomPadding: DefaultPrinterBottomPadding,
	}
}

func GetKnownPrinterInfoByPrinterID(id string) KnownPrinterInfo {
	if id != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(id)
		if err == nil {
			raw := string(decoded)
			for _, part := range strings.Split(raw, "|") {
				if strings.HasPrefix(part, "vp:") {
					vidPid := strings.TrimPrefix(part, "vp:")
					return GetKnownPrinterInfo(vidPid)
				}
			}
		}
	}

	return KnownPrinterInfo{
		Type:          "receipt",
		Protocol:      "ESCPOS",
		Width:         DefaultPrinterWidth,
		BottomPadding: DefaultPrinterBottomPadding,
	}
}
