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

type PrinterJson struct {
	Name     string `json:"name"`
	Serial   string `json:"serial"`
	Ip       string `json:"ip"`
	Id       string `json:"id"`
	IsLAN    bool   `json:"isLAN"`
	LANIp    string `json:"lanIp,omitempty"`
	Online   bool   `json:"online"`
	CupsName string `json:"cupsName,omitempty"`
}

type UnavailablePrinter struct {
	Name     string `json:"name"`
	ErrorMsg string `json:"errorMsg"`
	IsLAN    bool   `json:"isLAN"`
	LANIp    string `json:"lanIp,omitempty"`
}

type Info struct {
	ProductName string
	VendorName  string
	Serial      string
	Id          string
	CupsName    string
	Path        string
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
