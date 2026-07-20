//go:build darwin && cgo

package bluetooth

import (
	"fmt"

	cbgo "github.com/tinygo-org/cbgo"
	tinygoBT "tinygo.org/x/bluetooth"
)

// discoverPrinterCharacteristic overrides the cross-platform fallback in ble.go.
// On Darwin, cbgo exposes CBCharacteristicProperties so we can select the first
// characteristic that has Write or WriteWithoutResponse, which is what BLE
// receipt printers require.
func discoverPrinterCharacteristic(service tinygoBT.DeviceService) (*tinygoBT.DeviceCharacteristic, error) {
	chars, err := service.DiscoverCharacteristics(nil)
	if err != nil {
		return nil, err
	}
	if len(chars) == 0 {
		return nil, fmt.Errorf("BT/ble: service %s exposes no characteristics", service.UUID())
	}

	writeProps := cbgo.CharacteristicPropertyWrite | cbgo.CharacteristicPropertyWriteWithoutResponse

	var fallback *tinygoBT.DeviceCharacteristic
	for i := range chars {
		c := &chars[i]
		// DeviceCharacteristic embeds *deviceCharacteristic which has the
		// cbgo.Characteristic field. Both types are in the same package so we
		// can access the unexported field directly.
		props := c.deviceCharacteristic.characteristic.Properties()
		logger.Debugf("BT/ble: characteristic %s props=0x%x", c.UUID(), int(props))
		if props&writeProps != 0 {
			logger.Debugf("BT/ble: selected writable characteristic %s", c.UUID())
			return c, nil
		}
		if fallback == nil {
			fallback = c
		}
	}

	logger.Debugf("BT/ble: no explicitly writable characteristic, using fallback %s", fallback.UUID())
	return fallback, nil
}
