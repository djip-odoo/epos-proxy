package bluetooth

import (
	"path/filepath"
	"testing"

	"epos-proxy/internal/config"
	"epos-proxy/internal/testutil"
)

func TestInitBluetoothManager(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Manager{}
	_ = tempDir

	InitBluetoothManager(cfg)
	testutil.ExpectedNotNil(t, BTManager)
	testutil.ExpectedNotNil(t, BTManager.cache)
}

func TestBluetoothManager_CacheOperations(t *testing.T) {
	tempDir := t.TempDir()
	cm := &config.Manager{}
	_ = filepath.Join(tempDir, "config.json")
	InitBluetoothManager(cm)

	const mac = "11:22:33:44:55:66"

	// 1. Initial lookup (empty)
	ch := BTManager.GetCachedRFCOMMChannel(mac)
	testutil.ExpectedEqual(t, ch, 0)

	devPath, ch, ok := BTManager.GetCachedBinding(mac)
	testutil.ExpectedFalse(t, ok)
	testutil.ExpectedEqual(t, devPath, "")
	testutil.ExpectedEqual(t, ch, 0)

	// 2. Set binding
	binding := &rfcommBinding{
		DevPath: "/dev/rfcomm0",
		Channel: 1,
		Index:   0,
	}
	BTManager.setBinding(mac, binding)

	// 3. Retrieve binding
	ch = BTManager.GetCachedRFCOMMChannel(mac)
	testutil.ExpectedEqual(t, ch, 1)

	devPath, ch, ok = BTManager.GetCachedBinding(mac)
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, devPath, "/dev/rfcomm0")
	testutil.ExpectedEqual(t, ch, 1)

	// 4. Address normalization in cache lookup (lowercase / hyphen format)
	const macHyphen = "11-22-33-44-55-66"
	ch = BTManager.GetCachedRFCOMMChannel(macHyphen)
	testutil.ExpectedEqual(t, ch, 1)

	devPath, ch, ok = BTManager.GetCachedBinding(macHyphen)
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, devPath, "/dev/rfcomm0")
	testutil.ExpectedEqual(t, ch, 1)

	// 5. Update same binding (noop check)
	BTManager.setBinding(mac, &rfcommBinding{
		DevPath: "/dev/rfcomm0",
		Channel: 1,
		Index:   0,
	})

	// 6. Delete from cache
	BTManager.cache.delete(NormalizeAddress(mac))
	ch = BTManager.GetCachedRFCOMMChannel(mac)
	testutil.ExpectedEqual(t, ch, 0)
}

func TestIsBluetoothAdapterActive(t *testing.T) {
	// Should execute without panicking
	_ = IsBluetoothAdapterActive()
}
