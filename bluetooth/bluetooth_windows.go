//go:build windows

package bluetooth

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"epos-proxy/logger"
	"epos-proxy/util"

	"golang.org/x/sys/windows"
)

// ---------------------------------------------------------------------------
// Windows Bluetooth socket constants
// ---------------------------------------------------------------------------

const (
	afBTH          = 32 // AF_BTH
	bthProtoRFCOMM = 3  // BTHPROTO_RFCOMM

	soSndtimeo = 0x1005 // SO_SNDTIMEO
	soRcvtimeo = 0x1006 // SO_RCVTIMEO

	// SOCKADDR_BTH is #pragma pack(1) in ws2bth.h — 30 bytes total:
	//   [0:2]  addressFamily (uint16)
	//   [2:10] btAddr        (uint64)
	//   [10:26] serviceClassId (GUID, 16 bytes)
	//   [26:30] port         (uint32)
	sockaddrBTHSize = 30
)

var (
	ws2_32          = windows.NewLazySystemDLL("ws2_32.dll")
	procSocket      = ws2_32.NewProc("socket")
	procConnect     = ws2_32.NewProc("connect")
	procSend        = ws2_32.NewProc("send")
	procRecv        = ws2_32.NewProc("recv")
	procCloseSocket = ws2_32.NewProc("closesocket")
	procSetsockopt  = ws2_32.NewProc("setsockopt")

	bluetoothAPIs        = windows.NewLazySystemDLL("BluetoothAPIs.dll")
	procBTFindFirst      = bluetoothAPIs.NewProc("BluetoothFindFirstDevice")
	procBTFindNext       = bluetoothAPIs.NewProc("BluetoothFindNextDevice")
	procBTFindClose      = bluetoothAPIs.NewProc("BluetoothFindDeviceClose")
	procBTFindFirstRadio = bluetoothAPIs.NewProc("BluetoothFindFirstRadio")
	procBTFindRadioClose = bluetoothAPIs.NewProc("BluetoothFindRadioClose")
)

// ---------------------------------------------------------------------------
// Windows Bluetooth API structures
// ---------------------------------------------------------------------------

// BLUETOOTH_DEVICE_SEARCH_PARAMS — parameter block for BluetoothFindFirstDevice.
type btDeviceSearchParams struct {
	dwSize               uint32
	fReturnAuthenticated uint32
	fReturnRemembered    uint32
	fReturnUnknown       uint32
	fReturnConnected     uint32
	fIssueInquiry        uint32
	cTimeoutMultiplier   uint8
	_                    [3]byte
	hRadio               uintptr
}

// SYSTEMTIME used inside BLUETOOTH_DEVICE_INFO.
type winSYSTEMTIME struct {
	Year, Month, DayOfWeek, Day uint16
	Hour, Minute, Second, Ms    uint16
}

// BLUETOOTH_DEVICE_INFO — 560 bytes, matches Windows SDK layout.
type btDeviceInfo struct {
	dwSize          uint32
	_               [4]byte // padding so Address aligns to 8
	Address         uint64  // BTH_ADDR
	ulClassOfDevice uint32
	fConnected      uint32
	fRemembered     uint32
	fAuthenticated  uint32
	stLastSeen      winSYSTEMTIME
	stLastUsed      winSYSTEMTIME
	szName          [248]uint16 // BLUETOOTH_MAX_NAME_SIZE
}

func ScanBluetoothPrinters() ([]BluetoothPrinterInfo, error) {
	logger.Debug("BT: scanning for Bluetooth printers on windows")
	if err := bluetoothAPIs.Load(); err != nil {
		return nil, fmt.Errorf("BluetoothAPIs.dll unavailable: %w", err)
	}

	params := btDeviceSearchParams{
		fReturnAuthenticated: 1,
		fReturnRemembered:    1,
		fReturnConnected:     1,
	}
	params.dwSize = uint32(unsafe.Sizeof(params))

	var info btDeviceInfo
	info.dwSize = uint32(unsafe.Sizeof(info))

	handle, _, e := procBTFindFirst.Call(
		uintptr(unsafe.Pointer(&params)),
		uintptr(unsafe.Pointer(&info)),
	)
	const invalidHandle = ^uintptr(0)
	if handle == invalidHandle || handle == 0 {
		if e == windows.ERROR_NO_MORE_ITEMS {
			return nil, nil
		}
		return nil, fmt.Errorf("BluetoothFindFirstDevice failed: %w", e)
	}
	defer procBTFindClose.Call(handle)

	var devices []BluetoothPrinterInfo

	for {
		mac := btAddrToMAC(info.Address)
		name := utf16ToString(info.szName[:])
		if name == "" {
			name = mac
		}

		logger.Debugf("BT/Windows: found device %s (%s)", name, mac)

		devices = append(devices, BluetoothPrinterInfo{MAC: util.NormalizeMAC(mac), Name: name})
		logger.Debugf("BT/Windows: listing paired device %s (%s)", name, mac)

		info = btDeviceInfo{}
		info.dwSize = uint32(unsafe.Sizeof(info))
		r, _, _ := procBTFindNext.Call(handle, uintptr(unsafe.Pointer(&info)))
		if r == 0 {
			break
		}
	}

	logger.Debugf("BT/Windows: found %d Bluetooth printer(s)", len(devices))
	return devices, nil
}

