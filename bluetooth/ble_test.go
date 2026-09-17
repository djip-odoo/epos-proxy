//go:build !darwin || cgo

package bluetooth

import (
	"testing"
)

func TestBLETransport_Name(t *testing.T) {
	trans := &BLETransport{}
	if trans.Name() != "BLE" {
		t.Errorf("expected BLE transport name 'BLE', got %q", trans.Name())
	}
}
