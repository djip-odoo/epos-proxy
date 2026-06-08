package printer

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/gousb"
)

var nonAlphaRegex = regexp.MustCompile(`[^A-Z]+`)
var keyAliases = map[string]string{
	"CMD":         "CMD",
	"COMMAND SET": "CMD",
	"COMMANDSET":  "CMD",
	"COMMAND":     "CMD",
	"COMMANDS":    "CMD",

	"MFG":          "MFG",
	"MANUFACTURER": "MFG",

	"MDL":   "MDL",
	"MODEL": "MDL",

	"CLS":   "CLS",
	"CLASS": "CLS",
}

func getPrinterDeviceID(dev *gousb.Device) (DeviceID, bool, error) {
	buf := make([]byte, 1024)

	for _, cfg := range dev.Desc.Configs {
		for _, iFace := range cfg.Interfaces {
			for _, alt := range iFace.AltSettings {
				if alt.Class != gousb.ClassPrinter &&
					alt.Class != gousb.ClassVendorSpec {
					continue
				}
				n, err := dev.Control(
					0xA1,
					0x00,
					0x00,
					uint16(iFace.Number),
					buf,
				)
				if err != nil || n < 2 {
					continue
				}

				totalLen := int(buf[0])<<8 | int(buf[1])
				if totalLen <= 2 {
					continue
				}

				strLen := min(totalLen-2, n-2)
				if strLen <= 0 {
					continue
				}

				raw := sanitizeDeviceID(string(buf[2 : 2+strLen]))
				deviceID := parseDeviceID(raw)

				if len(deviceID) == 0 {
					continue
				}
				deviceID["RAW"] = raw

				isPrinter := alt.Class == gousb.ClassPrinter
				return deviceID, isPrinter, nil
			}
		}
	}

	return DeviceID{}, false, fmt.Errorf("device id not found")
}

func parseDeviceID(raw string) DeviceID {
	result := make(DeviceID)

	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}

		key := normalizeKey(kv[0])
		val := strings.TrimSpace(kv[1])

		if key == "" || val == "" {
			continue
		}

		if existing, ok := result[key]; ok {
			if !strings.Contains(existing, val) {
				result[key] = existing + "," + val
			}
			continue
		}

		result[key] = val
	}

	return result
}

func normalizeKey(key string) string {
	key = strings.ToUpper(strings.TrimSpace(key))

	if alias, ok := keyAliases[key]; ok {
		return alias
	}

	return key
}

func sanitizeDeviceID(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")

	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
}

func (id *DeviceID) extractCmds() []string {
	var result []string
	seen := make(map[string]bool)

	raw := (*id)["CMD"]
	if raw == "" {
		return result
	}

	for _, c := range strings.Split(raw, ",") {
		n := nonAlphaRegex.ReplaceAllString(
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

func (id *DeviceID) hasCommand(command string) bool {
	for _, c := range id.extractCmds() {
		if c == command {
			return true
		}
	}
	return false
}
