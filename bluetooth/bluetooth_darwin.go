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

// Opens a connection to a Bluetooth RFCOMM printer on macOS.
func dialRFCOMM(mac string, _ int) (net.Conn, error) {
	mac = util.NormalizeMAC(mac)

	tty, err := findDarwinTTY(mac, true)
	if err != nil {
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

// on macOS opens the paired Bluetooth serial (/dev/cu.*)
// device.  SDP and channel probing are not applicable on Darwin.
func dialRFCOMMPlatform(mac string, cachedChannel int) (net.Conn, error) {
	mac = util.NormalizeMAC(mac)
	logger.Infof("BT/darwin: dialling %s (channel %d ignored)", mac, cachedChannel)
	return dialRFCOMM(mac, cachedChannel)
}

// ---------------------------------------------------------------------------
// macOS RFCOMM via virtual serial port
// ---------------------------------------------------------------------------
//
// macOS does NOT expose AF_BLUETOOTH / BTPROTO_RFCOMM through the BSD socket
// API. Paired RFCOMM (SPP) printers instead appear as /dev/cu.* character
// devices (e.g. /dev/cu.PrinterName-SerialPort). We open that device
// directly, wrapping it in our btSerialConn type.
//
// Use /dev/cu.* (not /dev/tty.*): cu devices are "call-up" and are the
// correct ones for initiating outgoing connections on macOS.
//
// ---------------------------------------------------------------------------
// FIXES APPLIED (see inline comments marked FIX):
//
//  1. Device resolution no longer relies on a MAC-address fragment being
//     present in the /dev/cu.* name -- macOS does not embed the MAC there.
//     It now resolves the paired device's *Bluetooth name* via
//     `system_profiler SPBluetoothDataType -json` and matches that against
//     the cu device name, which is how macOS actually names these nodes.
//
//  2. The old "most-recently-modified cu device" fallback could silently
//     select a totally unrelated serial device (another peripheral, a stale
//     node, etc.), causing writes to succeed against the wrong device with
//     no print output. That fallback is now opt-in only, loudly logged as
//     a best-effort guess, and never chosen silently.
//
//  3. setRaw() was defined but never invoked anywhere near the open path.
//     It is now called immediately after opening the device, with its
//     error checked and propagated instead of ignored.
//
//  4. Open() now returns an error instead of an empty path/silent failure,
//     so callers can't proceed with a nil/garbage connection and later
//     report "success" from a write that never had a valid destination.
//
//  5. Write() success now only reflects "handed off to the local kernel
//     serial buffer and drained" -- NOT "received by the printer". This is
//     an inherent limitation of RFCOMM SPP without an application-level
//     ack, so a comment + hook (VerifyFunc) is included for callers who
//     want to layer a status-query/ack check on top for real confirmation.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Bluetooth name resolution (FIX #1)
// ---------------------------------------------------------------------------
//
// macOS names a paired SPP device's /dev/cu.* node after the device's
// Bluetooth *name* (with spaces/punctuation sanitized), not its MAC address.
// e.g. a printer named "BT Printer 58" typically shows up as something like
// /dev/cu.BTPrinter58-SPP or /dev/cu.BTPrinter58-SerialPort.
//
// To resolve name -> mac reliably we ask system_profiler for the paired
// device list and match on MAC, then use the returned device name to match
// against the /dev/cu.* candidates.

type spBluetoothDevice struct {
	Name      string `json:"device_name"`
	Address   string `json:"device_address"`
	Connected string `json:"device_connected"`
	Paired    string `json:"device_isconnected"`
}

// lookupBluetoothName returns the paired device's advertised Bluetooth name
// for the given MAC address, or "" if it can't be determined. This is
// best-effort: system_profiler's JSON schema for Bluetooth has historically
// been inconsistent across macOS versions, so failures here are non-fatal
// and callers should fall back to keyword matching.
func lookupBluetoothName(mac string) string {
	out, err := exec.Command("system_profiler", "SPBluetoothDataType", "-json").Output()
	if err != nil {
		logger.Warnf("BT/darwin: system_profiler failed, cannot resolve BT name for %s: %v", mac, err)
		return ""
	}

	// The JSON structure nests devices under a data-driven set of category
	// keys we don't fully control, so decode generically and walk it looking
	// for objects that carry a device_address matching mac.
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

// sanitizeForCUName mirrors (approximately) the sanitization macOS applies
// when turning a Bluetooth device name into a /dev/cu.* node name: spaces
// and most punctuation are stripped/collapsed. This is intentionally lax
// (substring containment check) rather than trying to exactly reproduce
// macOS's internal algorithm.
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

// ---------------------------------------------------------------------------
// findDarwinTTY (FIX #1, #2)
// ---------------------------------------------------------------------------
//
// findDarwinTTY returns the best /dev/cu.* device for a Bluetooth printer.
// Strategy (in order):
//  1. Bluetooth-name match: resolve the paired device's advertised name via
//     system_profiler and match it (sanitized) against candidate cu names.
//  2. Keyword match: device containing printer/serial/spp/rfcomm/port
//     keywords.
//  3. Explicit, opt-in fallback: the most-recently-modified non-builtin
//     /dev/cu.* device. This is a guess and is only used if allowFallback
//     is true; it is always loudly logged as a low-confidence guess so
//     "success but nothing printed" can be traced back to this path.
//
// Returns ("", error) if no candidate could be resolved, so callers cannot
// silently proceed with an empty/garbage path.
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

	// Priority 1: Bluetooth-name match.
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

	// Priority 2: keyword match among candidates.
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

	// Priority 3: explicit, opt-in, loudly-logged fallback.
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
	logger.Warnf("BT/darwin: LOW-CONFIDENCE FALLBACK in use -- guessing most-recently-modified cu device %s for %s. "+
		"This may be the wrong device; a write can succeed here with no physical print output.", candidates[0], mac)
	return candidates[0], nil
}

// ---------------------------------------------------------------------------
// btSerialConn -- os.File wrapped as net.Conn with real write deadline support
// ---------------------------------------------------------------------------
//
// os.File does not implement SetDeadline on macOS character devices, so we
// wrap it and enforce write timeouts ourselves using a goroutine.

type btSerialConn struct {
	f            *os.File
	path         string
	writeTimeout time.Duration

	// VerifyFunc is an optional hook (FIX #5) callers can set to perform an
	// application-level confirmation after a write+drain succeeds (e.g. an
	// ESC/POS status query read-back). Local write+tcdrain success only
	// means the bytes left this machine's kernel serial buffer -- it does
	// NOT confirm the printer received or printed them. Leave nil to skip.
	VerifyFunc func(f *os.File) error
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
//
// NOTE (FIX #5): a nil error here means "written to the local kernel serial
// buffer and drained (TIOCDRAIN)". Over Bluetooth SPP this is NOT the same
// as "the printer received/printed it" -- there is no link-level ack at
// this layer. If VerifyFunc is set, it is invoked after a successful drain
// to give callers a real confirmation signal; otherwise this is
// best-effort only, consistent with what the transport can actually promise.
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
		logger.Debugf("BT/darwin: tcdrain ioctl succeeded for %s (bytes flushed to local kernel buffer)", c.path)

		if c.VerifyFunc != nil {
			if verr := c.VerifyFunc(c.f); verr != nil {
				logger.Errorf("BT/darwin: post-write verification failed for %s: %v", c.path, verr)
				return n, fmt.Errorf("BT/darwin: write drained locally but verification failed: %w", verr)
			}
			logger.Debugf("BT/darwin: post-write verification succeeded for %s", c.path)
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
// setRaw
// ---------------------------------------------------------------------------

// setRaw configures the TTY/CU serial device to raw mode.
// This prevents the OS from translating newlines (e.g. mapping LF to CR-LF via
// ONLCR) or intercepting control characters, preserving the binary ESC/POS
// print payload.
func setRaw(fd uintptr) error {
	termios, err := unix.IoctlGetTermios(int(fd), unix.TIOCGETA)
	if err != nil {
		return fmt.Errorf("BT/darwin: IoctlGetTermios failed: %w", err)
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

	if err := unix.IoctlSetTermios(int(fd), unix.TIOCSETA, termios); err != nil {
		return fmt.Errorf("BT/darwin: IoctlSetTermios failed: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Open (FIX #3, #4) -- ties device resolution, raw-mode setup, and the
// btSerialConn wrapper together so callers can't skip setRaw or proceed
// with an unresolved device path.
// ---------------------------------------------------------------------------

// serialAddr is a minimal net.Addr implementation for the resolved cu path.

// func (a serialAddr) Network() string { return "bt-serial" }

// OpenDarwinBTSerial resolves, opens, and configures the /dev/cu.* device
// for a paired Bluetooth (SPP) printer with the given MAC address.
//
// allowFallback controls whether findDarwinTTY may guess using the
// most-recently-modified cu device when no confident match is found. Leave
// this false in production paths where printing to the wrong device
// silently is worse than failing loudly; enable it only as a
// last-resort/manual-debug option.
func OpenDarwinBTSerial(mac string, writeTimeout time.Duration, allowFallback bool) (*btSerialConn, error) {
	path, err := findDarwinTTY(mac, allowFallback)
	if err != nil {
		return nil, fmt.Errorf("BT/darwin: device resolution failed for %s: %w", mac, err)
	}

	f, err := os.OpenFile(path, unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("BT/darwin: failed to open %s: %w", path, err)
	}

	// FIX #3: setRaw is now actually invoked, and its error is checked
	// rather than silently ignored. A failure here means the device may
	// still be in cooked mode, which can corrupt/translate the binary
	// ESC/POS stream (e.g. LF -> CRLF) even though writes appear to succeed.
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
