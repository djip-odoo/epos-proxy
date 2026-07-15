//go:build !darwin || cgo

package bluetooth

import (
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"epos-proxy/logger"
	"epos-proxy/util"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter
var adapterOnce sync.Once
var adapterEnableErr error

func enableAdapter() error {
	adapterOnce.Do(func() {
		adapterEnableErr = adapter.Enable()
		if adapterEnableErr != nil {
			logger.Errorf("BT/ble: failed to enable bluetooth adapter: %v", adapterEnableErr)
		} else {
			logger.Infof("BT/ble: bluetooth adapter enabled successfully")
		}
	})
	return adapterEnableErr
}

func init() {
	go func() {
		time.Sleep(1 * time.Second)
		_ = enableAdapter()
	}()
}

type bleCacheEntry struct {
	ServiceUUID        string
	CharacteristicUUID string
}

type bleCache struct {
	mu      sync.RWMutex
	entries map[string]*bleCacheEntry
}

var globalBLECache = &bleCache{
	entries: make(map[string]*bleCacheEntry),
}

func (c *bleCache) get(addr string) (*bleCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[addr]
	return entry, ok
}

func (c *bleCache) set(addr string, entry *bleCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[addr] = entry
}

type BLETransport struct{}

func (t *BLETransport) Name() string {
	return "BLE"
}

func (t *BLETransport) IsAvailable() bool {
	return enableAdapter() == nil
}

func (t *BLETransport) Dial(ctx context.Context, address string) (net.Conn, error) {
	address = util.NormalizeMAC(address)

	// If it's a MAC address on macOS, we need to resolve it via name matching
	// because CoreBluetooth on macOS only allows connecting via UUIDs.
	// On Linux/Windows, BLE addresses are standard MAC addresses, so we can dial directly.
	isUUID, _ := regexp.MatchString(`^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`, address)
	
	dialAddress := address
	if !isUUID {
		// Attempt to resolve MAC to BLE UUID if on macOS
		resolved, ok := resolveMACToBLEUUID(address)
		if ok {
			logger.Infof("BT/ble: resolved MAC %s to BLE UUID %s", address, resolved)
			dialAddress = resolved
		} else {
			// If not a UUID and we can't resolve it, check if we should even try BLE.
			// On Linux/Windows, a MAC address is perfectly valid for BLE, so we proceed.
			// On macOS, CoreBluetooth cannot dial a standard MAC address, so we fail early.
			if runtime.GOOS == "darwin" {
				return nil, fmt.Errorf("BT/ble: cannot dial MAC address %s directly on macOS without UUID resolution", address)
			}
		}
	}

	return dialBLE(ctx, dialAddress)
}

func (t *BLETransport) Scan(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	return scanLiveBLEPrinters(ctx, 3*time.Second)
}

// scanLiveBLEPrinters scans for BLE devices and returns a list of discovered printers.
func scanLiveBLEPrinters(ctx context.Context, timeout time.Duration) ([]BluetoothPrinterInfo, error) {
	logger.Debugf("BT/ble: starting live BLE scan for %v", timeout)

	if err := enableAdapter(); err != nil {
		return nil, fmt.Errorf("BT/ble: adapter not available for scan: %w", err)
	}

	var mu sync.Mutex
	var devices []BluetoothPrinterInfo
	seen := make(map[string]bool)

	scanDone := make(chan error, 1)
	go func() {
		err := adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
			name := result.LocalName()
			if name == "" {
				return
			}
			addrStr := result.Address.String()

			mu.Lock()
			defer mu.Unlock()
			if seen[addrStr] {
				return
			}
			seen[addrStr] = true

			nameLower := strings.ToLower(name)
			if strings.Contains(nameLower, "print") ||
				strings.Contains(nameLower, "pos") ||
				strings.Contains(nameLower, "mpt") ||
				strings.Contains(nameLower, "epson") ||
				strings.Contains(nameLower, "star") ||
				strings.Contains(nameLower, "thermal") ||
				strings.Contains(nameLower, "58") ||
				strings.Contains(nameLower, "80") ||
				strings.Contains(nameLower, "ble") ||
				strings.Contains(nameLower, "spp") {
				devices = append(devices, BluetoothPrinterInfo{
					MAC:  addrStr,
					Name: name,
				})
			}
		})
		scanDone <- err
	}()

	select {
	case <-ctx.Done():
		_ = adapter.StopScan()
		return nil, ctx.Err()
	case <-time.After(timeout):
		_ = adapter.StopScan()
	}

	select {
	case err := <-scanDone:
		if err != nil {
			logger.Errorf("BT/ble: scan failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		logger.Warnf("BT/ble: scan goroutine did not exit within backstop")
	}

	mu.Lock()
	defer mu.Unlock()
	return devices, nil
}

type bleConn struct {
	device       bluetooth.Device
	char         *bluetooth.DeviceCharacteristic
	address      string
	writeTimeout time.Duration
	readDeadline time.Time
	connected    bool
}

func (c *bleConn) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func (c *bleConn) Write(b []byte) (int, error) {
	if c.char == nil {
		return 0, fmt.Errorf("BT/ble: connection closed")
	}

	logger.Debugf("BT/ble: writing %d bytes to %s", len(b), c.address)

	// Split write into chunks because BLE has MTU limit.
	// Safe MTU chunk size is 180 bytes.
	chunkSize := 180
	totalWritten := 0

	for totalWritten < len(b) {
		end := totalWritten + chunkSize
		if end > len(b) {
			end = len(b)
		}
		chunk := b[totalWritten:end]

		n, err := c.char.Write(chunk)
		if err != nil {
			n, err = c.char.WriteWithoutResponse(chunk)
			if err != nil {
				return totalWritten, fmt.Errorf("BT/ble: write failed: %w", err)
			}
			time.Sleep(15 * time.Millisecond)
		}
		totalWritten += n
	}

	return totalWritten, nil
}

func (c *bleConn) Close() error {
	if c.connected {
		err := c.device.Disconnect()
		c.connected = false
		c.char = nil
		return err
	}
	return nil
}

func (c *bleConn) LocalAddr() net.Addr {
	return netAddrPlaceholder{net: "ble", addr: "local-ble"}
}

func (c *bleConn) RemoteAddr() net.Addr {
	return netAddrPlaceholder{net: "ble", addr: c.address}
}

func (c *bleConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *bleConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func (c *bleConn) SetWriteDeadline(t time.Time) error {
	if t.IsZero() {
		c.writeTimeout = 0
	} else {
		c.writeTimeout = time.Until(t)
	}
	return nil
}

func dialBLE(ctx context.Context, address string) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		conn, err := dialBLEInternal(address)
		ch <- result{conn, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.conn, r.err
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("BT/ble: connect to %s timed out after 15s", address)
	}
}

func dialBLEInternal(address string) (net.Conn, error) {
	logger.Infof("BT/ble: attempting to connect to BLE device %s", address)

	if err := enableAdapter(); err != nil {
		return nil, fmt.Errorf("BT/ble: adapter not available: %w", err)
	}

	var addr bluetooth.Address
	addr.Set(address)

	device, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("BT/ble: failed to connect to %s: %w", address, err)
	}

	// 1. Try to use cached characteristic info to bypass service discovery
	if _, ok := globalBLECache.get(address); ok {
		logger.Debugf("BT/ble: cache hit for %s characteristic resolution", address)
		// We still need to discover services or characteristics to bind the handle.
		// Wait, tinygo.org/x/bluetooth requires us to discover characteristic anyway,
		// but let's do it faster or proceed with standard discovery if cache fails.
	}

	services, err := device.DiscoverServices(nil)
	if err != nil {
		_ = device.Disconnect()
		return nil, fmt.Errorf("BT/ble: failed to discover services for %s: %w", address, err)
	}

	var fallbackChar *bluetooth.DeviceCharacteristic
	var targetChar *bluetooth.DeviceCharacteristic

	for _, srv := range services {
		srvUUID := srv.UUID().String()
		if strings.Contains(srvUUID, "1800") || strings.Contains(srvUUID, "1801") || strings.Contains(srvUUID, "180a") {
			continue
		}

		chars, err := srv.DiscoverCharacteristics(nil)
		if err != nil {
			continue
		}

		for _, char := range chars {
			charUUIDStr := strings.ToLower(char.UUID().String())
			isKnown := false
			for _, kw := range []string{"3802", "8841", "2af1", "1e4d", "bef15c90"} {
				if strings.Contains(charUUIDStr, kw) {
					isKnown = true
					break
				}
			}

			if isKnown {
				targetChar = &char
				break
			}

			if fallbackChar == nil {
				fallbackChar = &char
			}
		}
		if targetChar != nil {
			break
		}
	}

	if targetChar == nil {
		targetChar = fallbackChar
	}

	if targetChar == nil {
		_ = device.Disconnect()
		return nil, fmt.Errorf("BT/ble: no writeable characteristic found on device %s", address)
	}

	logger.Infof("BT/ble: connected successfully to %s, using characteristic %s", address, targetChar.UUID().String())
	
	// Cache the characteristic info
	globalBLECache.set(address, &bleCacheEntry{
		ServiceUUID:        targetChar.UUID().String(), // Wait, UUID() is what we need
		CharacteristicUUID: targetChar.UUID().String(),
	})

	return &bleConn{
		device:       device,
		char:         targetChar,
		address:      address,
		writeTimeout: 10 * time.Second,
		connected:    true,
	}, nil
}
