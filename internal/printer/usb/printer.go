package usb

import (
	"context"
	"fmt"
	"sync"

	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"

	"github.com/google/gousb"
)

// UsbPrinter represents a USB-connected printer accessed via gousb/libusb.
type UsbPrinter struct {
	id          *ID
	idStr       string
	mu          sync.Mutex
	usbCtx      *gousb.Context
	device      *gousb.Device
	config      *gousb.Config
	iFace       *gousb.Interface
	outEndpoint *gousb.OutEndpoint
}

var _ printer.Printer = (*UsbPrinter)(nil)

// New creates a UsbPrinter for the given encoded printer ID.
// If idStr is empty, it will open the first available USB printer upon connection.
func New(idStr string) *UsbPrinter {
	var pid *ID
	if idStr != "" {
		pid, _ = decodeID(idStr)
	}
	return &UsbPrinter{
		id:    pid,
		idStr: idStr,
	}
}

// NewUsbPrinter is a backward-compatible constructor.
func NewUsbPrinter(idStr string) *UsbPrinter {
	return New(idStr)
}

// ID returns the printer's encoded ID string.
func (p *UsbPrinter) ID() string {
	return p.idStr
}

// Write writes the data payload to the USB bulk OUT endpoint in chunks.
func (p *UsbPrinter) Write(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureOpenLocked(); err != nil {
		return err
	}

	for len(data) > 0 {
		size := min(len(data), printer.ChunkSize)
		logger.Debugf("USB printer %s writing %d bytes", p.idToString(), size)

		ctx, cancel := context.WithTimeout(context.Background(), printer.WriteTimeout)
		_, err := p.outEndpoint.WriteContext(ctx, data[:size])
		cancel()

		if err != nil {
			p.closeLocked()
			return fmt.Errorf("failed to write %d bytes to USB printer %s: %w", size, p.idToString(), err)
		}

		data = data[size:]
	}
	return nil
}

// Close closes the USB device and context.
func (p *UsbPrinter) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *UsbPrinter) closeLocked() {
	if p.device == nil {
		return
	}
	if p.iFace != nil {
		p.iFace.Close()
		p.iFace = nil
	}
	if p.config != nil {
		_ = p.config.Close()
		p.config = nil
	}
	if p.device != nil {
		_ = p.device.Close()
		p.device = nil
	}
	if p.usbCtx != nil {
		_ = p.usbCtx.Close()
		p.usbCtx = nil
	}
	p.outEndpoint = nil
	logger.Debugf("USB printer %s device closed", p.idToString())
}

func (p *UsbPrinter) idToString() string {
	if p.id != nil {
		return fmt.Sprintf("USB:%s, %v", p.id.Serial, p.id)
	}
	if p.idStr != "" {
		return fmt.Sprintf("USB:%s", p.idStr)
	}
	return "USB:auto"
}

func (p *UsbPrinter) ensureOpenLocked() error {
	if p.device != nil {
		logger.Debugf("USB printer %s already connected", p.idToString())
		return nil // already connected
	}

	ctx := gousb.NewContext()

	var (
		eps     []EndpointInfo
		findAny = p.id == nil
	)

	devices, err := openDevices(ctx, func(desc *gousb.DeviceDesc) bool {
		if findAny && len(eps) > 0 {
			return false
		}

		if p.id != nil && p.id.VidPid != "" && p.id.VidPid != fmt.Sprintf("%04X:%04X", uint16(desc.Vendor), uint16(desc.Product)) {
			return false
		}

		ep, ok := findPrinterEndpoint(desc)
		if ok {
			eps = append(eps, ep)
			return true
		}
		return false
	})
	if err != nil {
		logger.Warnf("Some matching devices failed to open: %v", err)
	}
	if len(devices) == 0 {
		_ = ctx.Close()
		if err != nil {
			return err
		}
		return printer.ErrNotFound
	}

	var (
		target   *gousb.Device
		targetEP *EndpointInfo
	)
	for i, d := range devices {
		serial, _ := d.SerialNumber()

		match := false
		if findAny {
			match = true
		} else if p.id.Serial != "" {
			match = serial == p.id.Serial
		} else if p.id.Path != "" && p.id.VidPid != "" {
			match = pathToString(d.Desc) == p.id.Path && fmt.Sprintf("%04X:%04X", uint16(d.Desc.Vendor), uint16(d.Desc.Product)) == p.id.VidPid
		}

		if match && target == nil {
			target = d
			ep := eps[i]
			targetEP = &ep
		} else {
			_ = d.Close()
		}
	}
	if target == nil || targetEP == nil {
		_ = ctx.Close()
		return printer.ErrNotFound
	}

	_ = target.SetAutoDetach(true)

	cfg, err := target.Config(targetEP.config)
	if err != nil {
		// Retry without auto-detach.
		_ = target.SetAutoDetach(false)
		cfg, err = target.Config(targetEP.config)
	}
	logger.Debugf("Configuring USB device %s", p.idToString())
	if err != nil {
		_ = target.Close()
		_ = ctx.Close()
		return err
	}

	iFace, err := cfg.Interface(targetEP.iFace, targetEP.alternateSetting)
	if err != nil {
		logger.Errorf("Failed to claim USB interface for printer %s: Error: %v", p.idToString(), err)
		_ = cfg.Close()
		_ = target.Close()
		_ = ctx.Close()
		return err
	}

	ep, err := iFace.OutEndpoint(targetEP.outEndpoint)
	if err != nil {
		logger.Errorf("Failed to get USB out endpoint for printer %s: Error: %v", p.idToString(), err)
		iFace.Close()
		_ = cfg.Close()
		_ = target.Close()
		_ = ctx.Close()
		return err
	}

	p.usbCtx = ctx
	p.device = target
	p.config = cfg
	p.iFace = iFace
	p.outEndpoint = ep
	return nil
}
