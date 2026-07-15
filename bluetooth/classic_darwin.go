//go:build darwin

package bluetooth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"epos-proxy/logger"
	"epos-proxy/util"

	"golang.org/x/sys/unix"
)

func preferredTransports() []Transport {
	return []Transport{
		&BLETransport{},
		&ClassicTransport{},
	}
}

// resolveMACToBLEUUID resolves a Classic MAC address to a BLE UUID on macOS by
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

type ClassicTransport struct{}

func (t *ClassicTransport) Name() string {
	return "Classic"
}

func (t *ClassicTransport) IsAvailable() bool {
	return true
}

func (t *ClassicTransport) Dial(ctx context.Context, address string) (net.Conn, error) {
	channel := BTManager.GetCachedRFCOMMChannel(address)
	return dialRFCOMMPlatform(address, channel)
}

func (t *ClassicTransport) Scan(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	return scanPairedPrinters()
}

// dialRFCOMMPlatform delegates to dialRFCOMM.
func dialRFCOMMPlatform(mac string, cachedChannel int) (net.Conn, error) {
	return dialRFCOMM(mac, cachedChannel)
}

// dialRFCOMM attempts to open a native IOBluetooth channel first (via rfcomm_darwin.go's Connect).
// If that is unavailable or fails, it falls back to the virtual serial port (/dev/cu.*).
func dialRFCOMM(mac string, channel int) (net.Conn, error) {
	mac = util.NormalizeMAC(mac)

	ch := uint8(1)
	if channel > 0 && channel <= 30 {
		ch = uint8(channel)
	}

	// Strategy 1: Try native IOBluetooth.framework (cgo) Connect
	logger.Infof("BT/darwin/classic: trying native IOBluetooth Connect for %s ch %d", mac, ch)
	conn, err := Connect(mac, ch)
	if err == nil {
		return conn, nil
	}
	logger.Warnf("BT/darwin/classic: native IOBluetooth Connect failed for %s: %v; falling back to virtual serial port", mac, err)

	// Strategy 2: Fall back to virtual serial port /dev/cu.*
	logger.Infof("BT/darwin/classic: opening virtual serial port for %s with 5s timeout", mac)
	serialConn, err := OpenDarwinBTSerial(mac, 5*time.Second, true)
	if err != nil {
		return nil, fmt.Errorf("BT/darwin/classic: both native IOBluetooth and virtual serial connection failed for %s: %w", mac, err)
	}

	return serialConn, nil
}

func scanPairedPrinters() ([]BluetoothPrinterInfo, error) {
	logger.Debug("BT: scanning for paired Bluetooth devices on macOS")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "system_profiler", "-json", "SPBluetoothDataType")
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("system_profiler timed out")
	}
	if err != nil {
		return nil, fmt.Errorf("system_profiler failed: %w", err)
	}

	type deviceDetail struct {
		DeviceAddress   string `json:"device_address"`
		DeviceMajorType string `json:"device_majorType"`
		DeviceMinorType string `json:"device_minorType"`
	}

	type btData struct {
		DeviceConnected    []map[string]deviceDetail `json:"device_connected"`
		DeviceNotConnected []map[string]deviceDetail `json:"device_not_connected"`
	}

	var btDataSlice []btData

	var rawObj struct {
		SPBluetoothDataType []btData `json:"SPBluetoothDataType"`
	}
	if errObj := json.Unmarshal(out, &rawObj); errObj == nil {
		btDataSlice = rawObj.SPBluetoothDataType
	} else {
		var rawArr []struct {
			SPBluetoothDataType []btData `json:"SPBluetoothDataType"`
		}
		if errArr := json.Unmarshal(out, &rawArr); errArr == nil {
			if len(rawArr) > 0 {
				btDataSlice = rawArr[0].SPBluetoothDataType
			}
		} else {
			return nil, fmt.Errorf("failed to parse system_profiler output: %w (tried object: %v, tried array: %v)", err, errObj, errArr)
		}
	}

	var devices []BluetoothPrinterInfo
	seen := make(map[string]bool)

	addDevice := func(name string, address string, majorType string, minorType string) {
		mac := util.NormalizeMAC(address)
		if err := util.ValidateMAC(mac); err != nil {
			logger.Warnf("BT/darwin: skipping device %q with invalid MAC %q", name, address)
			return
		}

		majorLower := strings.ToLower(majorType)
		minorLower := strings.ToLower(minorType)

		if !strings.Contains(majorLower, "imaging") &&
			!strings.Contains(minorLower, "printer") {
			return
		}

		seen[mac] = true
		devices = append(devices, BluetoothPrinterInfo{MAC: util.NormalizeMAC(mac), Name: name})
	}

	for _, btInfo := range btDataSlice {
		for _, devMap := range btInfo.DeviceConnected {
			for name, detail := range devMap {
				addDevice(name, detail.DeviceAddress, detail.DeviceMajorType, detail.DeviceMinorType)
			}
		}
		for _, devMap := range btInfo.DeviceNotConnected {
			for name, detail := range devMap {
				addDevice(name, detail.DeviceAddress, detail.DeviceMajorType, detail.DeviceMinorType)
			}
		}
	}

	logger.Debugf("BT/darwin: found %d paired Bluetooth printer(s)", len(devices))
	return devices, nil
}

