package bluetooth

import (
	"sync"

	"epos-proxy/logger"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter
var adapterOnce sync.Once
var adapterEnableErr error

const defaultBLEWriteChunk = 180

func enableAdapter() error {
	adapterOnce.Do(func() {
		adapterEnableErr = adapter.Enable()
		if adapterEnableErr != nil {
			logger.Errorf("BT/ble: failed to enable bluetooth adapter: %v", adapterEnableErr)
		} else {
			logger.Debugf("BT/ble: bluetooth adapter enabled successfully")
		}
	})
	return adapterEnableErr
}
