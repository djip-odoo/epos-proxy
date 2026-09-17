//go:build darwin

package bluetooth

import (
	"context"
	"testing"
)

func TestClassicTransport_Darwin(t *testing.T) {
	trans := &ClassicTransport{}

	if trans.Name() != "Classic" {
		t.Errorf("expected transport name 'Classic', got %q", trans.Name())
	}

	if trans.IsAvailable() {
		t.Errorf("expected Classic transport to be unavailable on darwin")
	}

	_, err := trans.Dial(context.Background(), "AA:BB:CC:DD:EE:FF")
	if err == nil {
		t.Errorf("expected error from Dial on darwin Classic transport, got nil")
	}

	_, err = trans.Scan(context.Background())
	if err == nil {
		t.Errorf("expected error from Scan on darwin Classic transport, got nil")
	}

	deps := CheckDependencies()
	if len(deps) != 0 {
		t.Errorf("expected empty dependencies on darwin, got %d", len(deps))
	}
}
