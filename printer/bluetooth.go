package printer

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// btConnectTimeout is the maximum time allowed for a single RFCOMM connect attempt.
const btConnectTimeout = 6 * time.Second

// rfcommBinding records the state of a bound (or candidate) RFCOMM device.
// On Linux the DevPath refers to an actual /dev/rfcommX node;
// on Darwin/Windows it is used only as a value holder.
type rfcommBinding struct {
	DevPath string // e.g. "/dev/rfcomm0"
	Channel int    // RFCOMM channel number
	Index   int    // numeric index (0 = rfcomm0, 1 = rfcomm1, …)
}

// parseMACToBytes converts "AA:BB:CC:DD:EE:FF" → [6]byte in reversed order
// (little-endian as required by the BlueZ sockaddr_rc).
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

type BluetoothPrinterInfo struct {
	MAC  string `json:"mac"`
	Name string `json:"name"`
	Id   string `json:"id"`
}

func NormalizeMAC(mac string) string {
	mac = strings.ToUpper(strings.TrimSpace(mac))
	mac = strings.ReplaceAll(mac, "-", ":")
	return mac
}

func ValidateMAC(mac string) error {
	mac = strings.TrimSpace(mac)
	matched, _ := regexp.MatchString(`^([0-9A-Fa-f]{2}[:\-]){5}[0-9A-Fa-f]{2}$`, mac)
	if !matched {
		return fmt.Errorf("invalid MAC address format: %s (expected format: AA:BB:CC:DD:EE:FF)", mac)
	}
	return nil
}

// CheckBluetoothPrinter verifies that a Bluetooth printer is reachable by
// opening an RFCOMM connection and immediately closing it.
// On Linux it uses the RFCOMM device binding path (rfcomm bind + /dev/rfcommX)
// so that printers with broken SDP records are handled correctly.
// On other platforms it falls back to a raw RFCOMM socket.
func CheckBluetoothPrinter(mac string, cachedChannel int) error {
	conn, err := dialRFCOMMPlatform(mac, cachedChannel)
	if err != nil {
		return fmt.Errorf("bluetooth printer %s is unreachable: %w", mac, err)
	}
	_ = conn.Close()
	return nil
}

type serialConn struct {
	f    *os.File
	path string
}

type serialAddr struct{ path string }

func (a serialAddr) Network() string { return "rfcomm-serial" }
func (a serialAddr) String() string  { return a.path }

func (c *serialConn) Read(b []byte) (int, error)  { return c.f.Read(b) }
func (c *serialConn) Write(b []byte) (int, error) { return c.f.Write(b) }
func (c *serialConn) Close() error                { return c.f.Close() }

func (c *serialConn) LocalAddr() net.Addr  { return serialAddr{c.path} }
func (c *serialConn) RemoteAddr() net.Addr { return serialAddr{c.path} }

func (c *serialConn) SetDeadline(t time.Time) error      { return nil }
func (c *serialConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *serialConn) SetWriteDeadline(t time.Time) error { return nil }
