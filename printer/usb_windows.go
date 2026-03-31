//go:build windows

package printer

import (
	"fmt"
	"strings"

	"epos-proxy/logger"

	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows/registry"
)

type Win32_PnPEntity struct {
	Name     string
	DeviceID string
}

func getPrinterFriendlyName(vid, pid string) string {
	logger.Debugf("Attempting to get name for VID:%s PID:%s", vid, pid)
	name, _, _ := findPnPDeviceNameByVidPid(vid, pid)

	// If WMI returned something useful (not generic "USB..." name) use it
	if name != "" && !strings.Contains(strings.ToUpper(name), "USB") {
		return name
	}

	// Fallback: look up clean model name from USBPRINT registry
	logger.Debug("Falling back to registry lookup for printer friendly name")
	if regName := findUSBPrintModel(vid, pid); regName != "" {
		return regName
	}
	logger.Debug("Using generic name for printer")

	return fmt.Sprintf("USB ID: %s %s", vid, pid)
}

// ── WMI lookup ────────────────────────────────────────────────────────────────

func findPnPDeviceNameByVidPid(vid, pid string) (string, string, error) {
	vid = strings.ToUpper(strings.TrimSpace(vid))
	pid = strings.ToUpper(strings.TrimSpace(pid))

	if len(vid) == 0 || len(pid) == 0 {
		return "", "", fmt.Errorf("vid and pid must not be empty")
	}

	var entities []Win32_PnPEntity

	logger.Debugf("Querying WMI for device with VID_%s", vid)
	q := fmt.Sprintf("SELECT Name, DeviceID FROM Win32_PnPEntity WHERE DeviceID LIKE '%%VID_%s%%'", vid)
	if err := wmi.Query(q, &entities); err != nil {
		return "", "", fmt.Errorf("WMI query failed: %w", err)
	}

	needle1 := "VID_" + vid
	needle2 := "PID_" + pid
	needleConcat := "VID_" + vid + "&PID_" + pid

	for _, e := range entities {
		logger.Debugf("Checking WMI entity: Name=%s, DeviceID=%s", e.Name, e.DeviceID)
		id := strings.ToUpper(e.DeviceID)
		if strings.Contains(id, needleConcat) {
			return e.Name, e.DeviceID, nil
		}
		if strings.Contains(id, needle1) && strings.Contains(id, needle2) {
			return e.Name, e.DeviceID, nil
		}
	}
	logger.Debugf("No matching WMI entity found for VID:%s PID:%s", vid, pid)

	return "", "", nil
}

// ── Registry fallback ─────────────────────────────────────────────────────────

// findUSBPrintModel looks up the clean model name from the USBPRINT registry
// by linking Enum\USB (ParentIdPrefix value) → Enum\USBPRINT (instance key name).
func findUSBPrintModel(vid, pid string) string {
	vid = strings.ToUpper(vid)
	pid = strings.ToUpper(pid)

	prefix := readParentIdPrefix(vid, pid)
	if prefix == "" {
		return ""
	}
	logger.Debugf("Found ParentIdPrefix: %s for VID:%s PID:%s", prefix, vid, pid)
	prefixUpper := strings.ToUpper(prefix)

	logger.Debug("Opening registry key for USBPRINT models")
	root, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Enum\USBPRINT`,
		registry.ENUMERATE_SUB_KEYS,
	)
	if err != nil {
		logger.Errorf("Failed to open USBPRINT registry key: %v", err)
		return ""
	}
	defer root.Close()

	models, _ := root.ReadSubKeyNames(-1)
	for _, model := range models {
		modelKey, err := registry.OpenKey(root, model, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			logger.Warnf("Failed to open registry subkey %s: %v", model, err)
			continue
		}
		instances, _ := modelKey.ReadSubKeyNames(-1)
		modelKey.Close()

		for _, instance := range instances {
			logger.Debugf("Checking USBPRINT instance %s for prefix %s", instance, prefixUpper)
			if strings.HasPrefix(strings.ToUpper(instance), prefixUpper) {
				return model // e.g. "EPSONTM-T30II" — clean, no AC25 suffix
			}
		}
	}
	return ""
}

// readParentIdPrefix reads the ParentIdPrefix value from the connected
// device instance under Enum\USB\VID_xxxx&PID_xxxx.
func readParentIdPrefix(vid, pid string) string {
	keyPath := fmt.Sprintf(
		`SYSTEM\CurrentControlSet\Enum\USB\VID_%s&PID_%s`,
		vid, pid,
	)
	logger.Debugf("Opening registry key for USB device: %s", keyPath)
	devKey, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		keyPath,
		registry.ENUMERATE_SUB_KEYS,
	)
	if err != nil {
		logger.Warnf("Failed to open USB device registry key %s: %v", keyPath, err)
		return ""
	}
	defer devKey.Close()

	instances, _ := devKey.ReadSubKeyNames(-1)
	for _, instance := range instances {
		instKey, err := registry.OpenKey(devKey, instance, registry.QUERY_VALUE)
		if err != nil {
			logger.Warnf("Failed to open instance registry key %s\\%s: %v", keyPath, instance, err)
			continue
		}
		flags, _, _ := instKey.GetIntegerValue("ConfigFlags")
		prefix, _, err := instKey.GetStringValue("ParentIdPrefix")
		instKey.Close()

		if err == nil && prefix != "" && flags == 0 {
			logger.Debugf("Found ParentIdPrefix %s for instance %s\\%s", prefix, keyPath, instance)
			return prefix
		}
	}
	return ""
}

type Win32_Printer struct {
	Name        string
	DriverName  string
	PortName    string
	WorkOffline bool
	Default     bool
}

func ListSystemPrinters() ([]Info, error) {
	var printersWMI []Win32_Printer

	query := "SELECT Name, DriverName, PortName, WorkOffline, Default FROM Win32_Printer"
	err := wmi.Query(query, &printersWMI)
	if err != nil {
		return nil, err
	}

	var printers []Info

	for _, p := range printersWMI {
		serial := "" // ❌ usually not available on Windows

		info := Info{
			Serial:      serial,
			ProductName: p.DriverName,
			VendorName:  "Windows",
			CupsName:    p.Name, // keep same field for consistency
		}

		printers = append(printers, info)
	}

	return printers, nil
}

func GetSystemPrinterStatusMap() (map[string]string, error) {
	var printersWMI []Win32_Printer

	query := "SELECT Name, WorkOffline FROM Win32_Printer"
	err := wmi.Query(query, &printersWMI)
	if err != nil {
		return nil, err
	}

	statusMap := make(map[string]string)

	for _, p := range printersWMI {
		status := "online"
		if p.WorkOffline {
			status = "offline"
		}

		statusMap[p.Name] = status
	}

	return statusMap, nil
}
