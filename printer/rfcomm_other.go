//go:build darwin

package printer

import (
	"epos-proxy/logger"
	"net"
)

// globalRFCOMMCache is a no-op stub for non-Linux/non-Windows platforms (e.g. macOS).
// On Linux the real implementation is in rfcomm_linux.go.
var globalRFCOMMCache = &rfcommCache{}

// rfcommCache stub for non-Linux builds.
type rfcommCache struct{}

func (c *rfcommCache) get(_ string) (*rfcommBinding, bool) { return nil, false }
func (c *rfcommCache) set(_ string, _ *rfcommBinding)      {}

// rfcommBinding stub for non-Linux builds.
type rfcommBinding struct {
	DevPath string
	Channel int
	Index   int
}

// dialRFCOMMPlatform on macOS / other Unix falls back to the raw RFCOMM socket
// using SDP discovery or the caller's cached channel.
func dialRFCOMMPlatform(mac string, cachedChannel int) (net.Conn, error) {
	logger.Infof("bluetooth other")
	ch := cachedChannel
	if ch <= 0 {
		var err error
		ch, err = sdpDiscoverChannel(mac)
		if err != nil || ch <= 0 {
			ch = 1
		}
	}
	return dialRFCOMM(mac, ch)
}
