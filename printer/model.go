package printer

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type PrinterConnectionType int

const (
	PrinterTypeUSB PrinterConnectionType = iota
	PrinterTypeLAN
	PrinterTypeBluetooth
)

const (
	QueueSize    = 100
	WriteTimeout = 5 * time.Second
	ChunkSize    = 8 * 1024 // 8 KB
)

var ErrNotFound = errors.New("printer not found")
var ErrQueueFull = errors.New("printer queue is full")

type JobResult struct {
	OK  bool
	Err error
}

type JobFunc func(p *Printer) JobResult

type Job struct {
	run   JobFunc
	reply chan JobResult
}

type Printer struct {
	connectionType PrinterConnectionType
	id             *PrinterID
	lanIP          string
	mu             sync.Mutex
	// USB fields
	usbCtx      usbContext
	device      usbDevice
	config      usbConfig
	iFace       usbInterface
	outEndpoint usbOutEndpoint
	// LAN fields
	tcpConn net.Conn
	jobs    chan Job
	// Bluetooth fields
	bluetoothMAC string
	btChannel    int    // 0 = auto-discover
	btDevPath    string // cached /dev/rfcommX path (Linux only; empty on other platforms)
	btConn       net.Conn
}

type Info struct {
	Id   string
	Name string
}

type UnavailableInfo struct {
	Name  string
	Error string
}

type EndpointInfo struct {
	config           int
	iFace            int
	alternateSetting int
	outEndpoint      int
}

type PrinterID struct {
	Serial string
	VidPid string
	Path   string
}

type Printers struct {
	Available   []Info
	Unavailable []UnavailableInfo
}

type DeviceID map[string]string

type LibUsbPrinter struct {
	Serial   string
	Path     string
	Name     string
	VidPid   string
	DeviceId DeviceID
}

func (p LibUsbPrinter) String() string {
	return fmt.Sprintf(
		"Printer{Name:%q, Serial:%q, VIDPID:%s, Path:%q, DeviceId: %v}",
		p.Name,
		p.Serial,
		p.VidPid,
		p.Path,
		p.DeviceId,
	)
}

func (p PrinterID) String() string {
	return fmt.Sprintf(
		"Printer{Serial:%q, VidPid:%s, Path:%q}",
		p.Serial,
		p.VidPid,
		p.Path,
	)
}
