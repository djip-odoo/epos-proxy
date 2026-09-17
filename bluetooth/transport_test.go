package bluetooth

import (
	"context"
	"net"
	"runtime"
	"testing"
)

type mockTransport struct {
	name      string
	available bool
	dialFn    func(ctx context.Context, address string) (net.Conn, error)
	scanFn    func(ctx context.Context) ([]BluetoothPrinterInfo, error)
}

func (m *mockTransport) Name() string {
	return m.name
}

func (m *mockTransport) IsAvailable() bool {
	return m.available
}

func (m *mockTransport) Dial(ctx context.Context, address string) (net.Conn, error) {
	if m.dialFn != nil {
		return m.dialFn(ctx, address)
	}
	return nil, nil
}

func (m *mockTransport) Scan(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	if m.scanFn != nil {
		return m.scanFn(ctx)
	}
	return nil, nil
}

func TestTransportInterface(t *testing.T) {
	var _ Transport = (*mockTransport)(nil)
	var _ Transport = (*ClassicTransport)(nil)
	var _ Transport = (*BLETransport)(nil)
}

func TestSupportedTransportsByOS(t *testing.T) {
	transports := supportedTransportsByOS()
	if len(transports) == 0 {
		t.Fatal("expected at least one transport returned by supportedTransportsByOS")
	}

	if runtime.GOOS == "darwin" {
		if transports[0].Name() != "BLE" {
			t.Errorf("expected darwin to have BLE transport as primary, got %s", transports[0].Name())
		}
	} else {
		if transports[0].Name() != "Classic" {
			t.Errorf("expected %s to have Classic transport as primary, got %s", runtime.GOOS, transports[0].Name())
		}
	}
}
