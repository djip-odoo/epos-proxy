//go:build darwin

package printer

import (
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"

	"epos-proxy/logger"

	"golang.org/x/sys/unix"
)

// ---------------------------------------------------------------------------
// Raw RFCOMM socket
// ---------------------------------------------------------------------------

func dialRFCOMM(mac string, channel int) (net.Conn, error) {
	mac = NormalizeMAC(mac)

	addr, err := parseMACToBytes(mac)
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

	_ = unix.SetsockoptLinger(fd, unix.SOL_SOCKET, unix.SO_LINGER,
		&unix.Linger{Onoff: 1, Linger: 1})

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
// SDP (not available on macOS via sdptool)
// ---------------------------------------------------------------------------

func sdpDiscoverChannel(_ string) (int, error) {
	return 0, fmt.Errorf("SDP discovery not available on macOS")
}

// ---------------------------------------------------------------------------
// dialRFCOMMPlatform — macOS entry point
// ---------------------------------------------------------------------------

// dialRFCOMMPlatform on macOS uses a raw RFCOMM socket with the cached channel
// or defaults to channel 1.  SDP is not available so probing is not attempted.
func dialRFCOMMPlatform(mac string, cachedChannel int) (net.Conn, error) {
	logger.Infof("BT/darwin: dialling %s", mac)
	ch := cachedChannel
	if ch <= 0 {
		ch = 1
	}
	return dialRFCOMM(mac, ch)
}

// ---------------------------------------------------------------------------
// Scanner (not yet implemented on macOS)
// ---------------------------------------------------------------------------

// ScanBluetoothPrinters is not yet implemented on macOS.
func ScanBluetoothPrinters() ([]BluetoothPrinterInfo, error) {
	return nil, fmt.Errorf("Bluetooth scanning is not yet supported on macOS")
}

func GetCachedRFCOMMChannel(mac string) (int, bool) {
	return 0, false
}
