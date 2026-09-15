package usb

import (
	"sync"
	"testing"

	"epos-proxy/internal/printer"
	"epos-proxy/internal/testutil"
)

func TestBuildSnapshot(t *testing.T) {
	// Keys in different order should yield identical snapshot
	keys1 := []string{"dev-B", "dev-A", "dev-C"}
	keys2 := []string{"dev-C", "dev-B", "dev-A"}

	snap1 := buildSnapshot(keys1)
	snap2 := buildSnapshot(keys2)

	testutil.ExpectedEqual(t, snap1, snap2)
	testutil.ExpectedEqual(t, snap1, "dev-A|dev-B|dev-C")

	// Empty keys
	testutil.ExpectedEqual(t, buildSnapshot([]string{}), "")
}

func TestPrinterCache_Lifecycle(t *testing.T) {
	cache := &printerCache{}

	keys := []string{"dev1", "dev2"}
	testutil.ExpectedTrue(t, cache.HasChanged(keys))

	devices := []printer.Device{
		{Identifier: "p1", Name: "Printer 1", Type: string(printer.TypeReceipt), Online: true},
		{Name: "Printer Bad", ErrorMsg: "permission denied", Online: false},
	}

	cache.Update(keys, devices)

	// Now cache has been updated with keys -> HasChanged should be false
	testutil.ExpectedFalse(t, cache.HasChanged(keys))

	// HasUnavailable should be true
	testutil.ExpectedTrue(t, cache.HasUnavailable())

	// Get should return copy of cached printers
	printers := cache.Get()
	testutil.ExpectedLen(t, printers, 2)
	testutil.ExpectedEqual(t, printers[0].Identifier, "p1")

	// Mutating returned slice should not mutate internal cache
	printers[0].Name = "MUTATED"
	testutil.ExpectedNotEqual(t, cache.Get()[0].Name, "MUTATED")

	// Update with new keys and no unavailable printers
	newKeys := []string{"dev1", "dev2", "dev3"}
	onlyAvailable := []printer.Device{
		{Identifier: "p1", Name: "Printer 1", Type: string(printer.TypeReceipt), Online: true},
	}
	cache.Update(newKeys, onlyAvailable)

	testutil.ExpectedFalse(t, cache.HasUnavailable())
	testutil.ExpectedFalse(t, cache.HasChanged(newKeys))
}

func TestPrinterCache_ConcurrentAccess(t *testing.T) {
	cache := &printerCache{}
	var wg sync.WaitGroup

	for i := range 20 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			keys := []string{"dev1", "dev2"}
			if workerID%2 == 0 {
				cache.Update(keys, []printer.Device{{Identifier: "p1"}})
			} else {
				_ = cache.HasChanged(keys)
				_ = cache.Get()
				_ = cache.HasUnavailable()
			}
		}(i)
	}

	wg.Wait()
}
