package printer

import (
	"fmt"
	"sync"

	"epos-proxy/internal/logger"
)

type Manager struct {
	mu        sync.RWMutex
	workers   map[string]*printerWorker
	drivers   []Driver
	lanDriver LANDriver
}

func NewManager(drivers ...Driver) *Manager {
	m := &Manager{
		workers: make(map[string]*printerWorker),
	}
	for _, d := range drivers {
		m.RegisterDriver(d)
	}
	return m
}

func (m *Manager) RegisterDriver(d Driver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.drivers = append(m.drivers, d)
	if ld, ok := d.(LANDriver); ok {
		m.lanDriver = ld
	}
}

func (m *Manager) SetLANDriver(d LANDriver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lanDriver = d
}

func (m *Manager) Register(p Printer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.workers[p.ID()]; !exists {
		m.workers[p.ID()] = newPrinterWorker(p)
		logger.Debugf("Registered printer worker for ID: %s", p.ID())
	}
}

func (m *Manager) Unregister(id string) {
	m.mu.Lock()
	w, exists := m.workers[id]
	if exists {
		delete(m.workers, id)
	}
	m.mu.Unlock()

	if exists && w != nil {
		w.Close()
		logger.Debugf("Unregistered and closed printer worker for ID: %s", id)
	}
}

// Remove explicitly unregisters and closes a printer worker by ID.
func (m *Manager) Remove(id string) {
	m.Unregister(id)
}

func (m *Manager) Get(id string) (*printerWorker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if w, ok := m.workers[id]; ok {
		return w, nil
	}

	for _, d := range m.drivers {
		p, err := d.Open(id)
		if err == nil && p != nil {
			w := newPrinterWorker(p)
			m.workers[id] = w
			if p.ID() != id {
				m.workers[p.ID()] = w
			}
			logger.Debugf("Lazily registered printer worker for ID: %s via driver %s", id, d.Name())
			return w, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
}

func (m *Manager) Print(printerID string, data []byte) error {
	w, err := m.Get(printerID)
	if err != nil {
		return err
	}

	reply := make(chan JobResult, 1)
	err = w.Enqueue(func(p Printer) JobResult {
		logger.Debugf("Executing print job for printer %s", printerID)
		if err := p.Write(data); err != nil {
			return JobResult{Err: fmt.Errorf("print job failed for printer %s: %w", printerID, err)}
		}
		logger.Debugf("Print job completed for printer %s", printerID)
		return JobResult{OK: true}
	}, reply)
	if err != nil {
		return fmt.Errorf("failed to enqueue print job for printer %s: %w", printerID, err)
	}

	res := <-reply
	if res.Err != nil {
		m.Unregister(printerID)
	}
	return res.Err
}

func (m *Manager) WriteAsync(printerID string, data []byte) (<-chan JobResult, error) {
	w, err := m.Get(printerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get printer for ID %s: %w", printerID, err)
	}

	reply := make(chan JobResult, 1)
	err = w.Enqueue(func(p Printer) JobResult {
		logger.Debugf("Executing print job for printer %s", printerID)
		if err := p.Write(data); err != nil {
			return JobResult{Err: fmt.Errorf("print job failed for printer %s: %w", printerID, err)}
		}
		logger.Debugf("Print job completed for printer %s", printerID)
		return JobResult{OK: true}
	}, reply)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue print job for printer %s: %w", printerID, err)
	}

	return reply, nil
}

func (m *Manager) Discover() DiscoveryResult {
	m.mu.RLock()
	drivers := make([]Driver, len(m.drivers))
	copy(drivers, m.drivers)
	m.mu.RUnlock()

	available := make([]Device, 0)
	var scanErrs []string

	for _, d := range drivers {
		devs, err := d.Discover()
		if err != nil {
			scanErrs = append(scanErrs, err.Error())
			logger.Errorf("Driver %s discovery failed: %v", d.Name(), err)
		}
		available = append(available, devs...)
	}

	var scanErr string
	if len(scanErrs) > 0 {
		scanErr = scanErrs[0]
	}

	return DiscoveryResult{
		Printers: available,
		ErrorMsg: scanErr,
	}
}

func (m *Manager) DiscoverAllPrinters() DiscoveryResult {
	return m.Discover()
}

func (m *Manager) AddLANPrinter(ip string) error {
	m.mu.RLock()
	ld := m.lanDriver
	m.mu.RUnlock()

	if ld == nil {
		return fmt.Errorf("no LAN driver registered")
	}

	p, err := ld.Add(ip)
	if err != nil {
		return err
	}

	m.Register(p)
	return nil
}

func (m *Manager) RemoveLANPrinter(ip string) error {
	m.mu.RLock()
	ld := m.lanDriver
	m.mu.RUnlock()

	if ld == nil {
		return fmt.Errorf("no LAN driver registered")
	}

	id, err := ld.Remove(ip)
	if err != nil {
		return err
	}

	m.Unregister(id)
	return nil
}

func (m *Manager) CheckLANPrinterStatus(ip string) bool {
	m.mu.RLock()
	ld := m.lanDriver
	m.mu.RUnlock()

	if ld == nil {
		return false
	}
	return ld.CheckLANPrinter(ip) == nil
}

// Close terminates all printer workers and releases associated resources.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, w := range m.workers {
		w.Close()
	}
}
