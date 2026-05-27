package printer

import (
	"epos-proxy/config"
	"epos-proxy/logger"
	"fmt"

	"github.com/google/gousb"
)

func ListUSBPrinters() (*Printers, error) {
	logger.Debug("Starting USB printer detection")

	libusbPrinters, err := listLibUsbPrinters()
	if err != nil {
		return nil, fmt.Errorf("failed to get USB printers: %w", err)
	}

	return libusbPrinters, nil
}

func listLibUsbPrinters() (*Printers, error) {
	result := &Printers{
		Available:   make([]Info, 0),
		Unavailable: make([]UnavailableInfo, 0),
	}

	ctx := gousb.NewContext()
	defer func(ctx *gousb.Context) {
		_ = ctx.Close()
	}(ctx)

	current := make(map[string]struct{})

	// First list all  without opening devices, to avoid permission errors on some platforms
	_, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if _, supported := findPrinterEndpoint(desc); supported {
			key := fingerprintKey(desc)
			current[key] = struct{}{}
		}
		return false
	})

	if err != nil {
		return nil, fmt.Errorf("failed to enumerate USB devices: %w", err)
	}

	if !usbCache.HasChanged(current) {
		logger.Debugf("USB unchanged → using cache")
		available, unavailable := usbCache.Get()
		result.Available = available
		result.Unavailable = unavailable
		return result, nil
	}

	logger.Infof("USB changed → rescanning devices")

	devs, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		_, supported := findPrinterEndpoint(desc)
		return supported
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open USB devices: %w", err)
	}

	for _, device := range devs {
		libUsb := GetPrinterInfo(device)
		id, err := encodePrinterID(libUsb.Serial, libUsb.Path)
		if err != nil {
			logger.Errorf("failed to encode printer ID: %v", err)
			continue
		}
		if err := config.AddPrinterIfNotExist(id, libUsb.Protocol.String()); err != nil {
			logger.Errorf("Failed to update printer config: %v", err)
		}
		result.Available = append(result.Available, Info{
			Id:    id,
			Name:  libUsb.Name,
			IsLAN: false,
			IP:    "",
			Label: "USB",
		})
		device.Close()
	}

	usbCache.Update(current, result.Available, result.Unavailable)
	return result, nil
}

func GetPrinterInfo(device *gousb.Device) *LibUsbPrinter {
	isPrinter, protocol := isPrinterDevice(device)
	if !isPrinter {
		return nil
	}

	desc := device.Desc

	info := LibUsbPrinter{}
	productName, _ := device.Product()
	vendorName, _ := device.Manufacturer()
	serial, _ := device.SerialNumber()

	if productName == "" {
		productName = fmt.Sprintf("PID: %04X", uint16(desc.Product))
	}

	if vendorName == "" {
		vendorName = fmt.Sprintf("VID: %04X", uint16(desc.Vendor))
	}

	info.Name = fmt.Sprintf("%s %s", vendorName, productName)
	info.Serial = serial
	info.Path = pathToString(desc)
	info.VidPid = fmt.Sprintf("%04X:%04X", uint16(desc.Vendor), uint16(desc.Product))
	info.Protocol = protocol

	logger.Debugf("USB printer: %s (Serial: %s)", info.Name, info.Serial)
	return &info
}

func fingerprintKey(desc *gousb.DeviceDesc) string {
	return fmt.Sprintf("%d-%d-%04X:%04X",
		desc.Bus,
		desc.Address,
		desc.Vendor,
		desc.Product,
	)
}

func findPrinterEndpoint(dev *gousb.DeviceDesc) (EndpointInfo, bool) {
	for cfgNum, cfg := range dev.Configs {
		for _, iFace := range cfg.Interfaces {
			for _, alt := range iFace.AltSettings {
				if ep, ok := matchBulkOutEndpoint(alt); ok {
					return EndpointInfo{
						config:           cfgNum,
						iFace:            iFace.Number,
						alternateSetting: alt.Alternate,
						outEndpoint:      ep,
					}, true
				}
			}
		}
	}
	return EndpointInfo{}, false
}

func matchBulkOutEndpoint(alt gousb.InterfaceSetting) (int, bool) {
	if alt.Class != gousb.ClassPrinter && alt.Class != gousb.ClassVendorSpec {
		return 0, false
	}
	for _, ep := range alt.Endpoints {
		if ep.Direction == gousb.EndpointDirectionOut &&
			ep.TransferType == gousb.TransferTypeBulk {
			return ep.Number, true
		}
	}
	return 0, false
}
