//go:build linux

package printer

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"epos-proxy/logger"
	"epos-proxy/util"

	"golang.org/x/sys/unix"
)

var rfcommBindDisabled = false
var rfcommBindDisabledMu sync.RWMutex

func isRFCOMMBindDisabled() bool {
	rfcommBindDisabledMu.RLock()
	defer rfcommBindDisabledMu.RUnlock()
	return rfcommBindDisabled
}

func disableRFCOMMBind() {
	rfcommBindDisabledMu.Lock()
	rfcommBindDisabled = true
	rfcommBindDisabledMu.Unlock()
}

// ---------------------------------------------------------------------------
// Raw RFCOMM socket (fallback / probe path)
// ---------------------------------------------------------------------------

// dialRFCOMM opens a Bluetooth RFCOMM socket to the given MAC on the given channel.
// Returns a net.Conn-compatible value on success.
func dialRFCOMM(mac string, channel int) (net.Conn, error) {
	mac = util.NormalizeMAC(mac)

	addr, err := util.ParseMACToBytes(mac)
	if err != nil {
		return nil, fmt.Errorf("invalid bluetooth MAC %q: %w", mac, err)
	}

	fd, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_STREAM, unix.BTPROTO_RFCOMM)
	if err != nil {
		return nil, fmt.Errorf("create RFCOMM socket failed: %w", err)
	}

	cleanup := func() { _ = unix.Close(fd) }

	unix.CloseOnExec(fd)

	if err := unix.SetNonblock(fd, true); err != nil {
		cleanup()
		return nil, fmt.Errorf("set nonblocking mode failed: %w", err)
	}

	_ = unix.SetsockoptLinger(fd, unix.SOL_SOCKET, unix.SO_LINGER, &unix.Linger{Onoff: 1, Linger: 1})

	sa := &unix.SockaddrRFCOMM{Addr: addr, Channel: uint8(channel)}
	err = unix.Connect(fd, sa)
	if err != nil && err != unix.EINPROGRESS && err != unix.EAGAIN {
		cleanup()
		return nil, fmt.Errorf("RFCOMM connect to %s channel %d failed: %w", mac, channel, err)
	}

	pollFds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLOUT}}
	n, err := unix.Poll(pollFds, int(btConnectTimeout.Milliseconds()))
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("RFCOMM poll failed: %w", err)
	}
	if n == 0 {
		cleanup()
		return nil, fmt.Errorf("RFCOMM connect timeout to %s channel %d", mac, channel)
	}

	if pollFds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
		soErr, _ := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
		cleanup()
		if soErr != 0 {
			return nil, fmt.Errorf("RFCOMM connect to %s channel %d failed: errno=%d (%s)",
				mac, channel, soErr, syscall.Errno(soErr).Error())
		}
		return nil, fmt.Errorf("RFCOMM connect to %s channel %d failed", mac, channel)
	}

	soErr, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("SO_ERROR check failed: %w", err)
	}
	if soErr != 0 {
		cleanup()
		return nil, fmt.Errorf("RFCOMM connect to %s channel %d failed: errno=%d (%s)",
			mac, channel, soErr, syscall.Errno(soErr).Error())
	}

	file := os.NewFile(uintptr(fd), fmt.Sprintf("rfcomm-%s-%d", mac, channel))
	if file == nil {
		cleanup()
		return nil, fmt.Errorf("failed to create os.File from RFCOMM socket")
	}

	return &serialConn{f: file, path: fmt.Sprintf("rfcomm-%s-%d", mac, channel)}, nil
}

// ---------------------------------------------------------------------------
// SDP discovery
// ---------------------------------------------------------------------------

