package main

import (
	"fmt"
	"io"
	"io/fs"
	"runtime"
	"sync"

	"epos-proxy/config"
	"epos-proxy/escpos"
	"epos-proxy/logger"
	"epos-proxy/printer"
	"epos-proxy/server"
	"epos-proxy/util"

	autostart "github.com/emersion/go-autostart"
)

type PrinterService struct {
	config         *config.Manager
	printerManager *printer.Manager
	autoStart      *autostart.App
	webserver      *server.Server
	webserverMu    sync.Mutex
	assets         fs.FS
}

func NewPrinterService(cfg *config.Manager, pm *printer.Manager, autoStart *autostart.App, assets fs.FS) *PrinterService {
	return &PrinterService{
		config:         cfg,
		printerManager: pm,
		autoStart:      autoStart,
		assets:         assets,
	}
}

func (s *PrinterService) GetPrinterManager() *printer.Manager {
	return s.printerManager
}

func (s *PrinterService) StartServer(port int, host string) {
	s.webserverMu.Lock()
	defer s.webserverMu.Unlock()
	s.webserver = server.New(port, host, s, s.assets)
}

func (s *PrinterService) StopServer() error {
	s.webserverMu.Lock()
	defer s.webserverMu.Unlock()
	if s.webserver != nil {
		return s.webserver.Stop()
	}
	return nil
}

func (s *PrinterService) RestartServer() {
	s.webserverMu.Lock()
	defer s.webserverMu.Unlock()

	if s.webserver != nil {
		if err := s.webserver.Stop(); err != nil {
			logger.Errorf("Failed to stop current server: %v", err)
		}
	}

	host := "127.0.0.1"
	if s.config.Data.LANAccessEnabled {
		host = "0.0.0.0"
	}

	logger.Infof("Restarting server on host %s", host)
	s.webserver = server.New(s.config.Data.Port, host, s, s.assets)
}

func (s *PrinterService) GetPrinterIp(id string) string {
	settings := s.GetLANSettings()
	ip := fmt.Sprintf("%s:%d/p/%s", settings.IP, settings.Port, id)
	logger.Debugf("Generated printer endpoint: %s", ip)
	return ip
}

func (s *PrinterService) Status() (server.Status, error) {
	logger.Debug("Collecting printer status")

	printers := make([]server.Printer, 0)
	unavailablePrinters := make([]server.UnavailablePrinter, 0)

	printerInfos, err := printer.ListUSBPrinters()
	errorMsg := ""

	if err == nil {
		logger.Debugf("Detected %d available USB printers", len(printerInfos.Available))

		for _, info := range printerInfos.Available {
			printers = append(printers, server.Printer{
				Id:     info.Id,
				Name:   info.Name,
				Ip:     s.GetPrinterIp(info.Id),
				Online: true,
			})
		}

		for _, info := range printerInfos.Unavailable {
			unavailablePrinters = append(unavailablePrinters, server.UnavailablePrinter{
				Name:     info.Name,
				ErrorMsg: info.Error,
			})

			logger.Warnf("USB printer unavailable: %s (%s)", info.Name, info.Error)
		}
	} else {
		errorMsg = err.Error()
		logger.Errorf("USB printer detection failed: %v", err)
	}

	lanPrinters := printer.ListLANPrinters(s.config)

	for _, info := range lanPrinters {
		printers = append(printers, server.Printer{
			Id:    info.Id,
			Name:  fmt.Sprintf("Network - %s", info.IP),
			Ip:    s.GetPrinterIp(info.Id),
			IsLAN: true,
			LANIp: info.IP,
		})
	}

	var serverRunning bool
	var defaultIp string
	s.webserverMu.Lock()
	ws := s.webserver
	s.webserverMu.Unlock()

	if ws != nil {
		serverRunning = ws.Running()
		defaultIp = fmt.Sprintf("127.0.0.1:%d", ws.Port)
	}

	return server.Status{
		ServerRunning:       serverRunning,
		DefaultIp:           defaultIp,
		Printers:            printers,
		UnavailablePrinters: unavailablePrinters,
		ErrorMsg:            errorMsg,
		Os:                  runtime.GOOS,
	}, nil
}

