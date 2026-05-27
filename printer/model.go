package printer

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/google/gousb"
)

type PrinterConnectionType int

const (
	PrinterTypeUSB PrinterConnectionType = iota
	PrinterTypeLAN
)

const (
	QueueSize    = 100
	WriteTimeout = 30 * time.Second
	ChunkSize    = 4096
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
	Width          int
	// USB fields
	usbCtx      *gousb.Context
	device      *gousb.Device
	config      *gousb.Config
	iFace       *gousb.Interface
	outEndpoint *gousb.OutEndpoint
	// LAN fields
	tcpConn net.Conn
	jobs    chan Job
}

type Info struct {
	Id    string
	Name  string
	IsLAN bool
	IP    string
	Label string
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
	Path   string
}

type Printers struct {
	Available   []Info
	Unavailable []UnavailableInfo
}

type DeviceID map[string]string

type PrinterProtocol int

const (
	ProtocolUnknown PrinterProtocol = iota
	ProtocolESCPOS
	ProtocolTSPL
)

type LibUsbPrinter struct {
	Serial   string
	Path     string
	Name     string
	VidPid   string
	Protocol PrinterProtocol
}

func (p PrinterProtocol) String() string {
	switch p {
	case ProtocolESCPOS:
		return "ESCPOS"
	case ProtocolTSPL:
		return "TSPL"
	default:
		return "UNKNOWN"
	}
}
