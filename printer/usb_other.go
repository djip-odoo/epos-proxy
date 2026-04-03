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

func ListSystemPrinters() ([]Info, error) {
	var printers []Info
	out, err := exec.Command("lpstat", "-v").Output()

	if err != nil {
		return printers, err
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
		if !found {
			logger.Warnf("Invalid line format, skipping: %s", line)
			continue
		}
		name = strings.TrimSpace(name)
		uri = strings.TrimSpace(uri)
		serial := ""

		if strings.Contains(uri, "serial=") {
			parts := strings.Split(uri, "serial=")
			serialPart := parts[1]
			serial = strings.Split(serialPart, "&")[0]
			logger.Debugf("CUPS printer %s serial: %s", name, serial)
		} else {
			logger.Debugf("CUPS printer %s does not have a serial number in its URI", name)
		}
		info := Info{
			Serial:      serial,
			ProductName: name,
			VendorName:  "",
			CupsName:    name,
		}

		info.Type = DetectPrinterType(line)
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
