package config

type KnownPrinter struct {
	VID  string `json:"vid"`
	PID  string `json:"pid"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// Some thermal printers do not expose the standard USB printer class (0x07)
// and instead use vendor-specific interfaces. These known VID:PID pairs are
// treated as printers even when printer-class detection fails.
var defaultKnownPrinters = []KnownPrinter{
	// Receipt printers
	{VID: "2aaf", PID: "6015", Name: "Essae thermal"},      // 2aaf:6015
	{VID: "04b8", PID: "0e32", Name: "Epson thermal"},      // 04b8:0e32
	{VID: "04b8", PID: "0202", Name: "Epson thermal"},      // 04b8:0202
	{VID: "04b8", PID: "0203", Name: "Epson thermal"},      // 04b8:0203
	{VID: "04b8", PID: "0e27", Name: "Epson TM-T83III"},    // 04b8:0e27
	{VID: "2d84", PID: "c7c8", Name: "Zhuhai Poskey"},      // 2d84:c7c8
	{VID: "4b43", PID: "3830", Name: "Caysn"},              // 4b43:3830
	{VID: "0483", PID: "5720", Name: "STMicroelectronics"}, // 0483:5720
	// Label printers
	{VID: "0a5f", PID: "0187", Name: "Zebra ZD421", Type: "label"}, // 0a5f:0187
	{VID: "195f", PID: "0001", Name: "Godex G500", Type: "label"},  // 195f:0001
}

func DefaultKnownPrinters() []KnownPrinter {
	cp := make([]KnownPrinter, len(defaultKnownPrinters))
	copy(cp, defaultKnownPrinters)
	return cp
}

func (cm *Manager) GetKnownPrinters() []KnownPrinter {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.Data.KnownPrinters
}
