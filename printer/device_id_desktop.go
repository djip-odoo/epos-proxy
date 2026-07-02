//go:build !android

package printer

import (
	"epos-proxy/logger"

	"github.com/google/gousb"
)

func getPrinterDeviceID(dev *gousb.Device) DeviceID {
	buf := make([]byte, 1024)

	for _, cfg := range dev.Desc.Configs {
		for _, iFace := range cfg.Interfaces {
			for _, alt := range iFace.AltSettings {
				if alt.Class != gousb.ClassPrinter && !isKnownPrinter(dev.Desc) {
					continue
				}
				n, err := dev.Control(
					0xA1,
					0x00,
					0x00,
					uint16(iFace.Number),
					buf,
				)
				if err != nil || n < 2 {
					continue
				}

				totalLen := int(buf[0])<<8 | int(buf[1])
				if totalLen <= 2 {
					continue
				}

				strLen := min(totalLen-2, n-2)
				if strLen <= 0 {
					continue
				}

				raw := sanitizeDeviceID(string(buf[2 : 2+strLen]))
				deviceID := parseDeviceID(raw)
				deviceID["RAW"] = raw
				return deviceID
			}
		}
	}
	logger.Warnf("device id not found for device: VID=%s, PID=%s", dev.Desc.Vendor, dev.Desc.Product)
	return DeviceID{}
}