// sdpDiscoverChannel runs sdptool browse and returns the RFCOMM channel for
// the Serial Port service.  Supports two SDP output formats:
//   - Mode A: "Service Name: Serial Port" header → "Channel: N"
//   - Mode B: "UUID: 0x1101" (or variant) → "Channel: N" (no named header)
func sdpDiscoverChannel(mac string) (int, error) {
	mac = util.NormalizeMAC(mac)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sdptool", "browse", mac).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return 0, fmt.Errorf("sdptool timeout")
	}
	if err != nil {
		return 0, fmt.Errorf("sdptool failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	outStr := string(out)
	logger.Debugf("BT/SDP: raw sdptool output for %s:\n%s", mac, strings.TrimSpace(outStr))

	inSerialPort := false // mode A: inside "Service Name: Serial Port" block
	uuidSerial := false   // mode B: saw the Serial Port UUID line (0x1101)

	for _, line := range strings.Split(outStr, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if strings.HasPrefix(lower, "service name:") {
			inSerialPort = strings.Contains(lower, "serial port")
			uuidSerial = false
			continue
		}

		if strings.HasPrefix(lower, "uuid") &&
			(strings.Contains(lower, "0x1101") ||
				strings.Contains(lower, "00001101") ||
				strings.Contains(lower, "serial port")) {
			uuidSerial = true
			continue
		}

		if strings.HasPrefix(lower, "channel:") && (inSerialPort || uuidSerial) {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				ch, err := strconv.Atoi(parts[1])
				if err == nil && ch > 0 {
					logger.Infof("BT/SDP: found Serial Port channel %d for %s (mode-A=%v, mode-B=%v)",
						ch, mac, inSerialPort, uuidSerial)
					return ch, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("no RFCOMM channel found in SDP output")
}

// ---------------------------------------------------------------------------
// RFCOMM binding management
// ---------------------------------------------------------------------------

// listRFCOMMBindings runs `rfcomm -a` and returns index → normalised-MAC for
// all currently bound RFCOMM devices.
func listRFCOMMBindings() (map[int]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "rfcomm", "-a").CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("rfcomm -a timed out")
	}
	if err != nil {
		return nil, fmt.Errorf("rfcomm -a failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	bindings := make(map[int]string)
	// Example: "rfcomm0: AA:BB:CC:DD:EE:FF channel 1 clean"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		devName := strings.TrimSpace(line[:colonIdx])
		if !strings.HasPrefix(devName, "rfcomm") {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimPrefix(devName, "rfcomm"))
		if err != nil {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(line[colonIdx+1:]))
		if len(fields) < 1 {
			continue
		}
		bindings[idx] = util.NormalizeMAC(fields[0])
	}

	logger.Debugf("BT/RFCOMM: listRFCOMMBindings found %d bound device(s): %v", len(bindings), bindings)
	return bindings, nil
}

// findExistingRFCOMMDevice searches current bindings for one that matches mac.
func findExistingRFCOMMDevice(mac string) (*rfcommBinding, bool) {
	mac = util.NormalizeMAC(mac)

	bindings, err := listRFCOMMBindings()
	if err != nil {
		logger.Warnf("BT/RFCOMM: could not list RFCOMM bindings: %v", err)
		return nil, false
	}

	for idx, boundMAC := range bindings {
		if boundMAC == mac {
			devPath := fmt.Sprintf("/dev/rfcomm%d", idx)
			if _, err := os.Stat(devPath); err != nil {
				logger.Warnf("BT/RFCOMM: binding for %s found at index %d but %s does not exist: %v",
					mac, idx, devPath, err)
			}
			logger.Infof("BT/RFCOMM: existing binding found for %s → %s", mac, devPath)
			return &rfcommBinding{DevPath: devPath, Index: idx, Channel: 0}, true
		}
	}

	logger.Debugf("BT/RFCOMM: no existing RFCOMM binding for %s", mac)
	return nil, false
}

// findFreeRFCOMMIndex returns the lowest RFCOMM index (0..31) not in existing.
// Returns -1 if all are occupied.
func findFreeRFCOMMIndex(existing map[int]string) int {
	for i := 0; i <= 31; i++ {
		if _, used := existing[i]; !used {
			return i
		}
	}
	return -1
}

// bindRFCOMM executes `rfcomm bind <index> <MAC> <channel>` and waits up to
// 2 s for the /dev/rfcommX device node to appear.  Requires CAP_NET_ADMIN.
func bindRFCOMM(mac string, channel int) (*rfcommBinding, error) {
	mac = util.NormalizeMAC(mac)

	existing, err := listRFCOMMBindings()
	if err != nil {
		return nil, fmt.Errorf("BT/RFCOMM: cannot list bindings (is rfcomm installed?): %w", err)
	}

	// Race-check: MAC may have been bound since findExistingRFCOMMDevice ran.
	for idx, boundMAC := range existing {
		if boundMAC == mac {
			devPath := fmt.Sprintf("/dev/rfcomm%d", idx)
			logger.Infof("BT/RFCOMM: bind skipped — %s already bound as %s", mac, devPath)
			return &rfcommBinding{DevPath: devPath, Index: idx, Channel: channel}, nil
		}
	}

	idx := findFreeRFCOMMIndex(existing)
	if idx < 0 {
		return nil, fmt.Errorf("BT/RFCOMM: no free RFCOMM index available (all 0..31 are occupied)")
	}

	devPath := fmt.Sprintf("/dev/rfcomm%d", idx)
	args := []string{"bind", strconv.Itoa(idx), mac, strconv.Itoa(channel)}
	logger.Infof("BT/RFCOMM: running: rfcomm %s", strings.Join(args, " "))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "rfcomm", args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("BT/RFCOMM: rfcomm bind timed out")
	}
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if strings.Contains(outStr, "Operation not permitted") ||
			strings.Contains(outStr, "permission denied") ||
			strings.Contains(err.Error(), "permission denied") {
			return nil, fmt.Errorf(
				"BT/RFCOMM: rfcomm bind failed — insufficient privileges "+
					"(run as root or add user to 'bluetooth'/'dialout' group): %w; output: %s",
				err, outStr)
		}
		return nil, fmt.Errorf("BT/RFCOMM: rfcomm bind failed: %w; output: %s", err, outStr)
	}

	logger.Infof("BT/RFCOMM: rfcomm bind succeeded, waiting for %s to appear…", devPath)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(devPath); err == nil {
			logger.Infof("BT/RFCOMM: device %s is ready", devPath)
			return &rfcommBinding{DevPath: devPath, Index: idx, Channel: channel}, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	disableRFCOMMBind()
	return nil, fmt.Errorf("BT/RFCOMM: %s did not appear after rfcomm bind", devPath)
}

// openRFCOMMDevice opens /dev/rfcommX for read/write.
// Does not require elevated privileges when the device node already exists.
func openRFCOMMDevice(b *rfcommBinding) (net.Conn, error) {
	f, err := os.OpenFile(b.DevPath, os.O_RDWR, 0)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf(
				"BT/RFCOMM: cannot open %s — add user to 'dialout' or 'bluetooth' group: %w",
				b.DevPath, err)
		}
		return nil, fmt.Errorf("BT/RFCOMM: failed to open %s: %w", b.DevPath, err)
	}
	logger.Infof("BT/RFCOMM: opened %s as serial connection", b.DevPath)
	return &serialConn{f: f, path: b.DevPath}, nil
}

