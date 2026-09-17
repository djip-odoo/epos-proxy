package bluetooth

import (
	"path/filepath"
	"sync"
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
	BTManager.cache.delete(normalizeAddress(mac))
	ch = BTManager.GetCachedRFCOMMChannel(mac)
	testutil.ExpectedEqual(t, ch, 0)
}

func TestBluetoothManager_TargetedCacheDeletion(t *testing.T) {
	InitBluetoothManager(&config.Manager{})

	const mac = "AA:BB:CC:DD:EE:FF"
	binding1 := &rfcommBinding{DevPath: "/dev/rfcomm0", Channel: 1, Index: 0}
	binding2 := &rfcommBinding{DevPath: "raw", Channel: 2, Index: -1}

	// 1. Set channel 1 binding
	BTManager.setBinding(mac, binding1)

	// 2. Try deleting with non-matching binding (should not delete)
	BTManager.deleteBindingIfMatching(mac, binding2)
	devPath, ch, ok := BTManager.GetCachedBinding(mac)
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, ch, 1)
	testutil.ExpectedEqual(t, devPath, "/dev/rfcomm0")

	// 3. Try deleting with matching binding (should delete)
	BTManager.deleteBindingIfMatching(mac, binding1)
	_, _, ok = BTManager.GetCachedBinding(mac)
	testutil.ExpectedFalse(t, ok)

	// 4. Delete by channel
	BTManager.setBinding(mac, binding2)
	BTManager.deleteBindingForChannel(mac, 1) // Non-matching channel -> still present
	_, ch, ok = BTManager.GetCachedBinding(mac)
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, ch, 2)

	BTManager.deleteBindingForChannel(mac, 2) // Matching channel -> deleted
	_, _, ok = BTManager.GetCachedBinding(mac)
	testutil.ExpectedFalse(t, ok)
}

func TestBluetoothManager_ConcurrentCacheAccess(t *testing.T) {
	InitBluetoothManager(&config.Manager{})

	const mac = "12:34:56:78:9A:BC"
	var wg sync.WaitGroup

	for i := range 20 {
		wg.Add(3)
		ch := (i % 8) + 1

		// Concurrent writers
		go func(channel int) {
			defer wg.Done()
			BTManager.setBinding(mac, &rfcommBinding{
				DevPath: "raw",
				Channel: channel,
				Index:   -1,
			})
		}(ch)

		// Concurrent readers
		go func() {
			defer wg.Done()
			_ = BTManager.GetCachedRFCOMMChannel(mac)
			_, _, _ = BTManager.GetCachedBinding(mac)
		}()

		// Concurrent targeted deleters
		go func(channel int) {
			defer wg.Done()
			BTManager.deleteBindingIfMatching(mac, &rfcommBinding{
				DevPath: "raw",
				Channel: channel,
				Index:   -1,
			})
		}(ch)
	}

	wg.Wait()
}

func TestBluetoothManager_PrinterManagement(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	InitBluetoothManager(cfg)

	// 1. Invalid address validation error
	err = BTManager.AddBluetoothPrinter("not-valid-address", "Test Printer")
	testutil.ExpectedError(t, err)

	// 2. Empty address
	err = BTManager.AddBluetoothPrinter("", "Test Printer")
	testutil.ExpectedError(t, err)

	// 3. Remove printer
	const mac = "AA:BB:CC:DD:EE:FF"
	err = cfg.AddBluetoothPrinter(mac, "Test Printer")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedLen(t, cfg.GetBluetoothPrinters(), 1)

	// Set a binding in cache
	BTManager.setBinding(mac, &rfcommBinding{DevPath: "/dev/rfcomm0", Channel: 1, Index: 0})
	testutil.ExpectedEqual(t, BTManager.GetCachedRFCOMMChannel(mac), 1)

	// Remove via BTManager
	err = BTManager.RemoveBluetoothPrinter(mac)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedLen(t, cfg.GetBluetoothPrinters(), 0)
	testutil.ExpectedEqual(t, BTManager.GetCachedRFCOMMChannel(mac), 0)
}
