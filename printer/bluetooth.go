package printer

import (
	"epos-proxy/util"
	"fmt"
	"net"
	"os"
	"time"
)

// btConnectTimeout is the maximum time allowed for a single RFCOMM connect attempt.
const btConnectTimeout = 5 * time.Second

// rfcommBinding records the state of a bound (or candidate) RFCOMM device.
// On Linux the DevPath refers to an actual /dev/rfcommX node;
// on Darwin/Windows it is used only as a value holder.
type rfcommBinding struct {
	DevPath string // e.g. "/dev/rfcomm0"
	Channel int    // RFCOMM channel number
	Index   int    // numeric index (0 = rfcomm0, 1 = rfcomm1, …)
}

type BluetoothPrinterInfo struct {
	MAC  string `json:"mac"`
	Name string `json:"name"`
	Id   string `json:"id"`
}

func CheckBluetoothPrinter(mac string, cachedChannel int) error {
	conn, err := dialRFCOMMPlatform(mac, cachedChannel)
	if err != nil {
		return fmt.Errorf("bluetooth printer %s is unreachable: %w", mac, err)
	}
	_ = conn.Close()
	return nil
}

func newBluetoothPrinter(mac, name string) BluetoothPrinterInfo {
	if name == "" {
		name = mac
	}
	return BluetoothPrinterInfo{
		MAC:  util.NormalizeMAC(mac),
		Name: name,
		Id:   EncodeBluetoothPrinterID(mac),
	}
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