// ---------------------------------------------------------------------------
// Full orchestration
// ---------------------------------------------------------------------------

// ensureRFCOMMDevice finds or creates a /dev/rfcommX device for mac.
func ensureRFCOMMDevice(mac string, sdpChannel int) (*rfcommBinding, error) {
	mac = util.NormalizeMAC(mac)

	if isRFCOMMBindDisabled() {
		return nil, fmt.Errorf("BT/RFCOMM: rfcomm bind is disabled on this system")
	}

	// 1. Cache hit.
	if b, ok := globalRFCOMMCache.get(mac); ok {
		logger.Debugf("BT/RFCOMM: cache hit for %s → %s (channel %d)", mac, b.DevPath, b.Channel)
		if b.DevPath == "raw" {
			return nil, fmt.Errorf("BT/RFCOMM: raw cache hit")
		}
		if _, err := os.Stat(b.DevPath); err == nil {
			return b, nil
		}
		logger.Warnf("BT/RFCOMM: cached device %s no longer exists; re-binding", b.DevPath)
	}

	// 2. Reuse existing kernel binding.
	if existing, ok := findExistingRFCOMMDevice(mac); ok {
		if sdpChannel > 0 {
			existing.Channel = sdpChannel
		}
		if existing.Channel == 0 {
			existing.Channel = 1
		}
		logger.Infof("BT/RFCOMM: reusing existing device %s for %s (channel %d)",
			existing.DevPath, mac, existing.Channel)
		globalRFCOMMCache.set(mac, existing)
		return existing, nil
	}

	// 3. Bind a new device.
	channel := sdpChannel
	if channel <= 0 {
		logger.Infof("BT/RFCOMM: SDP gave no channel for %s; will probe channels 1..8 after bind", mac)
		channel = 1
	} else {
		logger.Infof("BT/RFCOMM: using SDP-discovered channel %d for %s", channel, mac)
	}

	b, err := bindRFCOMM(mac, channel)
	if err != nil {
		return nil, err
	}

	globalRFCOMMCache.set(mac, b)
	logger.Infof("BT/RFCOMM: bound %s → %s (channel %d)", mac, b.DevPath, b.Channel)
	return b, nil
}

