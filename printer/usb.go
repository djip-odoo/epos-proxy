package printer

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"epos-proxy/logger"

	"github.com/google/gousb"
)

var supportedVendorIDs = map[gousb.ID]string{
	0x04B8: "Epson",
}

func ListUSBPrinters(activeFilter string) (*Printers, error) {
	logger.Debug("Starting USB printer detection")
	ctx := gousb.NewContext()
	defer func(ctx *gousb.Context) {
		_ = ctx.Close()

	}(ctx)
	result := &Printers{
		Available:   make([]Info, 0),
		Unavailable: make([]UnavailableInfo, 0),
	}
	logger.Infof("filter: %s", activeFilter)
	if activeFilter == "EPOS" {
		logger.Infof("usb printer added")
		err := addLibUsbPrinters(ctx, result)
		if err != nil {
			return nil, err
		}
		if activeFilter == "EPOS" {
			return result, nil
		}
	}

	err := getSystemPrinters(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func addLibUsbPrinters(ctx *gousb.Context, result *Printers) error {
	var descriptors []gousb.DeviceDesc
	_, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		_, supported := findPrinterEndpoint(desc)
		if supported {
			descriptors = append(descriptors, *desc)
		}
		return false
	})

	if err != nil {
		return fmt.Errorf("failed to open USB devices for listing: %w", err)
	}

	for _, desc := range descriptors {
		info, err := GetPrinterInfo(ctx, &desc)
		if err != nil {
			// Device is not accessible, likely due to permissions / drivers.
			vid := fmt.Sprintf("%04X", uint16(desc.Vendor))
			pid := fmt.Sprintf("%04X", uint16(desc.Product))
			result.Unavailable = append(result.Unavailable, UnavailableInfo{
				Name:  getPrinterFriendlyName(vid, pid),
				Error: err.Error(),
			})
		} else if info != nil {
			logger.Debugf("Found available USB printer: %s (Serial: %s)", info.ProductName, info.Serial)
			result.Available = append(result.Available, *info)
		}
	}
	return nil
}
func GetPrinterInfo(ctx *gousb.Context, descToFind *gousb.DeviceDesc) (*Info, error) {
	logger.Debugf("Attempting to get info for USB device: Bus %d, Address %d, Vendor %04X, Product %04X", descToFind.Bus, descToFind.Address, uint16(descToFind.Vendor), uint16(descToFind.Product))
	var found bool
	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if found {
			return false
		}
		if descToFind.Bus != desc.Bus || descToFind.Address != desc.Address ||
			descToFind.Vendor != desc.Vendor || descToFind.Product != desc.Product {
			return false
		}
		found = true
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open USB device for info retrieval: %w", err)
	}

	if len(devices) == 0 {
		return nil, nil
	}

	defer func() {
		for _, d := range devices {
			_ = d.Close()
		}
	}()

	device := devices[0]
	info := &Info{}
	info.ProductName, _ = device.Product()

	if info.ProductName == "" {
		info.ProductName = fmt.Sprintf("PID: %04X", uint16(descToFind.Product))
	}

	info.VendorName, _ = device.Manufacturer()

	if info.VendorName == "" {
		info.VendorName = fmt.Sprintf("VID: %04X", uint16(descToFind.Vendor))
	}

	info.Serial, _ = device.SerialNumber()
	info.Path = PathToString(descToFind)
	info.Type = DetectPrinterType(fmt.Sprint(info))

	id, err := encodePrinterID(info.Serial, info.Path, "")
	if err != nil {
		logger.Errorf("Failed to encode printer ID: %v", err)
	} else {
		info.Id = id
	}
	return info, nil
}

func encodePrinterID(serial string, path string, cupsName string) (string, error) {
	parts := []string{}

	if serial != "" {
		parts = append(parts, "s:"+serial)
	} else if path != "" {
		parts = append(parts, "p:"+path)
	} else if cupsName != "" {
		parts = append(parts, "c:"+cupsName)
	}

	if len(parts) == 0 {
		err := fmt.Errorf("cannot encode printer ID: no identifier provided (serial, path, or CUPS name)")
		return "", err
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
	logger.Infof("Decoded printer ID: %s", raw)

	parts := strings.Split(raw, "|")

	var (
		serial   string
		path     string
		cupsName string
	)

	for _, part := range parts {
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

func findPrinterEndpoint(dev *gousb.DeviceDesc) (EndpointInfo, bool) {
	// _, supportedVendor := supportedVendorIDs[dev.Vendor]
	// if !supportedVendor {
	//	return EndpointInfo{}, false
	//}

	for cfgNum, cfg := range dev.Configs {
		for _, iFace := range cfg.Interfaces {
			for _, alt := range iFace.AltSettings {
				if alt.Class != gousb.ClassPrinter {
					continue
				}
				for _, ep := range alt.Endpoints {
					if ep.Direction == gousb.EndpointDirectionOut &&
						ep.TransferType == gousb.TransferTypeBulk {
						return EndpointInfo{
							config:           cfgNum,
							iFace:            iFace.Number,
							alternateSetting: alt.Alternate,
							outEndpoint:      ep.Number,
						}, true
					}
				}
			}
		}
	}
	return EndpointInfo{}, false
}

func getSystemPrinters(result *Printers) error {
	cupsPrinters, err := ListSystemPrinters()
	if err != nil {
		return fmt.Errorf("failed to list system printers: %w", err)
	}

	statusMap, err := GetSystemPrinterStatusMap()
	if err != nil {
		return fmt.Errorf("failed to get CUPS printer status: %w", err)
	}

	for _, cups := range cupsPrinters {
		status := statusMap[cups.CupsName]

		if strings.Contains(status, "disabled") || strings.Contains(status, "stopped") {
			logger.Debugf("CUPS printer unavailable: %s (%s)", cups.CupsName, status)
			continue
		}

		id, err := encodePrinterID(cups.Serial, "", cups.CupsName)
		if err != nil {
			logger.Errorf("Failed to encode printer ID: %v", err)
		} else {
			cups.Id = id
		}

		result.Available = append(result.Available, cups)

	}
	return nil
}
