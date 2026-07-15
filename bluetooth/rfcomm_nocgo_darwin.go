//go:build darwin && !cgo

// rfcomm_nocgo_darwin.go – compile-time stub for CGO_ENABLED=0 builds.
//
// Wails' binding generator and cross-compilation tool chains frequently set
// CGO_ENABLED=0.  In that mode the Go toolchain excludes any file that
// contains "import C", so rfcomm_darwin.go is invisible and the package would
// fail with "undefined: Connect".
//
// This stub defines the same exported API surface (Connection + Connect) so
// the package always compiles.  The functions return errors at runtime if
// someone actually calls them without cgo, which cannot happen on a real
// macOS binary because cgo is always available there.

package bluetooth

import (
	"errors"
	"net"
	"time"
)

// errNoCGO is returned by every method when the binary was built without cgo.
var errNoCGO = errors.New(
	"bluetooth/rfcomm: Bluetooth Classic (IOBluetooth) requires CGO_ENABLED=1 on macOS")

// rfcommAddr is a minimal net.Addr implementation for RFCOMM connections.
// Defined here (and not in rfcomm_darwin.go) so it is available in both the
// cgo and !cgo build variants of the package.
type rfcommAddr struct{ addr string }

func (a rfcommAddr) Network() string { return "rfcomm" }
func (a rfcommAddr) String() string  { return a.addr }

// Connection is a placeholder type for non-cgo builds.
// On real macOS binaries (which always have cgo), this type is provided by
// rfcomm_darwin.go with a concrete IOBluetooth implementation.
type Connection struct{ mac string }

// Read implements net.Conn; always returns an error in non-cgo builds.
func (c *Connection) Read(_ []byte) (int, error) { return 0, errNoCGO }

// Write implements net.Conn; always returns an error in non-cgo builds.
func (c *Connection) Write(_ []byte) (int, error) { return 0, errNoCGO }

// Close implements net.Conn; no-op in non-cgo builds.
func (c *Connection) Close() error { return nil }

// LocalAddr implements net.Conn.
func (c *Connection) LocalAddr() net.Addr { return rfcommAddr{"local"} }

// RemoteAddr implements net.Conn.
func (c *Connection) RemoteAddr() net.Addr { return rfcommAddr{c.mac} }

// SetDeadline implements net.Conn.
func (c *Connection) SetDeadline(_ time.Time) error { return nil }

// SetReadDeadline implements net.Conn.
func (c *Connection) SetReadDeadline(_ time.Time) error { return nil }

// SetWriteDeadline implements net.Conn.
func (c *Connection) SetWriteDeadline(_ time.Time) error { return nil }

// Connect always returns errNoCGO in non-cgo builds.
func Connect(mac string, rfchannel uint8) (*Connection, error) {
	return nil, errNoCGO
}
