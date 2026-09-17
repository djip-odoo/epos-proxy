//go:build darwin

package bluetooth

import (
	"context"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestClassicTransport_Darwin(t *testing.T) {
	trans := &classicTransport{}

	testutil.ExpectedEqual(t, trans.name(), "Classic")
	testutil.ExpectedFalse(t, trans.isAvailable(), "expected Classic transport to be unavailable on darwin")

	_, err := trans.dial(context.Background(), "AA:BB:CC:DD:EE:FF")
	testutil.ExpectedError(t, err, "expected error from Dial on darwin Classic transport")

	_, err = trans.scan(context.Background())
	testutil.ExpectedError(t, err, "expected error from Scan on darwin Classic transport")

	deps := checkDependencies()
	testutil.ExpectedLen(t, deps, 0, "expected empty dependencies on darwin")
}
