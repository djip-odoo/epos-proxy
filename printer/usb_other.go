//go:build !windows

package printer

import (
	"epos-proxy/logger"
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

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "device for ") {
			continue
		}

		lineName := strings.TrimPrefix(line, "device for ")
		name, uri, found := strings.Cut(lineName, ":")
		if !found {
			logger.Warnf("Invalid line format, skipping: %s", line)
			continue
		}

		name = strings.TrimSpace(name)
		uri = strings.TrimSpace(uri)
		// we will only use locally added printer
		if !strings.HasPrefix(uri, "usb://") &&
		!strings.HasPrefix(uri, "ipp://") &&
		!strings.HasPrefix(uri, "ipps://") {
			continue
		}
			
		data := parseUSBURI(uri)
		printers = append(printers, SystemUsbPrinter{
			Serial:  data.Serial,
			IdName:  name,
			CupsUri: uri,
			Status:  strings.Contains(statusMap[name], "enabled"),
			Type:    getPrinterTypeFromCupsURI(uri),
		})
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
	name := p.cupsName
	out, err := exec.Command("lpstat", "-p", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get printer status: %v (%s)   name: %v", err, string(out), p)
	}

	status := string(out)
	if strings.Contains(status, "disabled") || strings.Contains(status, "stopped") {
		return fmt.Errorf("office printer %s is unavailable: %s", name, status)
	}

	logger.Debugf("Office printer %s is available: %s", name, status)
	return nil
}

func AddLanPdfPrinter(ip string) error {
	printerName := fmt.Sprintf("PDF_NET_%s", ip)
	uri := fmt.Sprintf("ipp://%s/ipp/print", ip)

	cmd := exec.Command("lpadmin", "-p", printerName, "-E", "-v", uri, "-m", "everywhere")
	if err := cmd.Run(); err != nil {
		if printerExists(printerName) {
			if rmErr := DeleteSystemPrinter(printerName); rmErr != nil {
				return fmt.Errorf("lpadmin failed: %v; cleanup also failed: %v", err, rmErr)
			}
		}
		return fmt.Errorf("failed to add CUPS printer: %w", err)
	}

	return nil
}

func DeleteSystemPrinter(name string) error {
	cmd := exec.Command("lpadmin", "-x", name)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete printer %s: %v (%s)", name, err, string(out))
	}

	return nil
}

func printerExists(name string) bool {
	return exec.Command("lpstat", "-p", name).Run() == nil
}

func getPrinterTypeFromCupsURI(uri string) PrinterType {
	if strings.HasPrefix(uri, "usb://") {
		return TypeUNKNOWN
	}
	return TypePDF
}

type USBInfo struct {
	VendorName  string
	ProductName string
	Serial      string
}

func parseUSBURI(uri string) USBInfo {
	var info USBInfo
	var query string
	uri = strings.TrimPrefix(uri, "usb://")
	if idx := strings.Index(uri, "?"); idx != -1 {
		query = uri[idx+1:]
		uri = uri[:idx]
	}

	// parts := strings.Split(uri, "/")
	// if len(parts) >= 2 {
	// 	info.VendorName = parts[0]
	// 	info.ProductName = parts[1]
	// }

	if query != "" {
		for _, q := range strings.Split(query, "&") {
			if strings.HasPrefix(q, "serial=") {
				info.Serial = strings.TrimPrefix(q, "serial=")
			}
		}
	}

	return info
}