func releaseRFCOMM(index int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "rfcomm", "release", strconv.Itoa(index)).CombinedOutput()
	if err != nil {
		logger.Debugf("BT/RFCOMM: rfcomm release for index %d returned: %v; output: %s", index, err, strings.TrimSpace(string(out)))
		return err
	}
	logger.Infof("BT/RFCOMM: released rfcomm%d", index)
	return nil
}

// tryRFCOMMDevice ensures a device exists for the given channel and opens it.
// If the /dev/rfcommX device cannot be bound or opened, it falls back to a raw RFCOMM socket.
func tryRFCOMMDevice(mac string, channel int) (net.Conn, error) {
	if b, ok := globalRFCOMMCache.get(mac); ok && b.Channel != channel {
		globalRFCOMMCache.mu.Lock()
		delete(globalRFCOMMCache.entries, mac)
		globalRFCOMMCache.mu.Unlock()
		_ = releaseRFCOMM(b.Index)
	}

	b, err := ensureRFCOMMDevice(mac, channel)
	if err == nil {
		conn, err := openRFCOMMDevice(b)
		if err == nil {
			return conn, nil
		}
		logger.Warnf("BT/RFCOMM: failed to open bound device %s: %v", b.DevPath, err)
		_ = releaseRFCOMM(b.Index)
		globalRFCOMMCache.mu.Lock()
		delete(globalRFCOMMCache.entries, mac)
		globalRFCOMMCache.mu.Unlock()
	} else {
		logger.Warnf("BT/RFCOMM: binding device failed: %v", err)
	}

	logger.Infof("BT/RFCOMM: falling back to raw RFCOMM socket for channel %d", channel)
	conn, err := dialRFCOMM(mac, channel)
	if err == nil {
		globalRFCOMMCache.set(mac, &rfcommBinding{DevPath: "raw", Channel: channel, Index: -1})
		return conn, nil
	}
	return nil, err
}

