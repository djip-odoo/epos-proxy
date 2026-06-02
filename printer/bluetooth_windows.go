//go:build windows

package printer

import (
	"fmt"
	"net"
)

var ErrBluetoothNotSupported = fmt.Errorf("Bluetooth printing is not supported on Windows")

func dialRFCOMM(_ string, _ int) (net.Conn, error) {
	return nil, ErrBluetoothNotSupported
}

func sdpDiscoverChannel(_ string) (int, error) {
	return 0, ErrBluetoothNotSupported
}

func dialRFCOMMPlatform(_ string, _ int) (net.Conn, error) {
	return nil, ErrBluetoothNotSupported
}

func ScanBluetoothPrinters() ([]BluetoothPrinterInfo, error) {
	return nil, ErrBluetoothNotSupported
}

// ---------------------------------------------------------------------------
// RFCOMM cache stubs
// ---------------------------------------------------------------------------

type rfcommCache struct{}

var globalRFCOMMCache = &rfcommCache{}

func (c *rfcommCache) get(_ string) (*rfcommBinding, bool) { return nil, false }
func (c *rfcommCache) set(_ string, _ *rfcommBinding)      {}
