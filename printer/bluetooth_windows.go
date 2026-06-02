//go:build windows

package printer

import (
	"fmt"
	"net"
)

var ErrBluetoothNotSupported = fmt.Errorf("Bluetooth printing is not supported on Windows")

// dialRFCOMM is not implemented on Windows.
func dialRFCOMM(mac string, channel int) (net.Conn, error) {
	return nil, ErrBluetoothNotSupported
}

// sdpDiscoverChannel is not implemented on Windows.
func sdpDiscoverChannel(mac string) (int, error) {
	return 0, ErrBluetoothNotSupported
}

// ScanBluetoothPrinters is not implemented on Windows.
func ScanBluetoothPrinters() ([]BluetoothPrinterInfo, error) {
	return nil, ErrBluetoothNotSupported
}

// dialRFCOMMPlatform is not implemented on Windows.
func dialRFCOMMPlatform(_ string, _ int) (net.Conn, error) {
	return nil, ErrBluetoothNotSupported
}

// globalRFCOMMCache is a no-op stub on Windows.
var globalRFCOMMCache = &rfcommCache{}

// rfcommCache stub for Windows.
type rfcommCache struct{}

func (c *rfcommCache) get(_ string) (*rfcommBinding, bool) { return nil, false }
func (c *rfcommCache) set(_ string, _ *rfcommBinding)      {}

// rfcommBinding stub for Windows.
type rfcommBinding struct {
	DevPath string
	Channel int
	Index   int
}
