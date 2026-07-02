//go:build android

package printer

import (
	"errors"
)

func (p *Printer) writeUSB(data []byte) error {
	return errors.New("USB printing is not supported on Android")
}

func (p *Printer) ensureOpenUSBLocked() error {
	return errors.New("USB printing is not supported on Android")
}

func (p *Printer) closeUSBDeviceLocked() {
}
