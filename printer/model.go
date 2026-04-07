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

type PrinterCategory int

const (
	PrinterThermal PrinterCategory = iota
	PrinterOffice
)

const (
	QueueSize    = 100
	WriteTimeout = 5 * time.Second
)

var ErrNotFound = errors.New("printer not found")
var ErrQueueFull = errors.New("printer queue is full")

type PrinterID struct {
	Serial   string
	Path     string
	CupsName string
}

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
	category       PrinterCategory
	id             *PrinterID
	lanIP          string
	mu             sync.Mutex
	// USB fields
	usbCtx      *gousb.Context
	device      *gousb.Device
	config      *gousb.Config
	iFace       *gousb.Interface
	outEndpoint *gousb.OutEndpoint
	// LAN fields
	tcpConn net.Conn
	jobs    chan Job
	// For office printer
	cupsName string
}

type PrinterType string

const (
	TypeEPOS    PrinterType = "EPOS"
	TypePDF     PrinterType = "PDF"
	TypeUNKNOWN PrinterType = "UNKNOWN"
)

type SystemUsbPrinter struct {
	Serial     string
	IdName     string
	DeviceID   string
	Status     bool
	Name       string
	Type       PrinterType
	CupsUri    string // linux
	DriverName string // win
}

type LibUsbPrinter struct {
	Serial string
	Path   string
	Name   string
	Type   PrinterType
	VidPid string
}

type Info struct {
	Id   string
	Name string
	Type PrinterType
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

type Printers struct {
	Available   []Info
	Unavailable []UnavailableInfo
}
