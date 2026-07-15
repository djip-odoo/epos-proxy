package bluetooth

import (
	"context"
	"encoding/base64"
	"epos-proxy/config"
	"epos-proxy/logger"
	"epos-proxy/util"
	"fmt"
	"net"
	"strings"
	"time"
)

type BluetoothPrinterInfo struct {
	MAC  string `json:"mac"`
	Name string `json:"name"`
}

type DependencyStatus struct {
	Name        string `json:"name"`
	Installed   bool   `json:"installed"`
	InstallCmd  string `json:"installCmd"`
	Description string `json:"description"`
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
	return BTManager
}

func (bm *BluetoothManager) GetCachedRFCOMMChannel(mac string) int {
	mac = util.NormalizeMAC(mac)
	if b, ok := bm.cache.get(mac); ok {
		return b.Channel
	}
	return 0
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

// Dial attempts to connect to the Bluetooth device at mac using the transport
// that matches the printer's configured type (defaulting to Classic).
// It does NOT fall back to other transports.
func (bm *BluetoothManager) Dial(mac string) (net.Conn, error) {
	mac = util.NormalizeMAC(mac)

	// Fetch printer type from config
	connType := "classic"
	if bm.Cfg != nil {
		for _, p := range bm.Cfg.GetBluetoothPrinters() {
			if util.NormalizeMAC(p.MAC) == mac {
				if p.Type != "" {
					connType = strings.ToLower(p.Type)
				}
				break
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, t := range preferredTransports() {
		if strings.ToLower(t.Name()) == connType {
			if !t.IsAvailable() {
				return nil, fmt.Errorf("bluetooth/manager: transport %s is not active/available", t.Name())
			}
			logger.Infof("BT/manager: dialing via %s to %s", t.Name(), mac)
			return t.Dial(ctx, mac)
		}
	}

	return nil, fmt.Errorf("bluetooth/manager: no transport found for connection type %q", connType)
}

// ScanBluetoothPrinters queries the transport corresponding to the chosen connection type.
func ScanBluetoothPrinters(connType string) ([]BluetoothPrinterInfo, error) {
	connType = strings.ToLower(strings.TrimSpace(connType))
	if connType == "" {
		connType = "classic"
	}

	logger.Debugf("BT/manager: starting Bluetooth printer scan for connection type: %s", connType)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for _, t := range preferredTransports() {
		if strings.ToLower(t.Name()) == connType {
			if !t.IsAvailable() {
				return nil, fmt.Errorf("bluetooth/manager: transport %s is not active/available", t.Name())
			}
			return t.Scan(ctx)
		}
	}

	return nil, fmt.Errorf("bluetooth/manager: unknown connection type %q", connType)
}

func IsBluetoothAdapterActive() bool {
	for _, t := range preferredTransports() {
		if t.IsAvailable() {
			return true
		}
	}
	return false
}

func EncodeBluetoothPrinterID(mac string) string {
	return base64.RawURLEncoding.EncodeToString([]byte("b:" + mac))
}

func getTransportNames(transports []Transport) []string {
	names := make([]string, len(transports))
	for i, t := range transports {
		names[i] = t.Name()
	}
	return names
}
