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

// ScanBluetoothPrinters queries system_profiler for paired Bluetooth devices and filters for printers.
func ScanBluetoothPrinters() ([]BluetoothPrinterInfo, error) {
	logger.Debug("BT: scanning for Bluetooth devices on macOS")

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

	// 1. Try parsing as JSON object: {"SPBluetoothDataType": [...]}
	var rawObj struct {
		SPBluetoothDataType []btData `json:"SPBluetoothDataType"`
	}
	if errObj := json.Unmarshal(out, &rawObj); errObj == nil {
		btDataSlice = rawObj.SPBluetoothDataType
	} else {
		// 2. Try parsing as JSON array: [{"SPBluetoothDataType": [...]}]
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

	logger.Debugf("BT/darwin: found %d Bluetooth printer(s)", len(devices))
	return devices, nil
}

func IsBluetoothAdapterActive() bool {
	// macOS:
	// We intentionally skip checking the Bluetooth adapter state.
	//
	// - `defaults read ... ControllerPowerState` is unreliable on recent
	//   macOS versions and may report Bluetooth as off even when it is enabled.
	// - `system_profiler SPBluetoothDataType` provides accurate information
	//   but is too slow for routine checks.
	//
	// Instead, we attempt to open the printer's serial device directly.
	// If Bluetooth is disabled, the printer is unavailable, or the SPP
	// device does not exist, the connection attempt will fail with an
	// appropriate error.
	return true
}

func CheckDependencies() []DependencyStatus {
	return []DependencyStatus{}
}

// dialRFCOMM opens a connection to a Bluetooth RFCOMM printer on macOS.
func dialRFCOMM(mac string, _ int) (net.Conn, error) {
	mac = util.NormalizeMAC(mac)

	tty := findDarwinTTY(mac)
	if tty == "" {
		return nil, fmt.Errorf(
			"no Bluetooth serial port found in /dev/cu.* for %s — "+
				"pair the printer in System Preferences → Bluetooth and ensure "+
				"the Serial Port Profile (SPP) is enabled", mac)
	}

	logger.Infof("BT/darwin: opening %s for %s", tty, mac)

	f, err := os.OpenFile(tty, os.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf(
				"BT/darwin: cannot open %s — permission denied (try: sudo chmod a+rw %s): %w",
				tty, tty, err)
		}
		return nil, fmt.Errorf("BT/darwin: cannot open %s: %w", tty, err)
	}

	if err := unix.SetNonblock(int(f.Fd()), false); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("BT/darwin: failed to clear non-blocking mode: %w", err)
	}

	if err := setRaw(f.Fd()); err != nil {
		logger.Warnf("BT/darwin: failed to set raw mode on %s (might corrupt ESC/POS data): %v", tty, err)
	}

	return &btSerialConn{f: f, path: tty, writeTimeout: btConnectTimeout}, nil
}

// ---------------------------------------------------------------------------
// dialRFCOMMPlatform — macOS entry point
// ---------------------------------------------------------------------------

// dialRFCOMMPlatform on macOS opens the paired Bluetooth serial (/dev/cu.*)
// device.  SDP and channel probing are not applicable on Darwin.
func dialRFCOMMPlatform(mac string, cachedChannel int) (net.Conn, error) {
	mac = util.NormalizeMAC(mac)
	logger.Infof("BT/darwin: dialling %s (channel %d ignored)", mac, cachedChannel)
	return dialRFCOMM(mac, cachedChannel)
}

// --------------------- TODO -----------------------

// ---------------------------------------------------------------------------
// macOS RFCOMM via virtual serial port
// ---------------------------------------------------------------------------
//
// macOS does NOT expose AF_BLUETOOTH / BTPROTO_RFCOMM through the BSD socket
// API.  Paired RFCOMM (SPP) printers instead appear as /dev/cu.* character
// devices (e.g. /dev/cu.PrinterName-SerialPort).  We open that device
// directly, wrapping it in our btSerialConn type.
//
// Use /dev/cu.* (not /dev/tty.*): cu devices are "call-up" and are the
// correct ones for initiating outgoing connections on macOS.

// builtinCUPrefixes are macOS system serial devices that are never BT printers.
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

