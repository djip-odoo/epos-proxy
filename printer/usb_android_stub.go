//go:build android

package printer

func ListUSBPrinters() (*Printers, error) {
	return &Printers{
		Available:   make([]Info, 0),
		Unavailable: make([]UnavailableInfo, 0),
	}, nil
}
