//go:build darwin && !cgo

package bluetooth

import (
	"context"
	"errors"
	"net"
)

type BLETransport struct{}

func (t *BLETransport) Name() string {
	return "BLE"
}

func (t *BLETransport) IsAvailable() bool {
	return false
}

func (t *BLETransport) Dial(ctx context.Context, address string) (net.Conn, error) {
	return nil, errors.New("BT/ble: BLE requires CGO_ENABLED=1 on macOS")
}

func (t *BLETransport) Scan(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	return nil, errors.New("BT/ble: BLE requires CGO_ENABLED=1 on macOS")
}

func isMacOS() bool {
	return true
}
