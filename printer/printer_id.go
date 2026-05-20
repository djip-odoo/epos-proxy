package printer

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/gousb"
)

func encodePrinterID(serial string, vendorID gousb.ID, productID gousb.ID) string {
	if serial != "" {
		return base64.RawURLEncoding.EncodeToString([]byte("s:" + serial))
	}
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("p:%04X:%04X", uint16(vendorID), uint16(productID))))
}

var ErrInvalidPrinterID = errors.New("invalid printer ID format")

func decodePrinterID(id string) (*PrinterID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return nil, ErrInvalidPrinterID
	}

	if len(decoded) < 3 || decoded[1] != ':' {
		return nil, ErrInvalidPrinterID
	}

	kind := decoded[0]
	payload := decoded[2:]

	switch kind {
	case 's':
		if len(payload) == 0 {
			return nil, ErrInvalidPrinterID
		}
		return &PrinterID{Serial: string(payload)}, nil

	case 'p':
		// Expect payload: "<vendor>:<product>"
		vStr, pStr, ok := strings.Cut(string(payload), ":")
		if !ok || vStr == "" || pStr == "" {
			return nil, ErrInvalidPrinterID
		}

		v, err := strconv.ParseUint(vStr, 16, 16)
		if err != nil {
			return nil, ErrInvalidPrinterID
		}
		p, err := strconv.ParseUint(pStr, 16, 16)
		if err != nil {
			return nil, ErrInvalidPrinterID
		}

		return &PrinterID{
			VendorID:  gousb.ID(v),
			ProductID: gousb.ID(p),
		}, nil

	default:
		return nil, ErrInvalidPrinterID
	}
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
