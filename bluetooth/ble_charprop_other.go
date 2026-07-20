//go:build !darwin

package bluetooth

import (
	"epos-proxy/logger"
	"fmt"

	tinygoBT "tinygo.org/x/bluetooth"
)

// discoverPrinterCharacteristic returns the first discovered characteristic.
// On Darwin (cgo), ble_darwin_charprop.go overrides this to prefer characteristics
// with the Write or WriteWithoutResponse property.
func discoverPrinterCharacteristic(service tinygoBT.DeviceService) (*tinygoBT.DeviceCharacteristic, error) {
	chars, err := service.DiscoverCharacteristics(nil)
	if err != nil {
		return nil, err
	}
	if len(chars) == 0 {
		return nil, fmt.Errorf("BT/ble: service %s exposes no characteristics", service.UUID())
	}
	for _, c := range chars {
		logger.Debugf("BT/ble: characteristic %s", c.UUID())
	}
	return &chars[0], nil
}
