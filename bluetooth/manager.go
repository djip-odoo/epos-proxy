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
	Address string `json:"address"`
	Name    string `json:"name"`
	Device  string `json:"device"`
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

func InitBluetoothManager(cfg *config.Manager) {
	BTManager = &BluetoothManager{
		Cfg: cfg,
		cache: &rfcommCache{
			entries: make(map[string]*rfcommBinding),
		},
	}
}

func (bm *BluetoothManager) GetCachedRFCOMMChannel(address string) int {
	address = util.NormalizeAddress(address)
	if b, ok := bm.cache.get(address); ok {
		return b.Channel
	}
	return 0
}

func (bm *BluetoothManager) CheckBluetoothPrinter(address string) error {
	conn, err := bm.Dial(address)
	if err != nil {
		return fmt.Errorf("bluetooth printer %s is unreachable: %w", address, err)
	}
	_ = conn.Close()
	return nil
}

func (bm *BluetoothManager) GetCachedBinding(address string) (string, int, bool) {
	address = util.NormalizeAddress(address)
	if b, ok := bm.cache.get(address); ok {
		return b.DevPath, b.Channel, true
	}
	return "", 0, false
}

func supportedTransportsByOS() []Transport {
	return []Transport{
		&ClassicTransport{},
		&BLETransport{},
	}
}

// Dial attempts to connect to the Bluetooth device at mac using the transport
// that matches the printer's configured type (defaulting to Classic).
// It does NOT fall back to other transports.
func (bm *BluetoothManager) Dial(address string) (net.Conn, error) {
	address = util.NormalizeAddress(address)

	// Fetch printer type from config
	connType := "classic"
	if bm.Cfg != nil {
		for _, p := range bm.Cfg.GetBluetoothPrinters() {
			if util.NormalizeAddress(p.Address) == address {
				if p.Type != "" {
					connType = strings.ToLower(p.Type)
				}
				break
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, t := range supportedTransportsByOS() {
		if strings.ToLower(t.Name()) == connType {
			if !t.IsAvailable() {
				return nil, fmt.Errorf("bluetooth/manager: transport %s is not active/available", t.Name())
			}
			logger.Infof("BT/manager: dialing via %s to %s", t.Name(), address)
			return t.Dial(ctx, address)
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

	for _, t := range supportedTransportsByOS() {
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
	for _, t := range supportedTransportsByOS() {
		if t.IsAvailable() {
			return true
		}
	}
	return false
}

func EncodeBluetoothPrinterID(address string) string {
	return base64.RawURLEncoding.EncodeToString([]byte("b:" + address))
}

func getTransportNames(transports []Transport) []string {
	names := make([]string, len(transports))
	for i, t := range transports {
		names[i] = t.Name()
	}
	return names
}
