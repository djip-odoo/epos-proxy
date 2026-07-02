//go:build !android

package printer

import "github.com/google/gousb"

type usbContext = *gousb.Context
type usbDevice = *gousb.Device
type usbConfig = *gousb.Config
type usbInterface = *gousb.Interface
type usbOutEndpoint = *gousb.OutEndpoint
