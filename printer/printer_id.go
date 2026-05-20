package printer

import (
	"encoding/base64"
	"epos-proxy/logger"
	"errors"
	"fmt"
	"strings"
)

func encodePrinterID(serial string, path string) (string, error) {
	var parts []string

	if serial != "" {
		parts = append(parts, "s:"+serial)
	} else if path != "" {
		parts = append(parts, "p:"+path)
	} else {
		return "", fmt.Errorf("cannot encode printer ID: no identifier provided (serial or path)")
	}

	base := strings.Join(parts, "|")
	return base64.RawURLEncoding.EncodeToString([]byte(base)), nil
}

var ErrInvalidPrinterID = errors.New("invalid printer ID format")

func decodePrinterID(id string) (*PrinterID, error) {

	decoded, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return nil, ErrInvalidPrinterID
	}

	raw := string(decoded)
	logger.Debugf("Decoded printer ID: %s", raw)

	var (
		serial string
		path   string
	)

	for _, part := range strings.Split(raw, "|") {
		switch {
		case strings.HasPrefix(part, "s:"):
			serial = strings.TrimPrefix(part, "s:")

		case strings.HasPrefix(part, "p:"):
			path = strings.TrimPrefix(part, "p:")
		}
	}

	if serial == "" && path == "" {
		return nil, ErrInvalidPrinterID
	}

	logger.Infof("Decoded printer ID: {serial: %s, path: %s}", serial, path)
	return &PrinterID{
		Serial: serial,
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
