package printer

import (
	"errors"
	"time"
)

const (
	QueueSize    = 100
	WriteTimeout = 5 * time.Second
	ChunkSize    = 8 * 1024 // 8 KB
)

var (
	ErrNotFound  = errors.New("printer not found")
	ErrQueueFull = errors.New("printer queue is full")
)

type Type string

const (
	TypeReceipt Type = "receipt"
	TypeLabel   Type = "label"
)

type Printer interface {
	ID() string
	Write(data []byte) error
	Close()
}

type Driver interface {
	Name() string
	Discover() ([]Device, []UnavailableDevice, error)
	Open(id string) (Printer, error)
}

type LANDriver interface {
	Driver
	Add(ip string) (Printer, error)
	Remove(ip string) (string, error)
	CheckLANPrinter(ip string) error
}

type Device struct {
	Ip         string `json:"ip"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	IsLAN      bool   `json:"isLAN"`
	LANIp      string `json:"lanIp,omitempty"`
	Online     bool   `json:"online"`
}

type UnavailableDevice struct {
	Name     string `json:"name"`
	ErrorMsg string `json:"errorMsg"`
	IsLAN    bool   `json:"isLAN"`
	LANIp    string `json:"lanIp,omitempty"`
}

type DiscoveryResult struct {
	Printers            []Device            `json:"printers"`
	UnavailablePrinters []UnavailableDevice `json:"unavailablePrinters"`
	ErrorMsg            string              `json:"errorMsg"`
}
