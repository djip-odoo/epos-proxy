//go:build android && !cgo

package printer

import (
	"errors"
)

func callJavaPrinter(method string, arg string) string {
	return ""
}

func (p *Printer) writeUSB(data []byte) error {
	return errors.New("USB printing requires CGO on Android")
}

func (p *Printer) ensureOpenUSBLocked() error {
	return errors.New("USB printing requires CGO on Android")
}

func (p *Printer) closeUSBDeviceLocked() {
}
