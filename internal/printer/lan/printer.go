package lan

import (
	"fmt"
	"net"
	"sync"
	"time"

	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"
)

const (
	LANPort           = 9100
	LANConnectTimeout = 3 * time.Second
)

// LanPrinter represents a network printer reachable over TCP.
type LanPrinter struct {
	ip      string
	id      string
	mu      sync.Mutex
	tcpConn net.Conn
}

// Ensure LanPrinter implements printer.Printer interface.
var _ printer.Printer = (*LanPrinter)(nil)

// New creates a new LanPrinter for the given IP address.
func New(ip string) *LanPrinter {
	return &LanPrinter{
		ip: ip,
		id: encodeID(ip),
	}
}

// ID returns the encoded printer ID.
func (p *LanPrinter) ID() string {
	return p.id
}

// IP returns the configured LAN IP address.
func (p *LanPrinter) IP() string {
	return p.ip
}

func (p *LanPrinter) ensureOpenLocked() error {
	if p.tcpConn != nil {
		logger.Debugf("LAN printer %s already connected", p.id)
		return nil
	}

	addr := net.JoinHostPort(p.ip, fmt.Sprintf("%d", LANPort))
	logger.Debugf("Attempting to connect to LAN printer %s at %s", p.id, addr)
	conn, err := net.DialTimeout("tcp", addr, LANConnectTimeout)
	if err != nil {
		logger.Errorf("Failed to connect to LAN printer %s at %s: %v", p.id, addr, err)
		return fmt.Errorf("failed to connect to LAN printer at %s: %w", addr, err)
	}

	p.tcpConn = conn
	return nil
}

func (p *LanPrinter) Write(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureOpenLocked(); err != nil {
		return err
	}

	logger.Debugf("Writing %d bytes to LAN printer %s", len(data), p.id)

	if err := p.tcpConn.SetWriteDeadline(time.Now().Add(printer.WriteTimeout)); err != nil {
		p.closeLocked()
		return fmt.Errorf("failed to set write deadline for LAN printer %s: %w", p.id, err)
	}
	if _, err := p.tcpConn.Write(data); err != nil {
		p.closeLocked()
		return fmt.Errorf("failed to write to LAN printer %s: %w", p.id, err)
	}
	logger.Debugf("Successfully wrote to LAN printer %s", p.id)
	return nil
}

func (p *LanPrinter) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *LanPrinter) closeLocked() {
	if p.tcpConn != nil {
		_ = p.tcpConn.Close()
		p.tcpConn = nil
		logger.Debugf("LAN printer %s connection closed", p.id)
	}
}
