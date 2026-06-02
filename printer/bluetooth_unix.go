//go:build !windows

package printer

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"epos-proxy/logger"

	"golang.org/x/sys/unix"
)

const btConnectTimeout = 6 * time.Second

// rfcommConn wraps a raw RFCOMM file descriptor as a net.Conn.
type rfcommConn struct {
	fd    int
	file  *net.TCPConn // unused placeholder; we use rawConn
	laddr rfcommAddr
	raddr rfcommAddr
}

type rfcommAddr struct {
	mac     string
	channel int
}

func (a rfcommAddr) Network() string { return "rfcomm" }
func (a rfcommAddr) String() string  { return fmt.Sprintf("%s/%d", a.mac, a.channel) }

// parseMACToBytes converts "AA:BB:CC:DD:EE:FF" → [6]byte in reversed order
// (little-endian as required by the BlueZ sockaddr).
func parseMACToBytes(mac string) ([6]byte, error) {
	parts := strings.Split(strings.ToUpper(mac), ":")
	if len(parts) != 6 {
		return [6]byte{}, fmt.Errorf("invalid MAC: %s", mac)
	}
	var b [6]byte
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return [6]byte{}, fmt.Errorf("invalid MAC octet %q: %w", p, err)
		}
		b[5-i] = byte(v) // reversed (little-endian)
	}
	return b, nil
}

// dialRFCOMM opens a Bluetooth RFCOMM socket to the given MAC on the given channel.
// Returns a net.Conn-compatible value on success.
func dialRFCOMM(mac string, channel int) (net.Conn, error) {
	mac = NormalizeMAC(mac)

	addr, err := parseMACToBytes(mac)
	if err != nil {
		return nil, fmt.Errorf("invalid bluetooth MAC %q: %w", mac, err)
	}

	fd, err := unix.Socket(
		unix.AF_BLUETOOTH,
		unix.SOCK_STREAM,
		unix.BTPROTO_RFCOMM,
	)
	if err != nil {
		return nil, fmt.Errorf("create RFCOMM socket failed: %w", err)
	}

	cleanup := func() {
		_ = unix.Close(fd)
	}

	unix.CloseOnExec(fd)

	// Non-blocking connect so we can enforce timeout ourselves.
	if err := unix.SetNonblock(fd, true); err != nil {
		cleanup()
		return nil, fmt.Errorf("set nonblocking mode failed: %w", err)
	}

	// Optional: helps avoid long hangs on close with some printers.
	_ = unix.SetsockoptLinger(
		fd,
		unix.SOL_SOCKET,
		unix.SO_LINGER,
		&unix.Linger{
			Onoff:  1,
			Linger: 1,
		},
	)

	sa := &unix.SockaddrRFCOMM{
		Addr:    addr,
		Channel: uint8(channel),
	}

	err = unix.Connect(fd, sa)
	if err != nil &&
		err != unix.EINPROGRESS &&
		err != unix.EAGAIN {
		cleanup()
		return nil, fmt.Errorf(
			"RFCOMM connect to %s channel %d failed: %w",
			mac,
			channel,
			err,
		)
	}

	// Wait until socket becomes writable.
	pollFds := []unix.PollFd{
		{
			Fd:     int32(fd),
			Events: unix.POLLOUT,
		},
	}

	timeoutMs := int(btConnectTimeout.Milliseconds())

	n, err := unix.Poll(pollFds, timeoutMs)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("RFCOMM poll failed: %w", err)
	}

	if n == 0 {
		cleanup()
		return nil, fmt.Errorf(
			"RFCOMM connect timeout to %s channel %d",
			mac,
			channel,
		)
	}

	revents := pollFds[0].Revents

	if revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
		soErr, _ := unix.GetsockoptInt(
			fd,
			unix.SOL_SOCKET,
			unix.SO_ERROR,
		)

		cleanup()

		if soErr != 0 {
			return nil, fmt.Errorf(
				"RFCOMM connect to %s channel %d failed: errno=%d (%s)",
				mac,
				channel,
				soErr,
				syscall.Errno(soErr).Error(),
			)
		}

		return nil, fmt.Errorf(
			"RFCOMM connect to %s channel %d failed",
			mac,
			channel,
		)
	}

	// Verify final socket state.
	soErr, err := unix.GetsockoptInt(
		fd,
		unix.SOL_SOCKET,
		unix.SO_ERROR,
	)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("SO_ERROR check failed: %w", err)
	}

	if soErr != 0 {
		cleanup()
		return nil, fmt.Errorf(
			"RFCOMM connect to %s channel %d failed: errno=%d (%s)",
			mac,
			channel,
			soErr,
			syscall.Errno(soErr).Error(),
		)
	}

	// Restore blocking mode for normal reads/writes.
	if err := unix.SetNonblock(fd, false); err != nil {
		cleanup()
		return nil, fmt.Errorf("restore blocking mode failed: %w", err)
	}

	file := os.NewFile(uintptr(fd), fmt.Sprintf(
		"rfcomm-%s-%d",
		mac,
		channel,
	))

	if file == nil {
		cleanup()
		return nil, fmt.Errorf("failed to create os.File from RFCOMM socket")
	}

	conn, err := net.FileConn(file)

	// net.FileConn duplicates the fd internally.
	_ = file.Close()

	if err != nil {
		return nil, fmt.Errorf("create net.Conn from RFCOMM socket failed: %w", err)
	}

	return conn, nil
}

