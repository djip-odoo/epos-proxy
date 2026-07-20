//go:build !darwin || cgo

package bluetooth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
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
var uuidRegexp = regexp.MustCompile(`^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`)

func enableAdapter() error {
	adapterOnce.Do(func() {
		adapterEnableErr = adapter.Enable()
		if adapterEnableErr != nil {
			logger.Errorf("BT/ble: failed to enable bluetooth adapter: %v", adapterEnableErr)
		} else {
			logger.Debugf("BT/ble: bluetooth adapter enabled successfully")
		}
	})
	return adapterEnableErr
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
	// macOS CoreBluetooth identifies BLE peripherals by UUID rather than exposing
	// their Bluetooth MAC address. Linux and Windows expose a Bluetooth address for
	// BLE devices, but connections are still established through BLE peripheral
	// discovery rather than directly dialing an address.

	dialAddress := address
	if !uuidRegexp.MatchString(address) && runtime.GOOS == "darwin" {
		// Attempt to resolve MAC to BLE UUID if on macOS
		resolved, ok := resolveMACToBLEUUID(address)
		if ok {
			logger.Infof("BT/ble: resolved MAC %s to BLE UUID %s", address, resolved)
			dialAddress = resolved
		} else {
			// If not a UUID and we can't resolve it, check if we should even try BLE.
			// On Linux/Windows, a MAC address is perfectly valid for BLE, so we proceed.
			// On macOS, CoreBluetooth cannot dial a standard MAC address, so we fail early.
			return nil, fmt.Errorf("BT/ble: cannot dial MAC address %s directly on macOS without UUID resolution", address)
		}
	}

	return dialBLE(ctx, dialAddress)
}

func (t *BLETransport) Scan(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	return scanLiveBLEPrinters(ctx, 3*time.Second)
}

// Scans for BLE devices and returns a list of discovered printers.
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
			if name == "" {
				return
			}

			devices = append(devices, BluetoothPrinterInfo{
				MAC:    addrStr,
				Name:   name,
				Device: getDeviceType(name),
			})
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

func dialBLEInternal(address string) (conn net.Conn, err error) {
	logger.Infof("BT/ble: connecting to %s", address)

	if err = enableAdapter(); err != nil {
		return nil, fmt.Errorf("BT/ble: adapter not available: %w", err)
	}

	var addr bluetooth.Address
	addr.Set(address)

	device, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("BT/ble: failed to connect to %s: %w", address, err)
	}

	success := false
	defer func() {
		if !success {
			_ = device.Disconnect()
		}
	}()

	services, err := device.DiscoverServices(nil)
	if err != nil {
		return nil, fmt.Errorf("BT/ble: failed to discover services: %w", err)
	}

	var char *bluetooth.DeviceCharacteristic

	for _, service := range services {
		char, err = discoverPrinterCharacteristic(service)
		if err != nil {
			logger.Debugf("BT/ble: skipping service %s: %v", service.UUID(), err)
			continue
		}
		if char != nil {
			break
		}
	}

	if char == nil {
		return nil, fmt.Errorf("BT/ble: no writable characteristic found")
	}

	logger.Infof(
		"BT/ble: connected to %s using characteristic %s",
		address,
		char.UUID(),
	)

	success = true

	return &bleConn{
		device:       device,
		char:         char,
		address:      address,
		writeTimeout: 10 * time.Second,
		connected:    true,
	}, nil
}

func discoverPrinterCharacteristic(service bluetooth.DeviceService) (*bluetooth.DeviceCharacteristic, error) {
	chars, err := service.DiscoverCharacteristics(nil)
	if err != nil {
		return nil, err
	}

	var fallback *bluetooth.DeviceCharacteristic

	for _, c := range chars {
		logger.Debugf("BT/ble: characteristic %s", c.UUID())

		if fallback == nil {
			fallback = &c
		}

		if isKnownPrinterCharacteristic(c.UUID().String()) {
			return &c, nil
		}
	}

	return fallback, nil
}

func isKnownPrinterCharacteristic(uuid string) bool {
	uuid = strings.ToLower(uuid)

	switch {
	case strings.Contains(uuid, "3802"):
		return true
	case strings.Contains(uuid, "8841"):
		return true
	case strings.Contains(uuid, "2af1"):
		return true
	case strings.Contains(uuid, "1e4d"):
		return true
	case strings.Contains(uuid, "bef15c90"):
		return true
	default:
		return false
	}
}

// Resolves a Classic MAC address to a BLE UUID on macOS by
// querying system_profiler for the device's Bluetooth name, then scanning for a
// BLE device with a matching or similar name.
func resolveMACToBLEUUID(mac string) (string, bool) {
	btName := lookupBluetoothName(mac)
	if btName == "" {
		return "", false
	}
	sanitizedTarget := sanitizeForCUName(btName)
	if sanitizedTarget == "" {
		return "", false
	}

	logger.Infof("BT/darwin/classic: attempting to resolve MAC %s (%q) via BLE scan name-matching", mac, btName)

	ble := &BLETransport{}
	if !ble.IsAvailable() {
		return "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	devices, err := ble.Scan(ctx)
	if err != nil {
		return "", false
	}

	for _, dev := range devices {
		if strings.Contains(strings.ToLower(sanitizeForCUName(dev.Name)), sanitizedTarget) {
			logger.Infof("BT/darwin/classic: matched BLE device %s (%q) for classic printer %s", dev.MAC, dev.Name, mac)
			return dev.MAC, true
		}
	}

	return "", false
}

func lookupBluetoothName(mac string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "system_profiler", "SPBluetoothDataType", "-json").Output()
	if ctx.Err() == context.DeadlineExceeded {
		logger.Warnf("BT/darwin: system_profiler timed out resolving BT name for %s", mac)
		return ""
	}
	if err != nil {
		logger.Warnf("BT/darwin: system_profiler failed, cannot resolve BT name for %s: %v", mac, err)
		return ""
	}

	var generic map[string]any
	if err := json.Unmarshal(out, &generic); err != nil {
		logger.Warnf("BT/darwin: failed to parse system_profiler JSON: %v", err)
		return ""
	}

	target := strings.ToLower(mac)
	var found string
	var walk func(v any)
	walk = func(v any) {
		if found != "" {
			return
		}
		switch t := v.(type) {
		case map[string]any:
			if addr, ok := t["device_address"].(string); ok && strings.ToLower(addr) == target {
				if name, ok := t["_name"].(string); ok {
					found = name
					return
				}
			}
			for _, val := range t {
				walk(val)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(generic)

	if found == "" {
		logger.Debugf("BT/darwin: no system_profiler entry matched MAC %s", mac)
	} else {
		logger.Infof("BT/darwin: resolved MAC %s -> Bluetooth name %q", mac, found)
	}
	return found
}

func sanitizeForCUName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

func getDeviceType(name string) string {
	nameLower := strings.ToLower(name)
	if strings.Contains(nameLower, "print") ||
		strings.Contains(nameLower, "pos") ||
		strings.Contains(nameLower, "epson") ||
		strings.Contains(nameLower, "star") ||
		strings.Contains(nameLower, "thermal") ||
		strings.Contains(nameLower, "58") ||
		strings.Contains(nameLower, "80") {
		return "printer"
	}
	return "other"
}
