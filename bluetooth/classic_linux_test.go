//go:build linux

package bluetooth

import (
	"sync"
	"testing"
)

func TestClassicTransport_Linux(t *testing.T) {
	trans := &ClassicTransport{}
	if trans.Name() != "Classic" {
		t.Errorf("expected transport name 'Classic', got %q", trans.Name())
	}
}

func TestLinux_RFCOMMBindDisabled(t *testing.T) {
	// Test thread-safety of RFCOMM bind disabling
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = isRFCOMMBindDisabled()
			disableRFCOMMBind()
			_ = isRFCOMMBindDisabled()
		}()
	}
	wg.Wait()

	if !isRFCOMMBindDisabled() {
		t.Errorf("expected RFCOMM bind to be disabled")
	}
}

func TestLinux_CheckDependencies(t *testing.T) {
	deps := CheckDependencies()
	// bluez dependency status can be tested for structure if returned
	for _, d := range deps {
		if d.Name == "" || d.InstallCmd == "" {
			t.Errorf("invalid dependency status struct: %+v", d)
		}
	}
}
