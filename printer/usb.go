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

func ListUSBPrinters() (*Printers, error) {
	logger.Debug("Starting USB printer detection")
	
	systemUsbPrinters, err := ListSystemPrinters()
	if err != nil {
		return nil, fmt.Errorf("Failed to get System printers , %v", err)
	}

	logger.Infof("usb printer added")
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
			})
		} else if info != nil {
			logger.Debugf("Found available USB printer: %s (Serial: %s)", info.ProductName, info.Serial)
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
	info.Type = TypeUNKNOWN
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

func mergePrinters(systemPrinters []SystemUsbPrinter, libusbPrinters []LibUsbPrinter, unavailable []UnavailableInfo) (*Printers, error) {
	result := &Printers{
		Available:   make([]Info, 0),
		Unavailable: make([]UnavailableInfo, 0),
	}
	result.Unavailable = append(result.Unavailable, unavailable...)

	for _, sysUsb := range systemPrinters {
		found := false

		for i, libUsb := range libusbPrinters {
			logger.Debugf("Matching USB[%d]: Serial=%v Path=%v with CUPS Serial=%v Name=%s",
				i, libUsb.Serial, libUsb.Path, sysUsb.Serial, sysUsb.IdName)
			
			// Serial match
			if libUsb.Serial != "" && sysUsb.Serial != "" && libUsb.Serial == sysUsb.Serial {
				logger.Debugf("Matched by SERIAL: %s ↔ %s", libUsb.Serial, sysUsb.IdName)
				id, err := encodePrinterID(libUsb.Serial, libUsb.Path, sysUsb.IdName)
				if err != nil {
					logger.Errorf("Failed to encode printer ID: %v", err)
				} else {
					result.Available = append(result.Available, Info{
						Id:          id,
						Name:        libUsb.VendorName + " " + libUsb.ProductName,
						Type:        sysUsb.Type,
					})
				}

				found = true
				break
			}
		}

		// No USB match → standalone System printer
		if !found {
			logger.Debugf("No USB match for CUPS printer: %s", sysUsb.IdName)

			id, err := encodePrinterID("", "", sysUsb.IdName)
			if err != nil {
				logger.Errorf("Failed to encode printer ID: %v", err)
				continue
			}

			result.Available = append(result.Available, Info{
				Id:          id,
				Name:        sysUsb.IdName,
				Type:        sysUsb.Type,	
			})
		}
	}

	// for i, usb := range result.Available {
	// 	if usb.Id == "" {
	// 		logger.Debugf("Assigning ID to USB-only printer: %s", usb.ProductName)

	// 		id, err := encodePrinterID(usb.Serial, usb.Path, usb.CupsName)
	// 		if err != nil {
	// 			logger.Errorf("Failed to encode printer ID: %v", err)
	// 		} else {
	// 			result.Available[i].Id = id
	// 		}
	// 	}
	// }
	return result, nil
}
