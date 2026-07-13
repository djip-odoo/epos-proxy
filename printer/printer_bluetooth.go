package printer

import (
	"epos-proxy/config"
	"epos-proxy/logger"
	"fmt"
)

func newBlueToothPrinter(mac string) *Printer {
	cfg := config.Get()
	channel := cfg.GetBluetoothPrinterChannel(mac)

	p := &Printer{
		connectionType: PrinterTypeBluetooth,
		bluetoothMAC:   mac,
		btChannel:      channel,
		jobs:           make(chan Job, QueueSize),
	}
	logger.Debugf("Created new Bluetooth printer instance for MAC: %s (saved channel: %d)", mac, channel)
	go p.loop()
	return p
}

func (p *Printer) ensureOpenBluetoothLocked() error {
	cfg := config.Get()
	if p.btConn != nil {
		logger.Debugf("BT printer %s already connected", p.idToString())
		return nil
	}

	conn, err := dialRFCOMMPlatform(p.bluetoothMAC, p.btChannel)
	if err != nil {
		return fmt.Errorf("failed to connect to BT printer %s: %w", p.bluetoothMAC, err)
	}

	p.btConn = conn

	// Capture device path and channel from the cache if it was updated by dialRFCOMMPlatform.
	if b, ok := globalRFCOMMCache.get(p.bluetoothMAC); ok {
		if p.btChannel != b.Channel {
			p.btChannel = b.Channel
			cfg.UpdateBluetoothChannel(p.bluetoothMAC, b.Channel)
		}
		p.btDevPath = b.DevPath
		logger.Infof("BT printer %s connected via %s (channel %d)",
			p.bluetoothMAC, b.DevPath, b.Channel)
	} else {
		logger.Infof("BT printer %s connected (raw socket)", p.bluetoothMAC)
	}

	return nil
}
