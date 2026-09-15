package usb

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"epos-proxy/internal/logger"
)

// ID is a decoded USB printer identity. Serial is preferred; VidPid plus
// Path is the fallback for devices that expose no serial number.
type ID struct {
	Serial string
	VidPid string
	Path   string
}

var ErrInvalidPrinterID = errors.New("invalid printer ID format")

func encodeID(libUsbPrinter *LibUsbPrinter) (string, error) {
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
	id := "usb_" + base64.RawURLEncoding.EncodeToString([]byte(base))
	logger.Infof("LibUsbPrinter: %v | base: %s | encoded id: %s", libUsbPrinter, base, id)
	return id, nil
}

func decodeID(id string) (*ID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "usb_"))
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

// Backward compatibility helpers
func encodePrinterID(p *LibUsbPrinter) (string, error) { return encodeID(p) }
func decodePrinterID(id string) (*ID, error)           { return decodeID(id) }
