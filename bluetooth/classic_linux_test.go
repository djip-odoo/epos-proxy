//go:build linux

package bluetooth

import (
	"sync"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestLinux_RFCOMMBindDisabled(t *testing.T) {
	// Test thread-safety of RFCOMM bind disabling
	resetRFCOMMBindDisabled()
	testutil.ExpectedFalse(t, isRFCOMMBindDisabled(), "expected RFCOMM bind to not be disabled initially")

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			_ = isRFCOMMBindDisabled()
			disableRFCOMMBind()
			_ = isRFCOMMBindDisabled()
		})
	}
	wg.Wait()

	testutil.ExpectedTrue(t, isRFCOMMBindDisabled(), "expected RFCOMM bind to be in cooldown")

	resetRFCOMMBindDisabled()
	testutil.ExpectedFalse(t, isRFCOMMBindDisabled(), "expected RFCOMM bind to be re-enabled after reset")
}

func TestLinux_CheckDependencies(t *testing.T) {
	deps := checkDependencies()
	// bluez dependency status can be tested for structure if returned
	for _, d := range deps {
		testutil.ExpectedTrue(t, d.Name != "", "dependency Name should not be empty")
		testutil.ExpectedTrue(t, d.InstallCmd != "", "dependency InstallCmd should not be empty")
	}
}
