package printer

import (
	"fmt"
	"runtime"
	"strings"

	"epos-proxy/logger"
	"epos-proxy/util"

	"github.com/google/gousb"
)

func ListUSBPrinters() (*Printers, error) {
	logger.Debug("Starting USB printer detection")

	systemUsbPrinters, err := listSystemPrinters()
	if err != nil {
		return nil, fmt.Errorf("Failed to get System printers , %v", err)
	}

	libusbPrinters, unavailable, err := listLibUsbPrinters()
	if err != nil {
		return nil, fmt.Errorf("Failed to get Usb printers, %v", err)
	}

	result, err := mergePrinters(systemUsbPrinters, libusbPrinters, unavailable)
	if err != nil {
		return nil, fmt.Errorf("Failed to merge printer list, failed: %v", err)
	}
	return result, nil
}

func listLibUsbPrinters() ([]LibUsbPrinter, []UnavailableInfo, error) {
	ctx := gousb.NewContext()
	defer func(ctx *gousb.Context) {
		_ = ctx.Close()
	}(ctx)

	// First list all  without opening devices, to avoid permission errors on some platforms
	var descriptors []gousb.DeviceDesc
	_, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if _, supported := findPrinterEndpoint(desc); supported {
			descriptors = append(descriptors, *desc)
		}
		return false
	})

	if err != nil {
		return nil, nil, fmt.Errorf("failed to enumerate USB devices: %w", err)
	}

	var printers []LibUsbPrinter
	var unavailable []UnavailableInfo

	for _, desc := range descriptors {
		info, err := getPrinterInfo(ctx, &desc)
		if err != nil {
			// Device is not accessible, likely due to permissions / drivers.
			vid := fmt.Sprintf("%04X", uint16(desc.Vendor))
			pid := fmt.Sprintf("%04X", uint16(desc.Product))
			unavailable = append(unavailable, UnavailableInfo{
				Name:  getPrinterFriendlyName(vid, pid),
				Error: err.Error(),
				Type:  TypeANY,
			})
		} else if info != nil {
			logger.Debugf("Found available USB printer: %s (Serial: %s)", info.Name, info.Serial)
			printers = append(printers, *info)
		}
	}
	return printers, unavailable, nil
}

func getPrinterInfo(ctx *gousb.Context, descToFind *gousb.DeviceDesc) (*LibUsbPrinter, error) {
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
	productName = util.Ternary(productName == "", fmt.Sprintf("PID: %04X", uint16(descToFind.Product)), productName)
	vendorName = util.Ternary(vendorName == "", fmt.Sprintf("VID: %04X", uint16(descToFind.Vendor)), vendorName)

	info.Name = fmt.Sprintf("%s %s", vendorName, productName)
	info.Serial, _ = device.SerialNumber()
	info.Path = pathToString(descToFind)
	info.Type = libUsbDetectPrinterType(device)
	info.VidPid = fmt.Sprintf("%04X:%04X", uint16(descToFind.Vendor), uint16(descToFind.Product))
	return info, nil
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

func mergePrinters(systemPrinters []SystemUsbPrinter, libusbPrinters []LibUsbPrinter, unavailable []UnavailableInfo) (*Printers, error) {
	result := &Printers{
		Available:   make([]Info, 0),
		Unavailable: make([]UnavailableInfo, 0),
	}
	result.Unavailable = append(result.Unavailable, unavailable...)

	matchedUSB := make(map[int]bool)
	matchedSystemPrinterName := make([]string, 0, len(systemPrinters))
	for _, sysUsb := range systemPrinters {
		found := false

		if runtime.GOOS != "windows" {
			for i, libUsb := range libusbPrinters {
				logger.Debugf("Matching USB[%d]: Serial=%v Path=%v with CUPS Serial=%v Name=%s",
					i, libUsb.Serial, libUsb.Path, sysUsb.Serial, sysUsb.IdName)

				// Serial match
				if libUsb.Serial != "" && sysUsb.Serial != "" && libUsb.Serial == sysUsb.Serial {
					logger.Debugf("Matched by SERIAL: %s ↔ %s", libUsb.Serial, sysUsb.Serial)
					if id, err := encodePrinterID(libUsb.Serial, libUsb.Path, sysUsb.IdName); err == nil {
						result.Available = append(result.Available, Info{
							Id:      id,
							Name:    sysUsb.IdName,
							Variant: string(TypeANY),
							Type:    detectPrinterType(libUsb.VidPid, libUsb.Type),
						})
					} else {
						logger.Errorf("Failed to encode printer ID: %v", err)
					}
					matchedUSB[i] = true
					found = true
					break
				}
			}
		}

		// No USB match → standalone System printer
		if !found {
			logger.Debugf("No USB match for CUPS printer: %s", sysUsb.IdName)

			id, err := encodePrinterID(sysUsb.Serial, "", sysUsb.IdName)
			if err != nil {
				logger.Errorf("Failed to encode printer ID: %v", err)
				continue
			}

			if !strings.HasPrefix(sysUsb.IdName, "PDF_NETWORK_") {
				matchedSystemPrinterName = append(matchedSystemPrinterName, sysUsb.IdName)
			}

			if sysUsb.Status {
				result.Available = append(result.Available, Info{
					Id:      id,
					Name:    sysUsb.IdName,
					Type:    sysUsb.Type,
					Variant: string(TypeOFFICE),
					IsLAN:   sysUsb.IsLAN,
					IP:      sysUsb.IP,
				})
			} else {
				result.Unavailable = append(result.Unavailable, UnavailableInfo{
					Name:  sysUsb.IdName,
					Error: "Offline",
					Type:  sysUsb.Type,
				})
			}
		}
	}
	appendLibusbEposPrinterOnly(libusbPrinters, matchedUSB, result, matchedSystemPrinterName)
	return result, nil
}

func appendLibusbEposPrinterOnly(libusbPrinters []LibUsbPrinter, matchedUSB map[int]bool, result *Printers, matchedSystemPrinterName []string) {
	for i, libUsb := range libusbPrinters {
		if matchedUSB[i] {
			continue
		}

		// libUsb.Type detected by COMMANDS supported by printer
		if libUsb.Type == TypeOFFICE {
			continue
		}

		matched := false
		// skip those which are normal standard printer
		for _, name := range matchedSystemPrinterName {
			if name == "" {
				continue
			}
			if util.IsMatch(libUsb.Name, name) {
				matched = true
				logger.Infof("Printer matched by fuzzy name: %s, %s ", libUsb.Name, name)
				break
			}
		}
		if matched {
			continue
		}
		logger.Debugf("USB-only printer detected: %s", libUsb.Name)

		id, err := encodePrinterID(libUsb.Serial, libUsb.Path, "")
		if err != nil {
			logger.Errorf("Failed to encode printer ID: %v", err)
			continue
		}

		result.Available = append(result.Available, Info{
			Id:      id,
			Name:    libUsb.Name,
			Variant: string(TypeTHERMAL),
			Type:    detectPrinterType(libUsb.VidPid, libUsb.Type),
		})
	}
}
