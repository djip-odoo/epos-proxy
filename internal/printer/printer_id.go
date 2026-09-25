package printer

import (
	"encoding/base64"
	"epos-proxy/internal/logger"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// btMACRegexp and btUUIDRegexp mirror bluetooth/util.go patterns.
// They are duplicated here to avoid an import cycle between internal/printer
// and the bluetooth package.
var (
	btMACRegexp  = regexp.MustCompile(`(?i)^([0-9A-F]{2}[:\-]){5}[0-9A-F]{2}$`)
	btUUIDRegexp = regexp.MustCompile(`(?i)^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`)
)

// ID is a decoded USB printer identity. Serial is preferred; VidPid plus
// Path is the fallback for devices that expose no serial number.
type ID struct {
	Serial string
	VidPid string
	Path   string
}

func encodePrinterID(libUsbPrinter *LibUsbPrinter) (string, error) {
	var parts []string

	if libUsbPrinter.VidPid != "" {
		parts = append(parts, "vp:"+libUsbPrinter.VidPid)
	}

	if libUsbPrinter.Serial != "" {
		parts = append(parts, "s:"+libUsbPrinter.Serial)
	} else if libUsbPrinter.Path != "" {
		parts = append(parts, "p:"+libUsbPrinter.Path)
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("cannot encode printer ID: no identifier provided")
	}

	base := strings.Join(parts, "|")
	id := base64.RawURLEncoding.EncodeToString([]byte(base))
	logger.Infof("LibUsbPrinter: %v | base: %s | encoded id: %s", libUsbPrinter, base, id)
	return id, nil
}

var ErrInvalidPrinterID = errors.New("invalid printer ID format")

func decodePrinterID(id string) (*ID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return nil, ErrInvalidPrinterID
	}

	var (
		serial string
		VidPid string
		path   string
	)

	raw := string(decoded)
	for _, part := range strings.Split(raw, "|") {
		switch {
		case strings.HasPrefix(part, "s:"):
			serial = strings.TrimPrefix(part, "s:")

		case strings.HasPrefix(part, "vp:"):
			VidPid = strings.TrimPrefix(part, "vp:")

		case strings.HasPrefix(part, "p:"):
			path = strings.TrimPrefix(part, "p:")
		}
	}

	if serial == "" && path == "" && VidPid == "" {
		return nil, ErrInvalidPrinterID
	}

	logger.Infof("Decoded printer ID: %s {serial: %s, VidPid: %s, path: %s}", id, serial, VidPid, path)
	return &ID{
		Serial: serial,
		VidPid: VidPid,
		Path:   path,
	}, nil
}

func EncodeLANPrinterID(ip string) string {
	return base64.RawURLEncoding.EncodeToString([]byte("l:" + ip))
}

func DecodeLANPrinterID(id string) (string, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", false
	}

	if len(decoded) < 3 || decoded[1] != ':' {
		return "", false
	}

	if decoded[0] != 'l' {
		return "", false
	}

	return string(decoded[2:]), true
}

func EncodeBluetoothPrinterID(mac string) string {
	return base64.RawURLEncoding.EncodeToString([]byte("b:" + mac))
}

func DecodeBluetoothPrinterID(id string) (string, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", false
	}
	if len(decoded) < 3 || decoded[1] != ':' || decoded[0] != 'b' {
		return "", false
	}
	address := string(decoded[2:])
	// Reject anything that isn't a valid Bluetooth MAC or CoreBluetooth UUID.
	// This closes the bypass where any base64("b:<garbage>") could reach Dial.
	if !btMACRegexp.MatchString(address) && !btUUIDRegexp.MatchString(address) {
		return "", false
	}
	return address, true
}
