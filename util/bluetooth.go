package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// parseMACToBytes converts "AA:BB:CC:DD:EE:FF" → [6]byte in reversed order
// (little-endian as required by the BlueZ sockaddr_rc).
func ParseMACToBytes(mac string) ([6]byte, error) {
	parts := strings.Split(strings.ToUpper(mac), ":")
	if len(parts) != 6 {
		return [6]byte{}, fmt.Errorf("invalid MAC: %s", mac)
	}
	var b [6]byte
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return [6]byte{}, fmt.Errorf("invalid MAC octet %q: %w", p, err)
		}
		b[5-i] = byte(v) // reversed (little-endian)
	}
	return b, nil
}

func NormalizeMAC(mac string) string {
	mac = strings.ToUpper(strings.TrimSpace(mac))
	mac = strings.ReplaceAll(mac, "-", ":")
	return mac
}

func ValidateMAC(mac string) error {
	mac = strings.TrimSpace(mac)
	matched, _ := regexp.MatchString(`^([0-9A-Fa-f]{2}[:\-]){5}[0-9A-Fa-f]{2}$`, mac)
	if !matched {
		return fmt.Errorf("invalid MAC address format: %s (expected format: AA:BB:CC:DD:EE:FF)", mac)
	}
	return nil
}
