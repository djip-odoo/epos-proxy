package printer

import (
	"fmt"
	"regexp"
	"strings"
)

// BluetoothPrinterInfo holds discovered information about a Bluetooth printer.
type BluetoothPrinterInfo struct {
	MAC  string `json:"mac"`
	Name string `json:"name"`
	Id   string `json:"id"`
}

// NormalizeMAC normalises a MAC address to uppercase with colons.
func NormalizeMAC(mac string) string {
	mac = strings.ToUpper(strings.TrimSpace(mac))
	mac = strings.ReplaceAll(mac, "-", ":")
	return mac
}

// ValidateMAC checks that a string looks like a valid Bluetooth MAC address.
func ValidateMAC(mac string) error {
	mac = strings.TrimSpace(mac)
	matched, _ := regexp.MatchString(`^([0-9A-Fa-f]{2}[:\-]){5}[0-9A-Fa-f]{2}$`, mac)
	if !matched {
		return fmt.Errorf("invalid MAC address format: %s (expected format: AA:BB:CC:DD:EE:FF)", mac)
	}
	return nil
}

// CheckBluetoothPrinter verifies that a Bluetooth printer is reachable by
// opening an RFCOMM connection and immediately closing it.
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
