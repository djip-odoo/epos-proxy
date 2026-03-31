//go:build !windows

package printer

import (
	"epos-proxy/logger"
	"fmt"
	"os/exec"
	"strings"
)

func getPrinterFriendlyName(vid, pid string) string {
	return fmt.Sprintf("VID:%s PID:%s", vid, pid)
}

func ListSystemPrinters() ([]Info, error) {
	out, err := exec.Command("lpstat", "-v").Output()
	if err != nil {
		return nil, err
	}

	var printers []Info
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "device for ") {
			continue
		}

		// Split only on the first colon
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

		// Optional: extract serial from URI safely
		if strings.Contains(uri, "serial=") {
			parts := strings.Split(uri, "serial=")
			serialPart := parts[1]
			// Stop at next & if present
			serial := strings.Split(serialPart, "&")[0]
			logger.Debugf("CUPS printer %s serial: %s", name, serial)
		} else {
			logger.Debugf("CUPS printer %s does not have a serial number in its URI", name)
		}
		info := Info{
			Serial:      serial,
			ProductName: name,
			VendorName:  "CUPS",
			CupsName:    name,
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

		// "printer NAME is idle. enabled since ..."
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
