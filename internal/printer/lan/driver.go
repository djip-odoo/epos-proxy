package lan

import (
	"fmt"
	"net"
	"strings"

	"epos-proxy/internal/config"
	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"
)

// Driver implements printer.Driver and printer.LANDriver for network printers.
type Driver struct {
	cfg *config.Manager
}

var _ printer.LANDriver = (*Driver)(nil)

func init() {
	printer.RegisterDriver("lan", func(cfg *config.Manager) printer.Driver {
		return NewDriver(cfg)
	})
}

func NewDriver(cfg *config.Manager) *Driver {
	return &Driver{cfg: cfg}
}

func (d *Driver) Name() string {
	return "LAN"
}

func (d *Driver) Discover() ([]printer.Device, error) {
	if d.cfg == nil {
		return nil, nil
	}

	ips := d.cfg.GetLANPrinters()
	logger.Debugf("LAN Driver discovering %d configured printers", len(ips))

	devices := make([]printer.Device, 0, len(ips))

	for _, ip := range ips {
		devices = append(devices, printer.Device{
			Ip:         ip,
			Identifier: encodeID(ip),
			Name:       fmt.Sprintf("Network - %s", ip),
			Type:       string(printer.TypeReceipt),
			IsLAN:      true,
			LANIp:      ip,
			Online:     true,
		})
	}

	return devices, nil
}

func (d *Driver) Open(id string) (printer.Printer, error) {
	if ip, ok := decodeID(id); ok {
		return New(ip), nil
	}
	if cleanIP, err := validateIPAddress(id); err == nil {
		return New(cleanIP), nil
	}
	return nil, printer.ErrNotFound
}

func (d *Driver) Add(ip string) (printer.Printer, error) {
	cleanIP, err := validateIPAddress(ip)
	if err != nil {
		return nil, err
	}

	if err := d.CheckLANPrinter(cleanIP); err != nil {
		return nil, fmt.Errorf("printer unreachable: %w", err)
	}

	if d.cfg != nil {
		ips := d.cfg.GetLANPrinters()
		exists := false
		for _, existing := range ips {
			if existing == cleanIP {
				exists = true
				break
			}
		}
		if !exists {
			ips = append(ips, cleanIP)
			if err := d.cfg.SetLANPrinters(ips); err != nil {
				return nil, err
			}
		}
	}

	return New(cleanIP), nil
}

func (d *Driver) Remove(ip string) (string, error) {
	if d.cfg != nil {
		ips := d.cfg.GetLANPrinters()
		newIPs := make([]string, 0, len(ips))
		for _, existing := range ips {
			if existing != ip {
				newIPs = append(newIPs, existing)
			}
		}
		if err := d.cfg.SetLANPrinters(newIPs); err != nil {
			return "", err
		}
	}

	return encodeID(ip), nil
}

func (d *Driver) CheckLANPrinter(ip string) error {
	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", LANPort))
	conn, err := net.DialTimeout("tcp", addr, LANConnectTimeout)
	if err != nil {
		logger.Debugf("LAN printer %s is offline or unreachable: %v", ip, err)
		return err
	}
	_ = conn.Close()
	logger.Debugf("Successfully connected to LAN printer %s", ip)
	return nil
}

func validateIPAddress(ip string) (string, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", fmt.Errorf("IP address cannot be empty")
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		logger.Warnf("Invalid IP address format for input: %s", ip)
		return "", fmt.Errorf("invalid IP address format")
	}

	return ip, nil
}
