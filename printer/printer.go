package printer

import (
	"fmt"
	"net"
	"time"

	"epos-proxy/logger"
)

func newPrinter(id string) *Printer {
	// Check if this is a LAN printer
	if lanIP, ok := DecodeLANPrinterID(id); ok {
		p := &Printer{
			connectionType: PrinterTypeLAN,
			lanIP:          lanIP,
			jobs:           make(chan Job, QueueSize),
		}
		go p.loop()
		return p
	}

	// USB printer
	var printerID *PrinterID = nil
	if id != "" {
		printerID, _ = decodePrinterID(id)
	}

	p := &Printer{
		connectionType: PrinterTypeUSB,
		id:             printerID,
		jobs:           make(chan Job, QueueSize),
	}

	logger.Debugf("Created new USB printer instance for ID: %s", p.idToString())
	go p.loop()
	return p
}

func (p *Printer) Enqueue(fn JobFunc, reply chan JobResult) error {
	j := Job{run: fn, reply: reply}
	select {
	case p.jobs <- j:
		logger.Debugf("Enqueued print job for printer %s", p.idToString())
		return nil
	default:
		logger.Warnf("Printer queue full for printer %s", p.idToString())
		return ErrQueueFull
	}
}

func (p *Printer) Write(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureOpen(); err != nil {
		return err
	}

	logger.Debugf("Writing %d bytes to printer %s", len(data), p.idToString())

	if p.connectionType == PrinterTypeLAN {
		if err := p.tcpConn.SetWriteDeadline(time.Now().Add(WriteTimeout)); err != nil {
			p.closeDeviceLocked()
			return fmt.Errorf("failed to set write deadline for LAN printer %s: %w", p.idToString(), err)
		}
		if _, err := p.tcpConn.Write(data); err != nil {
			p.closeDeviceLocked()
			return fmt.Errorf("failed to write to LAN printer %s: %w", p.idToString(), err)
		}
		logger.Debugf("Successfully wrote to LAN printer %s", p.idToString())
		return nil
	}

	return p.writeUSB(data)
}

func (p *Printer) loop() {
	logger.Debugf("Printer loop started for %s with %d jobs", p.idToString(), len(p.jobs))
	for j := range p.jobs {
		result := j.run(p)
		if j.reply != nil {
			j.reply <- result
			close(j.reply)
		}
		if len(p.jobs) == 0 {
			p.close()
		}
	}
}

func (p *Printer) ensureOpen() error {
	if p.connectionType == PrinterTypeLAN {
		return p.ensureOpenLANLocked()
	}
	return p.ensureOpenUSBLocked()
}

func (p *Printer) ensureOpenLANLocked() error {
	if p.tcpConn != nil {
		logger.Debugf("LAN printer %s already connected", p.idToString())
		return nil // already connected
	}

	addr := fmt.Sprintf("%s:%d", p.lanIP, LANPort)
	logger.Debugf("Attempting to connect to LAN printer %s at %s", p.idToString(), addr)
	conn, err := net.DialTimeout("tcp", addr, LANConnectTimeout)
	if err != nil {
		logger.Errorf("Failed to connect to LAN printer %s at %s: %v", p.idToString(), addr, err)
		return fmt.Errorf("failed to connect to LAN printer at %s: %w", addr, err)
	}

	p.tcpConn = conn
	return nil
}

func (p *Printer) close() {
	p.mu.Lock()
	logger.Debugf("Closing printer %s", p.idToString())
	defer p.mu.Unlock()
	p.closeDeviceLocked()
}

func (p *Printer) closeDeviceLocked() {
	if p.connectionType == PrinterTypeLAN {
		if p.tcpConn != nil {
			_ = p.tcpConn.Close()
			p.tcpConn = nil
			logger.Debugf("LAN printer %s connection closed", p.idToString())
		}
		return
	}

	p.closeUSBDeviceLocked()
}

func (p *Printer) idToString() string {
	if p.connectionType == PrinterTypeLAN {
		return fmt.Sprintf("LAN:%s", p.lanIP)
	}
	if p.id != nil {
		return fmt.Sprintf("USB:%s, %v", p.id.Serial, p.id)
	}
	return "USB:unknown"
}