// findDarwinTTY returns the best /dev/cu.* device for a Bluetooth printer.
// Strategy (in order):
//  1. Exact match: device whose name contains a MAC-derived fragment
//  2. Keyword match: device containing printer/serial/spp keywords
//  3. Fallback: the most-recently-modified non-builtin /dev/cu.* device
func findDarwinTTY(mac string) string {
	matches, err := filepath.Glob("/dev/cu.*")
	if err != nil || len(matches) == 0 {
		logger.Warnf("BT/darwin: no /dev/cu.* devices found: %v", err)
		return ""
	}

	// Log all candidates to aid debugging
	logger.Debugf("BT/darwin: /dev/cu.* devices: %v", matches)

	// Derive a MAC fragment macOS might embed in the device name
	// macOS uses dashes: AA-BB-CC-DD-EE-FF
	macDash := strings.ReplaceAll(mac, ":", "-")
	macNodash := strings.ReplaceAll(mac, ":", "")

	var candidates []string
	for _, m := range matches {
		base := strings.TrimPrefix(m, "/dev/")
		if isBuiltinCU(base) {
			continue
		}
		low := strings.ToLower(base)

		// Priority 1: MAC match
		if strings.Contains(low, strings.ToLower(macDash)) ||
			strings.Contains(low, strings.ToLower(macNodash)) {
			logger.Infof("BT/darwin: MAC-match device: %s", m)
			return m
		}

		candidates = append(candidates, m)
	}

	// Priority 2: keyword match among candidates
	keywords := []string{"printer", "serial", "spp", "rfcomm", "port"}
	for _, m := range candidates {
		low := strings.ToLower(m)
		for _, kw := range keywords {
			if strings.Contains(low, kw) {
				logger.Infof("BT/darwin: keyword-match device: %s", m)
				return m
			}
		}
	}

	// Priority 3: most recently modified non-builtin cu device
	if len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool {
			fi, ei := os.Stat(candidates[i])
			fj, ej := os.Stat(candidates[j])
			if ei != nil || ej != nil {
				return false
			}
			return fi.ModTime().After(fj.ModTime())
		})
		logger.Infof("BT/darwin: fallback device (most-recently modified): %s", candidates[0])
		return candidates[0]
	}

	logger.Warnf("BT/darwin: no suitable /dev/cu.* device found for %s", mac)
	return ""
}

// ---------------------------------------------------------------------------
// btSerialConn — os.File wrapped as net.Conn with real write deadline support
// ---------------------------------------------------------------------------
//
// os.File does not implement SetDeadline on macOS character devices, so we
// wrap it and enforce write timeouts ourselves using a goroutine.

type btSerialConn struct {
	f            *os.File
	path         string
	writeTimeout time.Duration
}

func (c *btSerialConn) Read(b []byte) (int, error)        { return c.f.Read(b) }
func (c *btSerialConn) Close() error                      { return c.f.Close() }
func (c *btSerialConn) LocalAddr() net.Addr               { return serialAddr{c.path} }
func (c *btSerialConn) RemoteAddr() net.Addr              { return serialAddr{c.path} }
func (c *btSerialConn) SetDeadline(t time.Time) error     { return nil }
func (c *btSerialConn) SetReadDeadline(t time.Time) error { return nil }

// SetWriteDeadline stores the deadline; actual enforcement happens in Write.
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

// Write sends data to the serial device, respecting the stored write timeout.
func (c *btSerialConn) Write(b []byte) (int, error) {
	logger.Debugf("BT/darwin: writing %d bytes to %s with timeout %v", len(b), c.path, c.writeTimeout)

	writeFunc := func() (int, error) {
		logger.Debugf("BT/darwin: calling physical file.Write for %s (%d bytes)", c.path, len(b))
		n, err := c.f.Write(b)
		if err != nil {
			logger.Errorf("BT/darwin: physical file.Write failed for %s: %v", c.path, err)
			return n, err
		}
		logger.Debugf("BT/darwin: physical file.Write succeeded for %s, wrote %d bytes. Calling tcdrain ioctl", c.path, n)

		if drainErr := unix.IoctlSetInt(int(c.f.Fd()), unix.TIOCDRAIN, 0); drainErr != nil {
			logger.Errorf("BT/darwin: tcdrain ioctl failed for %s: %v", c.path, drainErr)
			return n, fmt.Errorf("BT/darwin: tcdrain failed: %w", drainErr)
		}
		logger.Debugf("BT/darwin: tcdrain ioctl succeeded for %s (bytes fully transmitted)", c.path)
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
		if r.err != nil {
			logger.Errorf("BT/darwin: Write result for %s: %d bytes, error: %v", c.path, r.n, r.err)
		} else {
			logger.Debugf("BT/darwin: Write result for %s: %d bytes, success", c.path, r.n)
		}
		return r.n, r.err
	case <-time.After(c.writeTimeout):
		logger.Warnf("BT/darwin: Write timed out after %v for %s. Closing file descriptor", c.writeTimeout, c.path)
		_ = c.f.Close() // unblock the Tcdrain/Write goroutine
		return 0, fmt.Errorf("BT/darwin: write timeout after %v on %s", c.writeTimeout, c.path)
	}
}

// ---------------------------------------------------------------------------
// dialRFCOMM
// ---------------------------------------------------------------------------

// setRaw configures the TTY/CU serial device to raw mode.
// This prevents the OS from translating newlines (e.g. mapping LF to CR-LF via ONLCR)
// or intercepting control characters, preserving the binary ESC/POS print payload.
func setRaw(fd uintptr) error {
	termios, err := unix.IoctlGetTermios(int(fd), unix.TIOCGETA)
	if err != nil {
		return err
	}

	// Set input and output speed to 115200 (standard high-speed serial baud rate)
	termios.Ispeed = 115200
	termios.Ospeed = 115200

	// Enable receiver and ignore modem control lines
	termios.Cflag |= unix.CLOCAL | unix.CREAD

	// Disable parity, set 8 data bits
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8

	// Disable software flow control (XON/XOFF)
	termios.Iflag &^= unix.IXON | unix.IXOFF | unix.IXANY

	// Disable hardware flow control (CRTSCTS)
	termios.Cflag &^= unix.CRTSCTS

	// Make TTY raw
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN

	return unix.IoctlSetTermios(int(fd), unix.TIOCSETA, termios)
}
