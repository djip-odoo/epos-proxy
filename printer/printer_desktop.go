//go:build !android

package printer

import (
	"context"
	"fmt"

	"epos-proxy/logger"
	"github.com/google/gousb"
)

func (p *Printer) writeUSB(data []byte) error {
	for len(data) > 0 {
		size := min(len(data), ChunkSize)
		logger.Debugf("USB printer %s writing %d bytes", p.idToString(), size)

		ctx, cancel := context.WithTimeout(context.Background(), WriteTimeout)
		_, err := p.outEndpoint.WriteContext(ctx, data[:size])
		cancel()

		if err != nil {
			p.closeDeviceLocked()
			return fmt.Errorf("failed to write %d bytes to USB printer %s: %w", size, p.idToString(), err)
		}

		data = data[size:]
	}
	return nil
}

func (p *Printer) ensureOpenUSBLocked() error {
	if p.device != nil {
		logger.Debugf("USB printer %s already connected", p.idToString())
		return nil // already connected
	}

	ctx := gousb.NewContext()

	var (
		eps     []EndpointInfo
		findAny = p.id == nil
	)

	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if findAny && len(eps) > 0 {
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
		_ = ctx.Close()
		return fmt.Errorf("failed to open USB device for printer %s: %w", p.idToString(), err)
	}
	if len(devices) == 0 {
		_ = ctx.Close()
		logger.Warnf("USB printer %s not found", p.idToString())
		return ErrNotFound
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
		return ErrNotFound
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

func (p *Printer) closeUSBDeviceLocked() {
	if p.device == nil {
		return
	}
	p.iFace.Close()
	_ = p.config.Close()
	_ = p.device.Close()
	_ = p.usbCtx.Close()
	p.device = nil
	p.config = nil
	p.iFace = nil
	p.outEndpoint = nil
	p.usbCtx = nil
	logger.Debugf("USB printer %s device closed", p.idToString())
}
