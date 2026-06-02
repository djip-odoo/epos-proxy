package printer

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"epos-proxy/logger"
)

// BluetoothPrinterInfo holds discovered information about a Bluetooth printer.
type BluetoothPrinterInfo struct {
	MAC  string `json:"mac"`
	Name string `json:"name"`
	Id   string `json:"id"`
}

// EncodeBluetoothPrinterID creates a URL-safe base64 printer ID for a Bluetooth
// printer identified by its MAC address.
func EncodeBluetoothPrinterID(mac string) string {
	return base64.RawURLEncoding.EncodeToString([]byte("b:" + mac))
}

// DecodeBluetoothPrinterID extracts the MAC address from a Bluetooth printer ID.
// Returns the MAC and true on success, or an empty string and false on failure.
func DecodeBluetoothPrinterID(id string) (string, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", false
	}
	if len(decoded) < 3 || decoded[1] != ':' || decoded[0] != 'b' {
		return "", false
	}
	return string(decoded[2:]), true
}

// validateMAC checks that a string looks like a valid Bluetooth MAC address.
func validateMAC(mac string) error {
	mac = strings.TrimSpace(mac)
	matched, _ := regexp.MatchString(`^([0-9A-Fa-f]{2}[:\-]){5}[0-9A-Fa-f]{2}$`, mac)
	if !matched {
		return fmt.Errorf("invalid MAC address format: %s", mac)
	}
	return nil
}

// NormalizeMAC normalises a MAC address to uppercase with colons.
func NormalizeMAC(mac string) string {
	mac = strings.ToUpper(strings.TrimSpace(mac))
	mac = strings.ReplaceAll(mac, "-", ":")
	return mac
}

// discoverRFCOMMChannel finds the best RFCOMM channel to use for a printer.
// Priority:
//  1. Use the cached channel from config (if > 0)
//  2. SDP/RFCOMM service discovery via platform tool
//  3. Sequential probe of common channels (1–8)
//
// Note: on Linux, channel probing is handled inside dialRFCOMMLinux which uses
// the RFCOMM device binding path. This function is used on macOS / as a
// fallback probe for the raw socket path.
func discoverRFCOMMChannel(mac string, cached int) (int, error) {
	if cached > 0 {
		logger.Debugf("BT: Using cached RFCOMM channel %d for %s", cached, mac)
		return cached, nil
	}

	// Try SDP discovery
	ch, err := sdpDiscoverChannel(mac)
	if err == nil && ch > 0 {
		logger.Infof("BT: SDP discovered channel %d for %s", ch, mac)
		return ch, nil
	}

	logger.Errorf("BT: SDP discovery failed for %s: %v; probing common channels 1–8", mac, err)

	// Probe channels 1–8 (covers the full range of common thermal printer defaults).
	for _, ch := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		conn, err := dialRFCOMM(mac, ch)
		if err == nil {
			_ = conn.Close()
			logger.Infof("BT: Channel probe succeeded on channel %d for %s", ch, mac)
			return ch, nil
		}
	}

	return 0, fmt.Errorf("no working RFCOMM channel found for %s", mac)
}

// CheckBluetoothPrinter verifies that a Bluetooth printer is reachable.
// On Linux it uses the RFCOMM device binding path (rfcomm bind + /dev/rfcommX)
// so that printers with broken SDP records are handled correctly.
// On other platforms it falls back to a raw RFCOMM socket.
func CheckBluetoothPrinter(mac string, cachedChannel int) error {
	conn, err := dialRFCOMMPlatform(mac, cachedChannel)
	if err != nil {
		return fmt.Errorf("bluetooth printer %s is unreachable: %w", mac, err)
	}
	_ = conn.Close()
	return nil
}
