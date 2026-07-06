package printer

import (
	"regexp"
	"strings"
	"unicode"
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