func (s *PrinterService) AddLANPrinter(ip string) error {
	logger.Debugf("Adding LAN printer: %s", ip)

	ip, err := printer.ValidateIPAddress(ip)
	if err != nil {
		return fmt.Errorf("invalid IP address: %s, error: %v", ip, err)
	}

	if err := printer.CheckLANPrinter(ip); err != nil {
		return fmt.Errorf("LAN printer unreachable: %s, error: %v", ip, err)
	}

	if err := s.config.AddLanEposPrinter(ip); err != nil {
		return fmt.Errorf("failed to save LAN printer: %s, error: %v", ip, err)
	}

	logger.Debugf("LAN printer added successfully: %s", ip)
	return nil
}

func (s *PrinterService) RemoveLANPrinter(ip string) error {
	logger.Debugf("Removing LAN printer: %s", ip)
	return s.config.RemoveLANPrinter(ip)
}

func (s *PrinterService) CheckLANPrinterStatus(ip string) bool {
	logger.Debugf("Checking LAN printer status: %s", ip)
	return printer.CheckLANPrinter(ip) == nil
}

func (s *PrinterService) GetLANSettings() server.LANSettings {
	ip := "127.0.0.1"
	if s.config.Data.LANAccessEnabled {
		if lan_ip, err := util.GetOutboundIP(); err == nil {
			ip = lan_ip
		} else {
			logger.Errorf("Failed to get local IP: %v", err)
		}
	}

	port := s.config.Data.Port

	return server.LANSettings{
		Enabled: s.config.Data.LANAccessEnabled,
		IP:      ip,
		Port:    port,
	}
}

func (s *PrinterService) EnableLANAccess() error {
	logger.Infof("Enabling LAN Access")
	port := s.config.Data.Port

	err := util.AllowPortThroughFirewall(port)
	if err != nil {
		logger.Errorf("Failed to allow port through firewall: %v", err)
		return fmt.Errorf("firewall error: %v", err)
	}

	s.config.Data.LANAccessEnabled = true
	if err := s.config.Save(); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		return fmt.Errorf("config error: %v", err)
	}

	s.RestartServer()
	return nil
}

func (s *PrinterService) DisableLANAccess() error {
	logger.Infof("Disabling LAN Access")
	port := s.config.Data.Port

	err := util.BlockPortThroughFirewall(port)
	if err != nil {
		logger.Errorf("Failed to block port through firewall: %v", err)
	}

	s.config.Data.LANAccessEnabled = false
	if err := s.config.Save(); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		return fmt.Errorf("config error: %v", err)
	}

	s.RestartServer()
	return nil
}

func (s *PrinterService) IsAutostartEnabled() bool {
	return s.autoStart.IsEnabled()
}

func (s *PrinterService) EnableAutostart() error {
	logger.Info("Enabling autostart")

	if runtime.GOOS == "linux" {
		return util.EnableLinuxAutostart()
	}

	if !s.autoStart.IsEnabled() {
		return s.autoStart.Enable()
	}

	return nil
}

func (s *PrinterService) DisableAutostart() error {
	logger.Info("Disabling autostart")

	if s.autoStart.IsEnabled() {
		return s.autoStart.Disable()
	}

	return nil
}

func (s *PrinterService) GetPrinterSetting(id string) config.PrinterSettingConfig {
	return config.GetPrinterSetting(id)
}

func (s *PrinterService) SetPrinterSetting(id string, width int, bottomPadding int, protocol string) error {
	return config.SetPrinterSetting(id, width, bottomPadding, protocol)
}

func (s *PrinterService) OpenCashDrawer(printerId string) error {
	logger.Infof("Opening cash drawer for printer %s", printerId)
	reply, err := s.printerManager.WriteAsync(printerId, escpos.CmdPulse)
	if err != nil {
		return err
	}
	res := <-reply
	if !res.OK {
		return res.Err
	}
	return nil
}

func (s *PrinterService) ExportLogsToWriter(w io.Writer) error {
	logger.Debugf("Exporting logs to writer")
	logDir := logger.LogDirectory()
	configDir := s.config.ConfigDirectory()
	return util.ZipPathsToWriter(w, map[string]string{"logs": logDir, "config": configDir})
}

