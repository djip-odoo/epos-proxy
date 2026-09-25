package bluetooth

import (
	"context"
	"epos-proxy/internal/config"
	"epos-proxy/internal/logger"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// btConnectTimeout is the maximum time allowed for a single RFCOMM connect attempt.
const btConnectTimeout = 3 * time.Second

type BluetoothPrinterInfo struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Device  string `json:"device"`
}

type DependencyStatus struct {
	Name        string `json:"name"`
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

func (bm *BluetoothManager) getCachedRFCOMMChannel(address string) int {
	address = normalizeAddress(address)
	if b, ok := bm.cache.get(address); ok {
		return b.Channel
	}
	return 0
}

// checkBluetoothPrinter verifies connectivity to a Bluetooth printer at address.
// When isAllowlisted is true, it verifies that the address is in the saved printer list
// before dialing (used for UI status checks). When false, it dials directly without
// allowlist verification (used when testing a new device during AddBluetoothPrinter).
func (bm *BluetoothManager) checkBluetoothPrinter(address string, isAllowlisted bool) error {
	if !bm.isAdapterActive() {
		return fmt.Errorf("BT/manager: Bluetooth adapter is not available")
	}

	dial := bm.Dial
	if isAllowlisted {
		dial = bm.DialAllowlisted
	}

	conn, err := dial(address)
	if err != nil {
		logger.Errorf("BT/manager: Bluetooth printer %s is unreachable: %v", address, err)
		return fmt.Errorf("bluetooth printer %s is unreachable: %w", address, err)
	}

	_ = conn.Close()
	return nil
}

func (bm *BluetoothManager) AddBluetoothPrinter(address, name string) error {
	logger.Debugf("BT/manager: adding Bluetooth printer: %s (%s)", address, name)
	address = normalizeAddress(address)
	if err := validateAddress(address); err != nil {
		logger.Errorf("BT/manager: invalid Bluetooth address %s: %v", address, err)
		return err
	}

	if err := bm.checkBluetoothPrinter(address, false); err != nil {
		logger.Errorf("BT/manager: check printer failed for %s: %v", address, err)
		return err
	}

	if bm.Cfg == nil {
		return fmt.Errorf("BT/manager: config manager is not initialized")
	}

	if err := bm.Cfg.AddBluetoothPrinter(address, name); err != nil {
		logger.Errorf("BT/manager: failed to save Bluetooth printer: %v", err)
		return fmt.Errorf("failed to save Bluetooth printer: %w", err)
	}

	logger.Debugf("BT/manager: Bluetooth printer added: %s (%s)", address, name)
	return nil
}

func (bm *BluetoothManager) RemoveBluetoothPrinter(address string) error {
	logger.Debugf("BT/manager: removing Bluetooth printer: %s", address)
	address = normalizeAddress(address)
	bm.deleteBinding(address)

	if bm.Cfg == nil {
		return fmt.Errorf("BT/manager: config manager is not initialized")
	}

	if err := bm.Cfg.RemoveBluetoothPrinter(address); err != nil {
		logger.Errorf("BT/manager: failed to remove Bluetooth printer: %v", err)
		return fmt.Errorf("failed to remove Bluetooth printer: %w", err)
	}

	logger.Debugf("BT/manager: Bluetooth printer removed successfully: %s", address)
	return nil
}

func (bm *BluetoothManager) IsPrinterOnline(address string) bool {
	logger.Debugf("BT/manager: checking Bluetooth printer status: %s", address)
	if err := bm.checkBluetoothPrinter(address, true); err != nil {
		logger.Errorf("BT/manager: printer %s check failed: %v", address, err)
		return false
	}
	return true
}

func (bm *BluetoothManager) Scan() ([]BluetoothPrinterInfo, error) {
	if bm.isAdapterActive() {
		return scanBluetoothPrinters()
	}

	return nil, fmt.Errorf("Please turn on Bluetooth adapter and try again.")
}

func (bm *BluetoothManager) isAdapterActive() bool {
	for _, t := range supportedTransportsByOS() {
		if t.isAvailable() {
			return true
		}
	}
	return false
}

func (bm *BluetoothManager) CheckDependencies() []DependencyStatus {
	return checkDependencies()
}

func (bm *BluetoothManager) GetCachedBinding(address string) (string, int, bool) {
	address = normalizeAddress(address)
	if b, ok := bm.cache.get(address); ok {
		return b.DevPath, b.Channel, true
	}
	return "", 0, false
}

// bleTransportSingleton is the one shared instance of bleTransport. It must be a
// singleton because bleTransport embeds a sync.Mutex that serialises concurrent
// adapter.Scan and adapter.Connect calls — if supportedTransportsByOS returned a
// new &bleTransport{} on every call, each call site would get its own fresh,
// uncontended lock and the mutex would protect nothing.
var bleTransportSingleton = &bleTransport{}

func supportedTransportsByOS() []transport {
	if runtime.GOOS == "darwin" {
		return []transport{
			bleTransportSingleton,
		}
	}

	return []transport{
		&classicTransport{},
	}
}

// Dial attempts to connect to the Bluetooth device at address using the platform's
// preferred transports in order, automatically falling back if a connection fails.
// Callers from the print path should use DialAllowlisted instead.
func (bm *BluetoothManager) Dial(address string) (net.Conn, error) {
	var lastErr error
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, t := range supportedTransportsByOS() {
		if !t.isAvailable() {
			logger.Debugf("BT/manager: transport %s is not available, skipping", t.name())
			continue
		}

		logger.Debugf("BT/manager: attempting connection via %s to %s", t.name(), address)
		conn, err := t.dial(ctx, address)
		if err == nil {
			logger.Debugf("BT/manager: connected via %s to %s", t.name(), address)
			return conn, nil
		}
		logger.Warnf("BT/manager: connection via %s to %s failed: %v", t.name(), address, err)
		lastErr = err
	}

	if lastErr == nil {
		return nil, fmt.Errorf("bluetooth/manager: no available Bluetooth transports")
	}
	return nil, fmt.Errorf("bluetooth/manager: all connection strategies failed: %w", lastErr)
}

func (bm *BluetoothManager) DialAllowlisted(address string) (net.Conn, error) {
	address = normalizeAddress(address)
	if bm.Cfg == nil {
		return nil, fmt.Errorf("BT/manager: cannot dial allowlisted address %s: config not initialised", address)
	}

	allowed := false
	for _, p := range bm.Cfg.GetBluetoothPrinters() {
		if normalizeAddress(p.Address) == address {
			allowed = true
			break
		}
	}

	if !allowed {
		return nil, fmt.Errorf("bluetooth printer %s has not been added; add it via the Bluetooth setup before printing", address)
	}
	return bm.Dial(address)
}

func scanBluetoothPrinters() ([]BluetoothPrinterInfo, error) {
	logger.Debug("BT/manager: starting Bluetooth printer scan across all available transports")

	var allDevices []BluetoothPrinterInfo
	seen := make(map[string]bool)
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, t := range supportedTransportsByOS() {
		if !t.isAvailable() {
			continue
		}
		wg.Add(1)
		go func(trans transport) {
			defer wg.Done()
			devices, err := trans.scan(ctx)
			if err != nil {
				logger.Warnf("BT/manager: transport %s scan failed: %v", trans.name(), err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, d := range devices {
				norm := normalizeAddress(d.Address)
				if !seen[norm] {
					seen[norm] = true
					d.Address = norm
					allDevices = append(allDevices, d)
				}
			}
		}(t)
	}

	wg.Wait()

	sort.SliceStable(allDevices, func(i, j int) bool {
		if allDevices[i].Device != allDevices[j].Device {
			return strings.ToLower(allDevices[i].Device) > strings.ToLower(allDevices[j].Device)
		}
		return strings.ToLower(allDevices[i].Name) < strings.ToLower(allDevices[j].Name)
	})

	logger.Debugf("BT/manager: scan complete, found %d unique device(s)", len(allDevices))
	return allDevices, nil
}