func IsBluetoothAdapterActive() bool {
	if err := bluetoothAPIs.Load(); err != nil {
		return false
	}

	var params struct {
		dwSize uint32
	}
	params.dwSize = 4
	var hRadio syscall.Handle
	hFind, _, _ := procBTFindFirstRadio.Call(
		uintptr(unsafe.Pointer(&params)),
		uintptr(unsafe.Pointer(&hRadio)),
	)

	const invalidHandle = ^uintptr(0)
	if hFind == 0 || hFind == invalidHandle {
		return false
	}

	_ = syscall.CloseHandle(hRadio)
	_, _, _ = procBTFindRadioClose.Call(hFind)
	return true
}

func CheckDependencies() []DependencyStatus {
	return []DependencyStatus{}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// Converts "AA:BB:CC:DD:EE:FF" to the uint64 BTH_ADDR expected by Windows (big-endian in a 64-bit integer).
func macToWindowsBTHAddr(mac string) (uint64, error) {
	parts := strings.Split(strings.ToUpper(mac), ":")
	if len(parts) != 6 {
		return 0, fmt.Errorf("invalid MAC: %s", mac)
	}
	var addr uint64
	for _, p := range parts {
		v, err := parseHexByte(p)
		if err != nil {
			return 0, fmt.Errorf("invalid MAC octet %q: %w", p, err)
		}
		addr = (addr << 8) | uint64(v)
	}
	return addr, nil
}

func parseHexByte(s string) (byte, error) {
	var v uint64
	for _, c := range s {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= uint64(c - '0')
		case c >= 'A' && c <= 'F':
			v |= uint64(c-'A') + 10
		case c >= 'a' && c <= 'f':
			v |= uint64(c-'a') + 10
		default:
			return 0, fmt.Errorf("invalid hex char %q", c)
		}
	}
	return byte(v), nil
}

// Builds the packed SOCKADDR_BTH byte array (30 bytes).
func makeSockaddrBTH(btAddr uint64, channel uint32) [sockaddrBTHSize]byte {
	var sa [sockaddrBTHSize]byte
	binary.LittleEndian.PutUint16(sa[0:2], afBTH)
	binary.LittleEndian.PutUint64(sa[2:10], btAddr)
	// sa[10:26] = GUID (zeros → RFCOMM any service)
	binary.LittleEndian.PutUint32(sa[26:30], channel)
	return sa
}

// Converts a Windows BTH_ADDR uint64 back to "AA:BB:CC:DD:EE:FF".
func btAddrToMAC(addr uint64) string {
	b := make([]byte, 6)
	for i := 5; i >= 0; i-- {
		b[i] = byte(addr & 0xFF)
		addr >>= 8
	}
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", b[0], b[1], b[2], b[3], b[4], b[5])
}

// Converts a null-terminated UTF-16 slice to a Go string.
func utf16ToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			return windows.UTF16ToString(s[:i])
		}
	}
	return windows.UTF16ToString(s)
}

// ---------------------------------------------------------------------------
// windowsBTConn — net.Conn over a raw Winsock RFCOMM socket handle
// ---------------------------------------------------------------------------

type btAddr struct {
	mac string
	ch  int
}

func (a btAddr) Network() string { return "rfcomm" }
func (a btAddr) String() string  { return fmt.Sprintf("%s/%d", a.mac, a.ch) }

type windowsBTConn struct {
	sock syscall.Handle
	mac  string
	ch   int
}

func (c *windowsBTConn) LocalAddr() net.Addr  { return btAddr{c.mac, c.ch} }
func (c *windowsBTConn) RemoteAddr() net.Addr { return btAddr{c.mac, c.ch} }

func (c *windowsBTConn) Read(b []byte) (int, error) {
	r, _, err := procRecv.Call(
		uintptr(c.sock),
		uintptr(unsafe.Pointer(&b[0])),
		uintptr(len(b)),
		0,
	)
	if int32(r) < 0 {
		return 0, fmt.Errorf("recv failed: %w", err)
	}
	return int(r), nil
}