func (s *PrinterService) ExportLogsToPath(savePath string) error {
	logger.Debugf("Exporting logs to file: %s", savePath)
	logDir := logger.LogDirectory()
	configDir := s.config.ConfigDirectory()
	return util.ZipPaths(savePath, map[string]string{"logs": logDir, "config": configDir})
}

func (s *PrinterService) GetLocalIPAddress() string {
	ip, _ := util.GetOutboundIP()
	return ip
}

func (s *PrinterService) GetLANPin() string {
	return s.config.Data.LANPin
}

func (s *PrinterService) SetLANPin(pin string) error {
	if len(pin) != 4 {
		return fmt.Errorf("PIN must be exactly 4 digits")
	}
	for _, char := range pin {
		if char < '0' || char > '9' {
			return fmt.Errorf("PIN must contain only digits")
		}
	}

	s.config.Data.LANPin = pin
	if err := s.config.Save(); err != nil {
		logger.Errorf("Failed to save config with PIN: %v", err)
		return err
	}
	return nil
}

// ─── Customer Display WebView ─────────────────────────────────────────────────

func (s *PrinterService) GetCustomerDisplayURLs() []config.CustomerDisplayURL {
	urls, err := config.GetCustomerDisplayURLs()
	if err != nil {
		logger.Errorf("Failed to load customer display URLs: %v", err)
		return []config.CustomerDisplayURL{}
	}
	return urls
}

func (s *PrinterService) GetActiveCustomerDisplayURL() *config.CustomerDisplayURL {
	u, err := config.GetActiveCustomerDisplayURL()
	if err != nil {
		logger.Errorf("Failed to get active customer display URL: %v", err)
		return nil
	}
	return u
}

func (s *PrinterService) AddCustomerDisplayURL(name, rawURL, description string) (config.CustomerDisplayURL, error) {
	logger.Infof("Adding customer display URL: name=%s url=%s", name, rawURL)
	record, err := config.AddCustomerDisplayURL(name, rawURL, description)
	if err != nil {
		logger.Errorf("Failed to add customer display URL: %v", err)
		return config.CustomerDisplayURL{}, err
	}
	logger.Infof("Customer display URL added: id=%s", record.ID)
	return record, nil
}

func (s *PrinterService) UpdateCustomerDisplayURL(id, name, rawURL, description string, enabled bool) error {
	logger.Infof("Updating customer display URL: id=%s enabled=%v", id, enabled)
	if err := config.UpdateCustomerDisplayURL(id, name, rawURL, description, enabled); err != nil {
		logger.Errorf("Failed to update customer display URL %s: %v", id, err)
		return err
	}
	logger.Infof("Customer display URL updated: id=%s", id)
	return nil
}

func (s *PrinterService) SetActiveCustomerDisplayURL(id string) error {
	logger.Infof("Setting active customer display URL: id=%s", id)
	if err := config.SetActiveCustomerDisplayURL(id); err != nil {
		logger.Errorf("Failed to set active customer display URL %s: %v", id, err)
		return err
	}
	logger.Infof("Active customer display URL set: id=%s", id)
	return nil
}

func (s *PrinterService) DisableCustomerDisplayURL(id string) error {
	logger.Infof("Disabling customer display URL: id=%s", id)
	if err := config.DisableCustomerDisplayURL(id); err != nil {
		logger.Errorf("Failed to disable customer display URL %s: %v", id, err)
		return err
	}
	logger.Infof("Customer display URL disabled: id=%s", id)
	return nil
}

func (s *PrinterService) DeleteCustomerDisplayURL(id string) error {
	logger.Infof("Deleting customer display URL: id=%s", id)
	if err := config.DeleteCustomerDisplayURL(id); err != nil {
		logger.Errorf("Failed to delete customer display URL %s: %v", id, err)
		return err
	}
	logger.Infof("Customer display URL deleted: id=%s", id)
	return nil
}

// ValidateAdminPin checks the provided PIN against the configured LANPin.
func (s *PrinterService) ValidateAdminPin(pin string) bool {
	valid := s.config.Data.LANPin == pin
	if valid {
		logger.Infof("Admin PIN validated successfully for customer display recovery")
	} else {
		logger.Warnf("Admin PIN validation failed for customer display recovery")
	}
	return valid
}
