package printer

import (
	"errors"
	"sync"
	"testing"
	"time"

	"epos-proxy/internal/testutil"
)

type mockPrinter struct {
	id      string
	mu      sync.Mutex
	written [][]byte
	closed  bool
}

func (m *mockPrinter) ID() string {
	return m.id
}

func (m *mockPrinter) Write(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]byte, len(data))
	copy(copied, data)
	m.written = append(m.written, copied)
	return nil
}

func (m *mockPrinter) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
}

type mockDriver struct {
	name        string
	devices     []Device
	unavailable []UnavailableDevice
	printers    []Printer
}

func (d *mockDriver) Name() string { return d.name }
func (d *mockDriver) Discover() ([]Device, []UnavailableDevice, error) {
	return d.devices, d.unavailable, nil
}

func (d *mockDriver) Open(id string) (Printer, error) {
	for _, p := range d.printers {
		if p.ID() == id {
			return p, nil
		}
	}
	return nil, ErrNotFound
}

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	testutil.ExpectedNotNil(t, mgr)
	testutil.ExpectedNotNil(t, mgr.workers)
}

func TestWorker_QueueFull(t *testing.T) {
	mp := &mockPrinter{id: "mock1"}
	w := &printerWorker{
		printer: mp,
		jobs:    make(chan Job, 2),
	}
	// Note: We do not start w.loop() here so we can fill the queue deterministically

	// Fill the queue to QueueSize (2)
	for range 2 {
		err := w.Enqueue(func(p Printer) JobResult {
			return JobResult{OK: true}
		}, nil)
		testutil.ExpectedNoError(t, err)
	}

	// 3rd enqueue must return ErrQueueFull
	err := w.Enqueue(func(p Printer) JobResult {
		return JobResult{OK: true}
	}, nil)

	testutil.ExpectedTrue(t, errors.Is(err, ErrQueueFull))
}

func TestManager_Print_And_WriteAsync(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	mp := &mockPrinter{id: "printer-123"}
	mgr.Register(mp)

	// Synchronous Print
	payload := []byte("Hello World Print")
	err := mgr.Print("printer-123", payload)
	testutil.ExpectedNoError(t, err)

	// Asynchronous WriteAsync
	replyChan, err := mgr.WriteAsync("printer-123", []byte("Async Payload"))
	testutil.ExpectedNoError(t, err)

	select {
	case res := <-replyChan:
		testutil.ExpectedTrue(t, res.OK)
		testutil.ExpectedNoError(t, res.Err)
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out waiting for WriteAsync reply")
	}

	mp.mu.Lock()
	testutil.ExpectedLen(t, mp.written, 2)
	testutil.ExpectedBytesEqual(t, mp.written[0], payload)
	testutil.ExpectedBytesEqual(t, mp.written[1], []byte("Async Payload"))
	mp.mu.Unlock()
}

func TestManager_Get_And_Unregister(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	// 1. Unregistered printer returns error
	_, err := mgr.Get("unregistered-id")
	testutil.ExpectedError(t, err)

	// 2. Register printer
	mp := &mockPrinter{id: "p1"}
	mgr.Register(mp)

	w1, err := mgr.Get("p1")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, w1)

	// 3. Reusing same worker
	w2, err := mgr.Get("p1")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, w1, w2)

	// 4. Unregister
	mgr.Unregister("p1")
	_, err = mgr.Get("p1")
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, mp.closed)
}

func TestManager_Discover(t *testing.T) {
	p1 := &mockPrinter{id: "p1"}
	drv := &mockDriver{
		name: "MockDriver",
		devices: []Device{
			{Identifier: "p1", Name: "Mock P1", Type: string(TypeReceipt), Online: true},
		},
		unavailable: []UnavailableDevice{
			{Name: "Mock P2", ErrorMsg: "Access Denied"},
		},
		printers: []Printer{p1},
	}

	mgr := NewManager(drv)
	defer mgr.Close()

	res := mgr.Discover()
	testutil.ExpectedLen(t, res.Printers, 1)
	testutil.ExpectedLen(t, res.UnavailablePrinters, 1)
	testutil.ExpectedEqual(t, res.Printers[0].Identifier, "p1")
	testutil.ExpectedEqual(t, res.UnavailablePrinters[0].Name, "Mock P2")

	// Verify no printer worker was created during Discover
	mgr.mu.RLock()
	testutil.ExpectedEqual(t, len(mgr.workers), 0)
	mgr.mu.RUnlock()

	// Verify printer worker is lazily created on demand via Get
	w, err := mgr.Get("p1")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, w)

	mgr.mu.RLock()
	testutil.ExpectedEqual(t, len(mgr.workers), 1)
	mgr.mu.RUnlock()
}
