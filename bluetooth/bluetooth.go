package bluetooth

import (
	"encoding/base64"
	"epos-proxy/config"
	"epos-proxy/logger"
	"epos-proxy/util"
	"fmt"
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

func (c *rfcommCache) get(mac string) (*rfcommBinding, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	b, ok := c.entries[mac]
	return b, ok
}

func (c *rfcommCache) set(mac string, b *rfcommBinding) {
	c.mu.Lock()
	defer c.mu.Unlock()

	oldBinding, ok := c.entries[mac]
	if ok && oldBinding.Channel == b.Channel && oldBinding.DevPath == b.DevPath {
		return
	}

	c.entries[mac] = b

	if BTManager != nil && BTManager.Cfg != nil {
		currentConfigChannel := BTManager.Cfg.GetBluetoothPrinterChannel(mac)
		if currentConfigChannel != b.Channel {
			logger.Infof("BT: updating config channel for %s from %d to %d", mac, currentConfigChannel, b.Channel)
			BTManager.Cfg.UpdateBluetoothChannel(mac, b.Channel)
		}
	}
}

func (c *rfcommCache) delete(mac string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, mac)
}

type BluetoothPrinterInfo struct {
	MAC  string `json:"mac"`
	Name string `json:"name"`
}

type BluetoothManager struct {
	Cfg   *config.Manager
	cache *rfcommCache
}

var BTManager *BluetoothManager

func InitBluetoothManager(cfg *config.Manager) *BluetoothManager {
	BTManager = &BluetoothManager{
		Cfg: cfg,
		cache: &rfcommCache{
			entries: make(map[string]*rfcommBinding),
		},
	}

	for _, p := range cfg.GetBluetoothPrinters() {
		if p.Channel > 0 {
			mac := util.NormalizeMAC(p.MAC)
			BTManager.cache.set(mac, &rfcommBinding{
				DevPath: "",
				Channel: p.Channel,
				Index:   -1,
			})
		}
	}
	return BTManager
}

func (bm *BluetoothManager) GetCachedRFCOMMChannel(mac string) int {
	mac = util.NormalizeMAC(mac)
	if b, ok := bm.cache.get(mac); ok {
		return b.Channel
	}
	return bm.Cfg.GetBluetoothPrinterChannel(mac)
}

func (bm *BluetoothManager) CheckBluetoothPrinter(mac string) error {
	conn, err := bm.Dial(mac)
	if err != nil {
		return fmt.Errorf("bluetooth printer %s is unreachable: %w", mac, err)
	}
	_ = conn.Close()
	return nil
}

func (bm *BluetoothManager) GetCachedBinding(mac string) (string, int, bool) {
	mac = util.NormalizeMAC(mac)
	if b, ok := bm.cache.get(mac); ok {
		return b.DevPath, b.Channel, true
	}
	return "", 0, false
}

func (bm *BluetoothManager) Dial(mac string) (net.Conn, error) {
	channel := bm.GetCachedRFCOMMChannel(mac)
	return dialRFCOMMPlatform(mac, channel)
}

func EncodeBluetoothPrinterID(mac string) string {
	return base64.RawURLEncoding.EncodeToString([]byte("b:" + mac))
}

// --- server ---

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