func isBluetoothAdapterActive() bool {
	return true
}

func CheckDependencies() []DependencyStatus {
	return []DependencyStatus{}
}

var builtinCUPrefixes = []string{
	"cu.Bluetooth-",
	"cu.debug-console",
	"cu.wlan-debug",
	"cu.MALS",
}

func isBuiltinCU(name string) bool {
	for _, prefix := range builtinCUPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
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

func findDarwinTTY(mac string, allowFallback bool) (string, error) {
	matches, err := filepath.Glob("/dev/cu.*")
	if err != nil || len(matches) == 0 {
		logger.Warnf("BT/darwin: no /dev/cu.* devices found: %v", err)
		return "", fmt.Errorf("BT/darwin: no /dev/cu.* devices present")
	}

	logger.Debugf("BT/darwin: /dev/cu.* devices: %v", matches)

	var candidates []string
	for _, m := range matches {
		base := strings.TrimPrefix(m, "/dev/")
		if isBuiltinCU(base) {
			continue
		}
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("BT/darwin: no non-builtin /dev/cu.* candidates for %s", mac)
	}

	if btName := lookupBluetoothName(mac); btName != "" {
		sanitizedTarget := sanitizeForCUName(btName)
		for _, m := range candidates {
			base := strings.TrimPrefix(m, "/dev/")
			if sanitizedTarget != "" && strings.Contains(strings.ToLower(sanitizeForCUName(base)), sanitizedTarget) {
				logger.Infof("BT/darwin: Bluetooth-name-match device: %s (name %q)", m, btName)
				return m, nil
			}
		}
		logger.Debugf("BT/darwin: no cu device matched sanitized BT name %q", btName)
	}

	keywords := []string{"printer", "serial", "spp", "rfcomm", "port"}
	for _, m := range candidates {
		low := strings.ToLower(m)
		for _, kw := range keywords {
			if strings.Contains(low, kw) {
				logger.Infof("BT/darwin: keyword-match device: %s", m)
				return m, nil
			}
		}
	}

	if !allowFallback {
		logger.Warnf("BT/darwin: no confident match for %s and fallback disabled; refusing to guess", mac)
		return "", fmt.Errorf("BT/darwin: could not confidently resolve a /dev/cu.* device for %s", mac)
	}

	sort.Slice(candidates, func(i, j int) bool {
		fi, ei := os.Stat(candidates[i])
		fj, ej := os.Stat(candidates[j])
		if ei != nil || ej != nil {
			return false
		}
		return fi.ModTime().After(fj.ModTime())
	})
	logger.Warnf("BT/darwin: LOW-CONFIDENCE FALLBACK in use -- guessing most-recently-modified cu device %s for %s.", candidates[0], mac)
	return candidates[0], nil
}

type btSerialConn struct {
	f            *os.File
	path         string
	writeTimeout time.Duration
	VerifyFunc   func(f *os.File) error
}

func (c *btSerialConn) Read(b []byte) (int, error)        { return c.f.Read(b) }
func (c *btSerialConn) Close() error                      { return c.f.Close() }
func (c *btSerialConn) LocalAddr() net.Addr               { return netAddrPlaceholder{net: "bt-serial", addr: c.path} }
func (c *btSerialConn) RemoteAddr() net.Addr              { return netAddrPlaceholder{net: "bt-serial", addr: c.path} }
func (c *btSerialConn) SetDeadline(t time.Time) error     { return nil }
func (c *btSerialConn) SetReadDeadline(t time.Time) error { return nil }

func (c *btSerialConn) SetWriteDeadline(t time.Time) error {
	if t.IsZero() {
		c.writeTimeout = 0
		logger.Debugf("BT/darwin: write deadline set to zero (no timeout)")
	} else {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		c.writeTimeout = d
		logger.Debugf("BT/darwin: write deadline set to %v (timeout duration: %v)", t, d)
	}
	return nil
}

func (c *btSerialConn) Write(b []byte) (int, error) {
	logger.Debugf("BT/darwin: writing %d bytes to %s with timeout %v", len(b), c.path, c.writeTimeout)

	writeFunc := func() (int, error) {
		n, err := c.f.Write(b)
		if err != nil {
			return n, err
		}

		if drainErr := unix.IoctlSetInt(int(c.f.Fd()), unix.TIOCDRAIN, 0); drainErr != nil {
			return n, fmt.Errorf("BT/darwin: tcdrain failed: %w", drainErr)
		}

		if c.VerifyFunc != nil {
			if verr := c.VerifyFunc(c.f); verr != nil {
				return n, fmt.Errorf("BT/darwin: write drained locally but verification failed: %w", verr)
			}
		}

		return n, nil
	}

	if c.writeTimeout <= 0 {
		return writeFunc()
	}

	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		n, err := writeFunc()
		ch <- result{n, err}
	}()

	select {
	case r := <-ch:
		return r.n, r.err
	case <-time.After(c.writeTimeout):
		_ = c.f.Close()
		return 0, fmt.Errorf("BT/darwin: write timeout after %v on %s", c.writeTimeout, c.path)
	}
}

