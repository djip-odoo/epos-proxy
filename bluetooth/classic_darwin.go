//go:build darwin

package bluetooth

import (
	"context"
	"encoding/json"
	"epos-proxy/logger"
	"epos-proxy/util"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"

	"golang.org/x/sys/unix"
)

type ClassicTransport struct{}

func (t *ClassicTransport) Name() string {
	return "Classic"
}

func (t *ClassicTransport) IsAvailable() bool {
	return isBluetoothAdapterActive()
}

func (t *ClassicTransport) Dial(ctx context.Context, address string) (net.Conn, error) {
	return dialRFCOMMPlatform(address, 0)
}

func (t *ClassicTransport) Scan(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	return scanPairedPrinters()
}

func CheckDependencies() []DependencyStatus {
	return []DependencyStatus{}
}

func isBluetoothAdapterActive() bool {
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

// scanPairedPrinters queries system_profiler for paired Bluetooth devices and filters for printers.
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
		if err := util.ValidateAddress(address); err != nil {
			logger.Warnf("BT/darwin: skipping device %q with invalid MAC %q", name, address)
			return
		}

		majorLower := strings.ToLower(majorType)
		minorLower := strings.ToLower(minorType)

		if !strings.Contains(majorLower, "imaging") &&
			!strings.Contains(minorLower, "printer") {
			return
		}

		seen[address] = true
		devices = append(devices, BluetoothPrinterInfo{Address: util.NormalizeAddress(address), Name: name})
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

// dialRFCOMM opens a Bluetooth Classic RFCOMM channel to mac using
// IOBluetooth.framework (no /dev/cu.* dependency).
//
// channel selects the RFCOMM channel number (1–30).  Most SPP thermal printers
// advertise channel 1; the caller should pass the SDP-discovered channel when
// available, or 0 to accept the default of 1.
func dialRFCOMM(mac string, channel int) (net.Conn, error) {
	mac = util.NormalizeAddress(mac)

	ch := uint8(1) // SPP default
	if channel > 0 && channel <= 30 {
		ch = uint8(channel)
	}

	logger.Infof("BT/darwin/rfcomm: dialling IOBluetooth RFCOMM for %s ch %d", mac, ch)
	return Connect(mac, ch)
}

// dialRFCOMMPlatform is the macOS entry point for Bluetooth Classic printing.
// BLE UUIDs are routed to dialBLE; classic MAC addresses go through the
// IOBluetooth RFCOMM path in dialRFCOMM.
func dialRFCOMMPlatform(mac string, cachedChannel int) (net.Conn, error) {
	mac = util.NormalizeAddress(mac)
	if matched, _ := regexp.MatchString(`^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`, mac); matched {
		return dialBLE(mac)
	}
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
		// FIX #5: closing the fd should unblock the write/TIOCDRAIN syscall in the
		// goroutine above. If the kernel keeps the syscall stuck (possible on some
		// macOS versions with TTY character devices), the goroutine will leak until
		// the syscall eventually completes and sends on the buffered channel.
		logger.Warnf("BT/darwin: write timed out after %v for %s; closing fd to unblock write goroutine "+
			"(goroutine may leak temporarily if kernel write/TIOCDRAIN syscall is stuck)", c.writeTimeout, c.path)
		_ = c.f.Close()
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

// ---------------------------------------------------------------------------
// BLE Printer Connection and Conn Implementation
// ---------------------------------------------------------------------------

type bleConn struct {
	device       bluetooth.Device
	char         *bluetooth.DeviceCharacteristic
	uuid         string
	writeTimeout time.Duration
	readDeadline time.Time
	connected    bool
}

func (c *bleConn) Read(b []byte) (int, error) {
	// FIX #1: BLE printing is write-only. Returning 0, nil in a loop would cause
	// any caller that loops on Read (io.Copy, net/http, etc.) to spin forever.
	// Return io.EOF immediately to signal that no data will ever arrive.
	return 0, io.EOF
}

func (c *bleConn) Write(b []byte) (int, error) {
	if c.char == nil {
		return 0, fmt.Errorf("BT/darwin/ble: connection closed")
	}

	logger.Debugf("BT/darwin/ble: writing %d bytes to %s", len(b), c.uuid)

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

		// Try writing with response, fallback to without response if supported or fails
		n, err := c.char.Write(chunk)
		if err != nil {
			n, err = c.char.WriteWithoutResponse(chunk)
			if err != nil {
				return totalWritten, fmt.Errorf("BT/darwin/ble: write failed: %w", err)
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
	return bleAddr{addr: "local-ble"}
}

func (c *bleConn) RemoteAddr() net.Addr {
	return bleAddr{addr: c.uuid}
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

type bleAddr struct {
	addr string
}

func (a bleAddr) Network() string { return "ble" }
func (a bleAddr) String() string  { return a.addr }

// dialBLE connects to a BLE device and returns a net.Conn.
// FIX #6: adapter.Connect and device.DiscoverServices have no built-in timeout
// on macOS/CoreBluetooth and can hang indefinitely if the device is out of
// range or the BLE stack is initialising. We wrap the entire operation in a
// goroutine and enforce a 15-second hard deadline.
func dialBLE(uuid string) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		conn, err := dialBLEInternal(uuid)
		ch <- result{conn, err}
	}()
	select {
	case r := <-ch:
		return r.conn, r.err
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("BT/darwin/ble: connect to %s timed out after 15s (device out of range or BLE stack not ready)", uuid)
	}
}

// dialBLEInternal performs the actual BLE connection; called from dialBLE's goroutine.
func dialBLEInternal(uuid string) (net.Conn, error) {
	logger.Infof("BT/darwin/ble: attempting to connect to BLE device %s", uuid)

	// FIX #4: use sync.Once-guarded enableAdapter().
	if err := enableAdapter(); err != nil {
		return nil, fmt.Errorf("BT/darwin/ble: adapter not available: %w", err)
	}

	var addr bluetooth.Address
	addr.Set(uuid)

	device, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("BT/darwin/ble: failed to connect to %s: %w", uuid, err)
	}

	services, err := device.DiscoverServices(nil)
	if err != nil {
		_ = device.Disconnect()
		return nil, fmt.Errorf("BT/darwin/ble: failed to discover services for %s: %w", uuid, err)
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
		return nil, fmt.Errorf("BT/darwin/ble: no writeable characteristic found on device %s", uuid)
	}

	logger.Infof("BT/darwin/ble: connected successfully to %s, using characteristic %s", uuid, targetChar.UUID().String())
	return &bleConn{
		device:       device,
		char:         targetChar,
		uuid:         uuid,
		writeTimeout: 10 * time.Second,
		connected:    true,
	}, nil
}
