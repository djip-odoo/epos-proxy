package printer

import (
	"encoding/base64"
	"epos-proxy/logger"
	"errors"
	"strings"
)

func encodePrinterID(serial string, path string, cupsName string) string {
	parts := []string{}

	if serial != "" {
		parts = append(parts, "s:"+serial)
	} else if path != "" {
		parts = append(parts, "p:"+path)
	}

	if cupsName != "" {
		parts = append(parts, "c:"+cupsName)
	}

	if len(parts) == 0 {
		logger.Errorf("cannot encode printer ID: no identifier provided (serial, path, or CUPS name)")
		return ""
	}

	base := strings.Join(parts, "|")
	return base64.RawURLEncoding.EncodeToString([]byte(base))
}

var ErrInvalidPrinterID = errors.New("invalid printer ID format")

func decodePrinterID(id string) (*PrinterID, error) {

	decoded, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return nil, ErrInvalidPrinterID
	}

	raw := string(decoded)
	logger.Infof("Decoded printer ID: %s", raw)

	var (
		serial   string
		path     string
		cupsName string
	)

	for _, part := range strings.Split(raw, "|") {
		switch {
		case strings.HasPrefix(part, "s:"):
			serial = strings.TrimPrefix(part, "s:")

		case strings.HasPrefix(part, "p:"):
			path = strings.TrimPrefix(part, "p:")

		case strings.HasPrefix(part, "c:"):
			cupsName = strings.TrimPrefix(part, "c:")
		}
	}

	if serial == "" && path == "" && cupsName == "" {
		return nil, ErrInvalidPrinterID
	}

	return &PrinterID{
		Serial:   serial,
		Path:     path,
		CupsName: cupsName,
	}, nil
}