func setRaw(fd uintptr) error {
	termios, err := unix.IoctlGetTermios(int(fd), unix.TIOCGETA)
	if err != nil {
		return fmt.Errorf("BT/darwin: IoctlGetTermios failed: %w", err)
	}

	termios.Ispeed = 115200
	termios.Ospeed = 115200
	termios.Cflag |= unix.CLOCAL | unix.CREAD
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Iflag &^= unix.IXON | unix.IXOFF | unix.IXANY
	termios.Cflag &^= unix.CRTSCTS

	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN

	if err := unix.IoctlSetTermios(int(fd), unix.TIOCSETA, termios); err != nil {
		return fmt.Errorf("BT/darwin: IoctlSetTermios failed: %w", err)
	}
	return nil
}

func OpenDarwinBTSerial(mac string, writeTimeout time.Duration, allowFallback bool) (*btSerialConn, error) {
	path, err := findDarwinTTY(mac, allowFallback)
	if err != nil {
		return nil, fmt.Errorf("BT/darwin: device resolution failed for %s: %w", mac, err)
	}

	f, err := os.OpenFile(path, unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("BT/darwin: failed to open %s: %w", path, err)
	}

	if err := setRaw(f.Fd()); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("BT/darwin: failed to set raw mode on %s: %w", path, err)
	}
	logger.Infof("BT/darwin: opened %s for MAC %s and configured raw mode", path, mac)

	conn := &btSerialConn{
		f:            f,
		path:         path,
		writeTimeout: writeTimeout,
	}
	return conn, nil
}