func (c *windowsBTConn) Write(b []byte) (int, error) {
	total := 0
	for len(b) > 0 {
		r, _, err := procSend.Call(
			uintptr(c.sock),
			uintptr(unsafe.Pointer(&b[0])),
			uintptr(len(b)),
			0,
		)
		if int32(r) < 0 {
			return total, fmt.Errorf("send failed: %w", err)
		}
		n := int(r)
		total += n
		b = b[n:]
	}
	return total, nil
}

func (c *windowsBTConn) Close() error {
	r, _, err := procCloseSocket.Call(uintptr(c.sock))
	if r != 0 {
		return fmt.Errorf("closesocket failed: %w", err)
	}
	return nil
}

// setTimeoutMS sets SO_SNDTIMEO or SO_RCVTIMEO on the socket.
func (c *windowsBTConn) setTimeoutMS(optname int32, ms int32) error {
	r, _, err := procSetsockopt.Call(
		uintptr(c.sock),
		uintptr(windows.SOL_SOCKET),
		uintptr(optname),
		uintptr(unsafe.Pointer(&ms)),
		uintptr(4),
	)
	if r != 0 {
		return fmt.Errorf("setsockopt failed: %w", err)
	}
	return nil
}

func (c *windowsBTConn) SetDeadline(t time.Time) error {
	_ = c.SetReadDeadline(t)
	return c.SetWriteDeadline(t)
}

func (c *windowsBTConn) SetReadDeadline(t time.Time) error {
	ms := int32(time.Until(t).Milliseconds())
	if ms < 0 {
		ms = 1
	}
	return c.setTimeoutMS(soRcvtimeo, ms)
}

func (c *windowsBTConn) SetWriteDeadline(t time.Time) error {
	ms := int32(time.Until(t).Milliseconds())
	if ms < 0 {
		ms = 1
	}
	return c.setTimeoutMS(soSndtimeo, ms)
}

// ---------------------------------------------------------------------------
// dialRFCOMM — Winsock AF_BTH connection
// ---------------------------------------------------------------------------

func dialRFCOMM(mac string, channel int) (net.Conn, error) {
	mac = util.NormalizeMAC(mac)

	btAddr, err := macToWindowsBTHAddr(mac)
	if err != nil {
		return nil, fmt.Errorf("invalid bluetooth MAC %q: %w", mac, err)
	}

	r, _, e := procSocket.Call(afBTH, windows.SOCK_STREAM, bthProtoRFCOMM)
	const invalidSocket = ^uintptr(0)
	if r == invalidSocket {
		return nil, fmt.Errorf("BT socket() failed: %w", e)
	}
	sock := syscall.Handle(r)

	cleanup := func() { procCloseSocket.Call(uintptr(sock)) }

	sa := makeSockaddrBTH(btAddr, uint32(channel))

	// Blocking connect — btConnectTimeout enforced via SO_SNDTIMEO.
	timeoutMS := int32(btConnectTimeout.Milliseconds())
	procSetsockopt.Call(
		uintptr(sock),
		uintptr(windows.SOL_SOCKET),
		uintptr(soSndtimeo),
		uintptr(unsafe.Pointer(&timeoutMS)),
		4,
	)

	rc, _, e := procConnect.Call(
		uintptr(sock),
		uintptr(unsafe.Pointer(&sa[0])),
		sockaddrBTHSize,
	)
	if rc != 0 {
		cleanup()
		return nil, fmt.Errorf("RFCOMM connect to %s channel %d failed: %w", mac, channel, e)
	}

	logger.Debugf("BT/Windows: connected to %s on channel %d", mac, channel)
	return &windowsBTConn{sock: sock, mac: mac, ch: channel}, nil
}

// ---------------------------------------------------------------------------
// Platform entry point
// ---------------------------------------------------------------------------

// dialRFCOMMPlatform tries channels in order: cached → probe 1-8.
func dialRFCOMMPlatform(mac string, cachedChannel int) (net.Conn, error) {
	mac = util.NormalizeMAC(mac)
	logger.Debugf("BT/Windows: dialling %s (cached channel %d)", mac, cachedChannel)

	// Use the cached channel first.
	if cachedChannel > 0 {
		if conn, err := dialRFCOMM(mac, cachedChannel); err == nil {
			return conn, nil
		}
	}

	// Probe common printer channels.
	for _, ch := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		if ch == cachedChannel {
			continue
		}
		logger.Debugf("BT/Windows: probing channel %d for %s", ch, mac)
		if conn, err := dialRFCOMM(mac, ch); err == nil {
			logger.Debugf("BT/Windows: channel %d succeeded for %s", ch, mac)
			BTManager.setBinding(mac, &rfcommBinding{DevPath: "", Channel: ch, Index: -1})
			return conn, nil
		}
	}

	return nil, fmt.Errorf("BT/Windows: no working RFCOMM channel found for %s", mac)
}