// sdpDiscoverChannel queries the Bluetooth device for the RFCOMM channel via
// SDP using platform-specific tooling.
func sdpDiscoverChannel(mac string) (int, error) {
	mac = NormalizeMAC(mac)
	switch runtime.GOOS {
	case "linux":
		return sdpDiscoverLinux(mac)
	case "darwin":
		// macOS: we rely on channel probing; SDP via sdptool is not standard.
		return 0, fmt.Errorf("SDP not available on macOS")
	}
	return 0, fmt.Errorf("unsupported OS")
}

func sdpDiscoverLinux(mac string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	out, err := exec.CommandContext(
		ctx,
		"sdptool",
		"browse",
		mac,
	).CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return 0, fmt.Errorf("sdptool timeout")
	}

	if err != nil {
		return 0, fmt.Errorf(
			"sdptool failed: %w: %s",
			err,
			strings.TrimSpace(string(out)),
		)
	}

	outStr := string(out)
	logger.Debugf("BT/SDP: raw sdptool output for %s:\n%s", mac, strings.TrimSpace(outStr))

	lines := strings.Split(outStr, "\n")

	// Two parsing modes:
	//  A) Named-service mode  — "Service Name: Serial Port" header followed by "Channel: N"
	//  B) UUID mode           — UUID line contains "0x1101" (Serial Port UUID) and a
	//                           subsequent "Channel:" line (no named header).
	//
	// Many broken-SDP thermal printers only advertise the UUID without a service name.

	inSerialPort := false // mode A
	uuidSerial := false   // mode B: saw the Serial Port UUID line

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// ── Mode A: named service header ────────────────────────────────
		if strings.HasPrefix(lower, "service name:") {
			if strings.Contains(lower, "serial port") {
				inSerialPort = true
			} else {
				inSerialPort = false
			}
			uuidSerial = false // reset UUID mode on each new service block
			continue
		}

		// ── Mode B: UUID line containing Serial Port UUID (0x1101) ──────
		// Examples:
		//   "UUID: Serial Port (0x1101)"
		//   "UUID 128: 00001101-..."
		if strings.HasPrefix(lower, "uuid") &&
			(strings.Contains(lower, "0x1101") ||
				strings.Contains(lower, "00001101") ||
				strings.Contains(lower, "serial port")) {
			uuidSerial = true
			continue
		}

		// ── Channel extraction (both modes) ─────────────────────────────
		if strings.HasPrefix(lower, "channel:") {
			if inSerialPort || uuidSerial {
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
	}

	return 0, fmt.Errorf("no RFCOMM channel found in SDP output")
}

