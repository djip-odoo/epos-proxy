package printer

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"epos-proxy/logger"

	"github.com/google/gousb"
)

func ListUSBPrinters() (*Printers, error) {
	logger.Debug("Starting USB printer detection")

	systemUsbPrinters, err := ListSystemPrinters()
	if err != nil {
		return nil, fmt.Errorf("Failed to get System printers , %v", err)
	}

	libusbPrinters, unavailable, err := listLibUsbPrinters()
	if err != nil {
		return nil, fmt.Errorf("Failed to get Usb printers, %v", err)
	}

	result, err := mergePrinters(systemUsbPrinters, libusbPrinters, unavailable)
	if err != nil {
		logger.Errorf("System printer detection failed: %v", err)
		// return result, err2
	}
	return result, nil
}

func listLibUsbPrinters() ([]LibUsbPrinter, []UnavailableInfo, error) {
	ctx := gousb.NewContext()
	defer func(ctx *gousb.Context) {
		_ = ctx.Close()

	}(ctx)
	var descriptors []gousb.DeviceDesc
	_, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		_, supported := findPrinterEndpoint(desc)
		if supported {
			descriptors = append(descriptors, *desc)
		}
		return false
	})

	if err != nil {
		return nil, nil, fmt.Errorf("failed to open USB devices for listing: %w", err)
	}

	var printers []LibUsbPrinter
	var unavailable []UnavailableInfo

	for _, desc := range descriptors {
		info, err := GetPrinterInfo(ctx, &desc)
		if err != nil {
			// Device is not accessible, likely due to permissions / drivers.
			vid := fmt.Sprintf("%04X", uint16(desc.Vendor))
			pid := fmt.Sprintf("%04X", uint16(desc.Product))
			unavailable = append(unavailable, UnavailableInfo{
				Name:  getPrinterFriendlyName(vid, pid),
				Error: err.Error(),
				Type: TypeUNKNOWN,
			})
		} else if info != nil {
			logger.Debugf("Found available USB printer: %s (Serial: %s)", info.Name, info.Serial)
			printers = append(printers, *info)
		}
	}
	return printers, unavailable, nil
}
func GetPrinterInfo(ctx *gousb.Context, descToFind *gousb.DeviceDesc) (*LibUsbPrinter, error) {
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
	info := &LibUsbPrinter{}
	productName, _ := device.Product()
	vendorName, _ := device.Manufacturer()

	if productName == "" {
		productName = fmt.Sprintf("PID: %04X", uint16(descToFind.Product))
	}

	if vendorName == "" {
		vendorName = fmt.Sprintf("VID: %04X", uint16(descToFind.Vendor))
	}

	info.Name = fmt.Sprintf("%s %s", vendorName, productName)
	info.Serial, _ = device.SerialNumber()
	info.Path = PathToString(descToFind)
	info.Type = TypeUNKNOWN
	info.VidPid = fmt.Sprintf("%04X:%04X", uint16(descToFind.Vendor), uint16(descToFind.Product))
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
