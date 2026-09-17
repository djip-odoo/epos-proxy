package bluetooth

import (
	"net"
	"os"
	"sync"
	"time"
)

// btConnectTimeout is the maximum time allowed for a single RFCOMM connect attempt.
const btConnectTimeout = 3 * time.Second

// rfcommBinding records the state of a bound (or candidate) RFCOMM device.
// On Linux the DevPath refers to an actual /dev/rfcommX node;
// on Darwin/Windows it is used only as a value holder.
type rfcommBinding struct {
	DevPath string // e.g. "/dev/rfcomm0"
	Channel int    // RFCOMM channel number
	Index   int    // numeric index (0 = rfcomm0, 1 = rfcomm1, …)
}

type rfcommCache struct {
	mu      sync.RWMutex
	entries map[string]*rfcommBinding
}

func (c *rfcommCache) get(address string) (*rfcommBinding, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	b, ok := c.entries[address]
	if !ok || b == nil {
		return nil, false
	}
	copy := *b
	return &copy, true
}

func (c *rfcommCache) set(address string, b *rfcommBinding) {
	if b == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	copy := *b
	c.entries[address] = &copy
}

func (c *rfcommCache) delete(address string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, address)
}

func (c *rfcommCache) deleteIf(address string, predicate func(*rfcommBinding) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if b, ok := c.entries[address]; ok {
		if predicate == nil || predicate(b) {
			delete(c.entries, address)
		}
	}
}

func (bm *BluetoothManager) setBinding(address string, b *rfcommBinding) {
	if b == nil {
		return
	}
	address = normalizeAddress(address)
	bm.cache.set(address, b)
}

func (bm *BluetoothManager) deleteBinding(address string) {
	address = normalizeAddress(address)
	bm.cache.delete(address)
}

func (bm *BluetoothManager) deleteBindingIfMatching(address string, expected *rfcommBinding) {
	if expected == nil {
		return
	}
	address = normalizeAddress(address)
	bm.cache.deleteIf(address, func(b *rfcommBinding) bool {
		return b != nil && b.Channel == expected.Channel && b.DevPath == expected.DevPath && b.Index == expected.Index
	})
}

func (bm *BluetoothManager) deleteBindingForChannel(address string, channel int) {
	address = normalizeAddress(address)
	bm.cache.deleteIf(address, func(b *rfcommBinding) bool {
		return b != nil && (channel <= 0 || b.Channel == channel)
	})
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

func (c *serialConn) LocalAddr() net.Addr {
	return netAddrPlaceholder{net: "rfcomm-serial", addr: c.path}
}
func (c *serialConn) RemoteAddr() net.Addr {
	return netAddrPlaceholder{net: "rfcomm-serial", addr: c.path}
}

func (c *serialConn) SetDeadline(t time.Time) error      { return nil }
func (c *serialConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *serialConn) SetWriteDeadline(t time.Time) error { return nil }

type netAddrPlaceholder struct {
	net  string
	addr string
}

func (a netAddrPlaceholder) Network() string { return a.net }
func (a netAddrPlaceholder) String() string  { return a.addr }