// dialRFCOMMPlatform is the per-OS entry point used by ensureOpenBluetoothLocked.
// On Linux it routes through the RFCOMM device binding path (rfcomm_linux.go).
// On other Unix systems it falls back directly to the raw RFCOMM socket.
func dialRFCOMMPlatform(mac string, cachedChannel int) (net.Conn, error) {
	if runtime.GOOS == "linux" {
		return dialRFCOMMLinux(mac, cachedChannel)
	}
	// macOS / other Unix: use raw RFCOMM socket (no rfcomm bind available).
	ch := cachedChannel
	if ch <= 0 {
		var err error
		ch, err = sdpDiscoverChannel(mac)
		if err != nil || ch <= 0 {
			logger.Warnf("BT/SDP: discovery failed for %s: %v; trying channel 1", mac, err)
			ch = 1
		}
	}
	return dialRFCOMM(mac, ch)
}

func ScanBluetoothPrinters() ([]BluetoothPrinterInfo, error) {
	switch runtime.GOOS {
	case "linux":
		return scanLinux()
	// case "darwin":
	// 	return scanMacOS()
	default:
		return nil, fmt.Errorf("Bluetooth scanning is not supported on %s", runtime.GOOS)
	}
}

func scanLinux() ([]BluetoothPrinterInfo, error) {
	logger.Debug("BT: scanning for Bluetooth printers on Linux")

	out, err := exec.Command("bluetoothctl", "devices").Output()
	if err != nil {
		return nil, fmt.Errorf("bluetoothctl devices failed: %w — is bluez installed?", err)
	}

	var devices []BluetoothPrinterInfo
	seen := map[string]bool{}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)

		// Format:
		// Device AA:BB:CC:DD:EE:FF Device Name
		if !strings.HasPrefix(line, "Device ") {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}

		mac := NormalizeMAC(parts[1])

		if seen[mac] {
			continue
		}

		name := "Unknown"
		if len(parts) == 3 {
			name = strings.TrimSpace(parts[2])
		}

		// Query device info
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

		devices = append(devices, BluetoothPrinterInfo{
			MAC:  mac,
			Name: name,
			Id:   EncodeBluetoothPrinterID(mac),
		})
	}

	logger.Infof("BT: found %d Bluetooth printers", len(devices))

	return devices, nil
}

// TODO: test this
// func scanMacOS() ([]BluetoothPrinterInfo, error) {
// 	logger.Debug("BT: scanning for Bluetooth devices on macOS (system_profiler)")
// 	out, err := exec.Command("system_profiler", "SPBluetoothDataType").Output()
// 	if err != nil {
// 		return nil, fmt.Errorf("system_profiler failed: %w", err)
// 	}

// 	var devices []BluetoothPrinterInfo
// 	seen := map[string]bool{}
// 	lines := strings.Split(string(out), "\n")

// 	var currentName string
// 	for _, line := range lines {
// 		trimmed := strings.TrimSpace(line)
// 		// Device name lines end with ":"
// 		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, "Address") {
// 			currentName = strings.TrimSuffix(trimmed, ":")
// 		}
// 		// Address line: "Address: AA-BB-CC-DD-EE-FF"
// 		if strings.HasPrefix(trimmed, "Address:") {
// 			parts := strings.SplitN(trimmed, ":", 2)
// 			if len(parts) == 2 {
// 				mac := NormalizeMAC(strings.TrimSpace(parts[1]))
// 				if seen[mac] || mac == "" {
// 					continue
// 				}
// 				seen[mac] = true
// 				name := currentName
// 				if name == "" {
// 					name = mac
// 				}
// 				devices = append(devices, BluetoothPrinterInfo{
// 					MAC:  mac,
// 					Name: name,
// 					Id:   EncodeBluetoothPrinterID(mac),
// 				})
// 			}
// 		}
// 	}

// 	logger.Infof("BT: found %d Bluetooth devices on macOS", len(devices))
// 	return devices, nil
// }