// dialRFCOMMPlatform is the Linux Bluetooth dial entry point.
// Strategy: Try cached channel → SDP → probe channels 1–8 → raw socket fallback.
func dialRFCOMMPlatform(mac string, cachedChannel int) (net.Conn, error) {
	mac = util.NormalizeMAC(mac)

	// Step 0: Cache hit for raw socket or working device.
	if b, ok := globalRFCOMMCache.get(mac); ok {
		if b.DevPath == "raw" {
			logger.Infof("BT/RFCOMM: cache hit for raw socket connection to %s on channel %d", mac, b.Channel)
			conn, err := dialRFCOMM(mac, b.Channel)
			if err == nil {
				return conn, nil
			}
			logger.Warnf("BT/RFCOMM: cached raw socket connection failed: %v; clearing cache", err)
			globalRFCOMMCache.mu.Lock()
			delete(globalRFCOMMCache.entries, mac)
			globalRFCOMMCache.mu.Unlock()
		}
	}

	// Step 1: Try cached channel first if it exists.
	if cachedChannel > 0 {
		logger.Infof("BT/RFCOMM: trying cached channel %d for %s", cachedChannel, mac)
		if conn, err := tryRFCOMMDevice(mac, cachedChannel); err == nil {
			logger.Infof("BT/RFCOMM: connected successfully on cached channel %d for %s", cachedChannel, mac)
			return conn, nil
		}
		logger.Warnf("BT/RFCOMM: connection on cached channel %d failed for %s", cachedChannel, mac)
	}

	// Step 2: Perform SDP to discover the channel.
	var sdpChannel int
	ch, err := sdpDiscoverChannel(mac)
	if err != nil {
		logger.Warnf("BT/RFCOMM: SDP discovery failed for %s: %v", mac, err)
	} else {
		sdpChannel = ch
		logger.Infof("BT/RFCOMM: SDP discovered channel %d for %s", ch, mac)
	}

	// Step 3: Try connection on SDP channel (if it's different from the failed cached channel).
	if sdpChannel > 0 && sdpChannel != cachedChannel {
		if conn, err := tryRFCOMMDevice(mac, sdpChannel); err == nil {
			logger.Infof("BT/RFCOMM: connected successfully on SDP channel %d for %s", sdpChannel, mac)
			return conn, nil
		} else {
			logger.Warnf("BT/RFCOMM: connection on SDP channel %d failed for %s: %v; probing",
				sdpChannel, mac, err)
		}
	}

	// Step 4: Channel probing.
	probeChannels := []int{1, 2, 3, 4, 5, 6, 7, 8}
	filtered := probeChannels[:0]
	for _, ch := range probeChannels {
		if ch != cachedChannel && ch != sdpChannel {
			filtered = append(filtered, ch)
		}
	}
	probeChannels = filtered

	for _, ch := range probeChannels {
		logger.Infof("BT/RFCOMM: probing channel %d for %s", ch, mac)
		if conn, err := tryRFCOMMDevice(mac, ch); err == nil {
			logger.Infof("BT/RFCOMM: channel probe succeeded on channel %d for %s", ch, mac)
			return conn, nil
		} else {
			logger.Debugf("BT/RFCOMM: channel %d probe failed for %s: %v", ch, mac, err)
			globalRFCOMMCache.mu.Lock()
			delete(globalRFCOMMCache.entries, mac)
			globalRFCOMMCache.mu.Unlock()
		}
	}

	// Step 5: Raw socket fallback.
	logger.Warnf("BT/RFCOMM: /dev/rfcommX approach exhausted for %s; falling back to raw RFCOMM socket", mac)
	fallbackCh := sdpChannel
	if fallbackCh == 0 {
		fallbackCh = cachedChannel
	}
	if fallbackCh == 0 {
		fallbackCh = 1
	}
	conn, err := dialRFCOMM(mac, fallbackCh)
	if err != nil {
		return nil, fmt.Errorf("BT/RFCOMM: all connection strategies failed for %s: %w", mac, err)
	}
	logger.Infof("BT/RFCOMM: raw socket fallback succeeded for %s on channel %d", mac, fallbackCh)
	globalRFCOMMCache.set(mac, &rfcommBinding{DevPath: "raw", Channel: fallbackCh, Index: -1})
	return conn, nil
}

// ---------------------------------------------------------------------------
// Bluetooth scanner
// ---------------------------------------------------------------------------

// ScanBluetoothPrinters lists paired Bluetooth printers via bluetoothctl.
func ScanBluetoothPrinters() ([]BluetoothPrinterInfo, error) {
	logger.Debug("BT: scanning for Bluetooth printers on Linux")

	out, err := exec.Command("bluetoothctl", "devices").Output()
	if err != nil {
		return nil, fmt.Errorf("bluetoothctl devices failed: %w — is bluez installed?", err)
	}

	var devices []BluetoothPrinterInfo
	seen := map[string]bool{}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		// Format: "Device AA:BB:CC:DD:EE:FF Device Name"
		if !strings.HasPrefix(line, "Device ") {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}
		mac := util.NormalizeMAC(parts[1])
		if seen[mac] {
			continue
		}
		name := "Unknown"
		if len(parts) == 3 {
			name = strings.TrimSpace(parts[2])
		}

		infoOut, err := exec.Command("bluetoothctl", "info", mac).Output()
		if err != nil {
			logger.Warnf("BT: failed to get info for %s: %v", mac, err)
			continue
		}
		info := strings.ToLower(string(infoOut))

		hasPrinterIcon := strings.Contains(info, "icon: printer")
		hasSPP := strings.Contains(info, "uuid: serial port")
		if !hasPrinterIcon || !hasSPP {
			continue
		}

		seen[mac] = true
		devices = append(devices, newBluetoothPrinter(mac, name))
	}

	logger.Infof("BT: found %d Bluetooth printers", len(devices))
	return devices, nil
}
