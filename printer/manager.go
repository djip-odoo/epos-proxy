package printer

import (
	"epos-proxy/logger"
	"fmt"
	"sync"
)

type Manager struct {
	mu       sync.Mutex
	printers map[string]*Printer
}

func NewManager() *Manager {
	return &Manager{printers: make(map[string]*Printer)}
}

func (m *Manager) Get(id string, is_thermal_printer bool) (*Printer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.printers[id]; ok {
		logger.Debugf("Reusing existing printer instance for ID: %s", id)
		return p, nil
	}

	logger.Debugf("Creating new printer instance for ID: %s", id)
	p := newPrinter(id)
	if is_thermal_printer {
		if err := p.ensureOpen(); err != nil {
			return nil, fmt.Errorf("failed to open new printer instance for ID %s: %w", id, err)
		}
	}

	m.printers[id] = p
	logger.Debugf("Registered new printer instance for ID: %s", id)
	return p, nil
}

func (m *Manager) WriteAsync(printerId string, data []byte, is_thermal_printer bool) (<-chan JobResult, error) {
	p, err := m.Get(printerId, is_thermal_printer)
	if err != nil {
		return nil, fmt.Errorf("failed to get printer for ID %s: %w", printerId, err)
	}

	reply := make(chan JobResult, 1)
	err = p.Enqueue(func(p *Printer) JobResult {
		logger.Debugf("Executing print job for printer %s", printerId)
		if err := p.Write(data, is_thermal_printer); err != nil {
			return JobResult{Err: fmt.Errorf("print job failed for printer %s: %w", printerId, err)}
		}
		logger.Debugf("Print job completed for printer %s", printerId)
		return JobResult{OK: true}
	}, reply)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue print job for printer %s: %w", printerId, err)
	}

	return reply, nil
}

func (m *Manager) ListPrinters(port int) ([]PrinterJson, error) {
	printers := make([]PrinterJson, 0)

	printerInfos, err := ListUSBPrinters()

	if err == nil {

		logger.Debugf("Detected %d available USB printers", len(printerInfos.Available))

		for _, info := range printerInfos.Available {
			printers = append(printers, PrinterJson{
				Id:       info.Id,
				Name:     info.VendorName + " " + info.ProductName,
				Serial:   info.Serial,
				Ip:       fmt.Sprintf("127.0.0.1:%d/p/%s", port, info.Id),
				CupsName: info.CupsName,
				Online:   true,
			})
		}
	} else {
		logger.Errorf("USB printer detection failed: %v", err)
	}

	return printers, nil
}
