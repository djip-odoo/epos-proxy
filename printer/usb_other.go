//go:build !windows

package printer

import (
	"epos-proxy/logger"
	"epos-proxy/util"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func getPrinterFriendlyName(vid, pid string) string {
	return fmt.Sprintf("VID:%s PID:%s", vid, pid)
}

func ListSystemPrinters() ([]SystemUsbPrinter, error) {
	var printers []SystemUsbPrinter
	out, err := exec.Command("lpstat", "-v").Output()

	if err != nil {
		// Check if it's exit status 1 (no printers configured)
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return []SystemUsbPrinter{}, nil
			}
		}
		return nil, err
	}

	statusMap, err := GetSystemPrinterStatusMap()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "device for ") {
			continue
		}

		prefix := "device for "
		lineName := strings.TrimPrefix(line, prefix)
		name, uri, found := strings.Cut(lineName, ":")
		// logger.Infof("Parsing CUPS printer line: %s, %s, %v", name, uri, found)
		if !found {
			logger.Warnf("Invalid line format, skipping: %s", line)
			continue
		}
		name = strings.TrimSpace(name)
		uri = strings.TrimSpace(uri)
		data := parseUSBURI(uri, name)

		info := SystemUsbPrinter{
			Serial:  data.Serial,
			IdName:  name,
			Name:    data.Vendor + " " + data.Product,
			CupsUri: uri,
			Status:  strings.Contains(statusMap[name], "enabled"),
			Type:    getPrinterTypeFromCupsURI(uri),
		}
		printers = append(printers, info)
	}

	return printers, nil
}

func GetSystemPrinterStatusMap() (map[string]string, error) {
	out, err := exec.Command("lpstat", "-p").Output()
	if err != nil {
		return nil, err
	}

	statusMap := make(map[string]string)

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "printer ") {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}

		name := parts[1]
		status := parts[2]

		statusMap[name] = status
	}

	return statusMap, nil
}

func PrintViaSystemPrinter(p *Printer, data []byte) error {
	logger.Debugf("Printing via office printer (CUPS): %s", p.idToString())

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("print-%d.pdf", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp PDF: %w", err)
	}

	cmd := exec.Command("lp", "-d", p.cupsName, tmpFile)
	logger.Debugf("Executing CUPS print command: %v", cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("CUPS print failed: %w", err)
	}

	defer os.Remove(tmpFile)
	logger.Debugf("Successfully sent to office printer %s", p.idToString())
	return nil
}

func EnsureSystemPrinterOpen(p *Printer) error {
	statusMap, err := GetSystemPrinterStatusMap()
	if err != nil {
		logger.Warnf("Failed to get CUPS status: %v", err)
	}
	status, exists := statusMap[p.cupsName]
	if !exists {
		return fmt.Errorf("office printer %s not found in CUPS", p.cupsName)
	}
	if strings.Contains(status, "disabled") || strings.Contains(status, "stopped") {
		return fmt.Errorf("office printer %s is unavailable: %s", p.cupsName, status)
	}
	logger.Debugf("Office printer %s is available with status: %s", p.cupsName, status)
	return nil
}

func AddLanPdfPrinter(ip string) error {
	printerName := fmt.Sprintf("PDF_NET_%s", ip)
	uri := fmt.Sprintf("ipp://%s/ipp/print", ip)

	cmd := exec.Command(
		"lpadmin",
		"-p", printerName,
		"-E",
		"-v", uri,
		"-m", "everywhere",
	)

	if err := cmd.Run(); err != nil {
		if printerExists(printerName) {
			if rmErr := removePrinter(printerName); rmErr != nil {
				return fmt.Errorf("lpadmin failed: %v; cleanup also failed: %v", err, rmErr)
			}
		}
		return fmt.Errorf("failed to add CUPS printer: %w", err)
	}

	return nil
}

func removePrinter(name string) error {
	cmd := exec.Command("lpadmin", "-x", name)
	return cmd.Run()
}

func printerExists(name string) bool {
	cmd := exec.Command("lpstat", "-p", name)
	return cmd.Run() == nil
}

func getPrinterTypeFromCupsURI(uri string) PrinterType {
	if strings.HasPrefix(uri, "usb://") {
		return TypeUNKNOWN
	}
	return TypePDF
}

type USBInfo struct {
	Vendor  string
	Product string
	Serial  string
}

func parseUSBURI(uri string, name string) USBInfo {
	var info USBInfo
	info.Vendor = name

	if !strings.HasPrefix(uri, "usb://") {
		logger.Debugf("CUPS printer %s does not have a serial number in its URI: %s, Name: %s", uri, name)
		return info
	}

	// Remove prefix
	uri = strings.TrimPrefix(uri, "usb://")

	// Split query
	var query string
	if idx := strings.Index(uri, "?"); idx != -1 {
		query = uri[idx+1:]
		uri = uri[:idx]
	}

	// Extract vendor + product
	parts := strings.Split(uri, "/")
	if len(parts) >= 2 {
		info.Vendor = parts[0]
		info.Product = parts[1]
	}

	// Extract serial
	if query != "" {
		for _, q := range strings.Split(query, "&") {
			if strings.HasPrefix(q, "serial=") {
				info.Serial = strings.TrimPrefix(q, "serial=")
			}
		}
	}

	return info
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

		for i, libUsb := range libusbPrinters {
			logger.Debugf("Matching USB[%d]: Serial=%v Path=%v with CUPS Serial=%v Name=%s",
				i, libUsb.Serial, libUsb.Path, sysUsb.Serial, sysUsb.IdName)

			// Serial match
			if libUsb.Serial != "" && sysUsb.Serial != "" && libUsb.Serial == sysUsb.Serial {
				logger.Infof("Matched by SERIAL: %s ↔ %s", libUsb.Serial, sysUsb.Serial)
				id, err := encodePrinterID(libUsb.Serial, libUsb.Path, sysUsb.IdName)
				if err != nil {
					logger.Errorf("Failed to encode printer ID: %v", err)
				} else {
					Type := sysUsb.Type
					if Type == TypeUNKNOWN {
						Type = DetectPrinterType(sysUsb.Name, libUsb.VidPid)
					}
					result.Available = append(result.Available, Info{
						Id:   id,
						Name: sysUsb.Name,
						Type: Type,
					})
				}
				matchedUSB[i] = true
				found = true
				break
			}
		}

		// No USB match → standalone System printer
		// cups list out all the printers so exclude printer which are not TypePDF
		if !found && sysUsb.Type == TypePDF {
			logger.Debugf("No USB match for CUPS printer: %s", sysUsb.IdName)

			id, err := encodePrinterID("", "", sysUsb.IdName)
			if err != nil {
				logger.Errorf("Failed to encode printer ID: %v", err)
				continue
			}
			if !strings.HasPrefix(sysUsb.Name, "PDF_NET_") {
				matchedSystemPrinterName = append(matchedSystemPrinterName, sysUsb.Name)
			}
			if sysUsb.Status {
				result.Available = append(result.Available, Info{
					Id:   id,
					Name: sysUsb.IdName,
					Type: sysUsb.Type,
				})
			} else {
				result.Unavailable = append(result.Unavailable, UnavailableInfo{
					Name:  sysUsb.IdName,
					Error: "Offline",
					Type: sysUsb.Type,
				})
			}
		}
	}

	// standalone libusb printer
	for i, libUsb := range libusbPrinters {
		if matchedUSB[i] {
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
			Id:   id,
			Name: libUsb.Name,
			Type: DetectPrinterType(libUsb.Name, libUsb.VidPid),
		})
	}
	return result, nil
}
