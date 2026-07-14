package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"epos-proxy/config"
	"epos-proxy/logger"
	"epos-proxy/printer"
	"epos-proxy/server"
	"epos-proxy/util"

	autostart "github.com/emersion/go-autostart"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx            context.Context
	webserver      *server.Server
	config         *config.Manager
	printerManager *printer.Manager
	autoStart      *autostart.App
}

func NewApp(cfg *config.Manager) *App {
	a := &App{}

	if cfg != nil {
		a.config = cfg
	}

	a.autoStart = &autostart.App{
		Name:        "epos-proxy",
		DisplayName: "ePOS Proxy",
		Exec:        []string{os.Args[0]},
	}
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logger.Debugf("Application startup")

	logger.Debugf("Config loaded from %s", a.config.Path())

	a.printerManager = printer.NewManager()

	port, err := a.config.ResolvePort()
	if err != nil {
		logger.Warn("Unable to resolve port, using default")
	}

	a.webserver = server.New(port, a.printerManager)
}

func (a *App) shutdown(ctx context.Context) {
	logger.Infof("Stopping proxy server")

	if err := a.webserver.Stop(); err != nil {
		logger.Errorf("Server stop error: %v", err)
	}
}

type Printer struct {
	Name   string `json:"name"`
	Ip     string `json:"ip"`
	Id     string `json:"id"`
	IsLAN  bool   `json:"isLAN"`
	LANIp  string `json:"lanIp,omitempty"`
	IsBT   bool   `json:"isBT"`
	BTMac  string `json:"btMac,omitempty"`
	Online bool   `json:"online"`
}

type UnavailablePrinter struct {
	Name     string `json:"name"`
	ErrorMsg string `json:"errorMsg"`
	IsLAN    bool   `json:"isLAN"`
	LANIp    string `json:"lanIp,omitempty"`
}

type Status struct {
	ServerRunning       bool                 `json:"serverRunning"`
	DefaultIp           string               `json:"defaultIp"`
	ErrorMsg            string               `json:"errorMsg"`
	Printers            []Printer            `json:"printers"`
	UnavailablePrinters []UnavailablePrinter `json:"unavailablePrinters"`
	Os                  string               `json:"os"`
}

func (a *App) GetPrinterIp(id string) string {
	ip := fmt.Sprintf("127.0.0.1:%d/p/%s", a.webserver.Port, id)
	logger.Debugf("Generated printer endpoint: %s", ip)
	return ip
}

func (a *App) Status() Status {

	logger.Debug("Collecting printer status")

	printers := make([]Printer, 0)
	unavailablePrinters := make([]UnavailablePrinter, 0)

	printerInfos, err := printer.ListUSBPrinters()
	errorMsg := ""

	if err == nil {

		logger.Debugf("Detected %d available USB printers", len(printerInfos.Available))

		for _, info := range printerInfos.Available {
			printers = append(printers, Printer{
				Id:     info.Id,
				Name:   info.Name,
				Ip:     a.GetPrinterIp(info.Id),
				Online: true,
			})
		}

		for _, info := range printerInfos.Unavailable {
			unavailablePrinters = append(unavailablePrinters, UnavailablePrinter{
				Name:     info.Name,
				ErrorMsg: info.Error,
			})

			logger.Warnf("USB printer unavailable: %s (%s)", info.Name, info.Error)
		}
	} else {
		errorMsg = err.Error()
		logger.Errorf("USB printer detection failed: %v", err)
	}

	lanPrinters := printer.ListLANPrinters(a.config)
	for _, info := range lanPrinters {
		printers = append(printers, Printer{
			Id:    info.Id,
			Name:  fmt.Sprintf("Network - %s", info.IP),
			Ip:    a.GetPrinterIp(info.Id),
			IsLAN: true,
			LANIp: info.IP,
		})
	}

	// Bluetooth printers from config
	btPrinters := a.config.GetBluetoothPrinters()
	for _, btCfg := range btPrinters {
		id := printer.EncodeBluetoothPrinterID(btCfg.MAC)
		name := btCfg.Name
		if name == "" {
			name = fmt.Sprintf("Bluetooth - %s", btCfg.MAC)
		}
		printers = append(printers, Printer{
			Id:    id,
			Name:  name,
			Ip:    a.GetPrinterIp(id),
			IsBT:  true,
			BTMac: btCfg.MAC,
		})
	}

	return Status{
		ServerRunning:       a.webserver.Running(),
		DefaultIp:           fmt.Sprintf("127.0.0.1:%d", a.webserver.Port),
		Printers:            printers,
		UnavailablePrinters: unavailablePrinters,
		ErrorMsg:            errorMsg,
		Os:                  runtime.GOOS,
	}
}

func (a *App) AddLANPrinter(ip string) error {

	logger.Debugf("Adding LAN printer: %s", ip)

	ip, err := printer.ValidateIPAddress(ip)
	if err != nil {
		return fmt.Errorf("invalid IP address: %s, error: %v", ip, err)
	}

	if err := printer.CheckLANPrinter(ip); err != nil {
		return fmt.Errorf("LAN printer unreachable: %s, error: %v", ip, err)
	}

	if err := a.config.AddLanEposPrinter(ip); err != nil {
		return fmt.Errorf("failed to save LAN printer: %s, error: %v", ip, err)
	}

	logger.Debugf("LAN printer added successfully: %s", ip)

	return nil
}

func (a *App) ConfirmRemoveLANPrinter(ip string) (bool, error) {

	logger.Debugf("Remove LAN printer requested: %s", ip)

	result, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Remove Printer",
		Message:       fmt.Sprintf("Are you sure you want to remove the printer at %s?", ip),
		Buttons:       []string{"Cancel", "Confirm"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return false, fmt.Errorf("failed to show confirmation dialog: %w", err)
	}
	if result == "Confirm" || result == "Yes" {
		if err := a.config.RemoveLANPrinter(ip); err != nil {
			return false, fmt.Errorf("failed to remove LAN printer: %w", err)
		}
		return true, nil
	}
	logger.Infof("Remove LAN printer cancelled, Remove printer dialog result: %s", result)
	return false, nil
}

func (a *App) CheckLANPrinterStatus(ip string) bool {
	logger.Debugf("Checking LAN printer status: %s", ip)
	return printer.CheckLANPrinter(ip) == nil
}

func (a *App) DownloadLogs() {
	logger.Debugf("Download logs requested")
	logDir := logger.LogDirectory()
	configDir := a.config.ConfigDirectory()
	zipName := fmt.Sprintf("epos-proxy-logs-%s.zip",
		time.Now().Format("2006-01-02"))
	logger.Debugf("Creating logs archive: %s", zipName)
	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Save Archive",
		DefaultFilename: zipName,
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Zip Archives (*.zip)",
				Pattern:     "*.zip",
			},
		},
	})
	err = util.ZipPaths(savePath, map[string]string{"logs": logDir, "config": configDir})
	if err != nil {
		logger.Errorf("Log export failed: %v", err)
		wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
			Type:    wailsruntime.ErrorDialog,
			Title:   "Download Logs Failed",
			Message: err.Error(),
		})
		return
	}
	logger.Infof("Logs successfully exported to: %s", savePath)
}

func (a *App) IsAutostartEnabled() bool {
	return a.autoStart.IsEnabled()
}

func (a *App) EnableAutostart() error {
	logger.Info("Enabling autostart")

	if runtime.GOOS == "linux" {
		return util.EnableLinuxAutostart()
	}

	if !a.autoStart.IsEnabled() {
		return a.autoStart.Enable()
	}

	return nil
}

func (a *App) DisableAutostart() error {
	logger.Info("Disabling autostart")

	if a.autoStart.IsEnabled() {
		return a.autoStart.Disable()
	}

	return nil
}

// --- Bluetooth printer methods ---

func (a *App) ScanBluetoothPrinters() ([]printer.BluetoothPrinterInfo, error) {
	logger.Debug("Scanning for Bluetooth devices")
	devices, err := printer.ScanBluetoothPrinters()
	if err != nil {
		logger.Errorf("Bluetooth scan failed: %v", err)
		return nil, err
	}
	return devices, nil
}

func (a *App) AddBluetoothPrinter(mac, name string) error {
	logger.Debugf("Adding Bluetooth printer: %s (%s)", mac, name)
	mac = util.NormalizeMAC(mac)
	if err := util.ValidateMAC(mac); err != nil {
		logger.Errorf("Invalid MAC address: %v", err)
		return err
	}

	if err := printer.CheckBluetoothPrinter(mac, 0); err != nil {
		logger.Errorf("Bluetooth printer unreachable: %v", err)
		return fmt.Errorf("Bluetooth printer unreachable: %w", err)
	}

	channel := 0
	if ch, ok := printer.GetCachedRFCOMMChannel(mac); ok {
		channel = ch
	}

	if err := a.config.AddBluetoothPrinter(mac, name, channel); err != nil {
		logger.Errorf("Failed to save Bluetooth printer: %v", err)
		return fmt.Errorf("failed to save Bluetooth printer: %w", err)
	}

	logger.Debugf("Bluetooth printer added: %s (%s) on channel %d", mac, name, channel)
	return nil
}

func (a *App) ConfirmRemoveBluetoothPrinter(mac string) (bool, error) {
	logger.Debugf("Remove Bluetooth printer requested: %s", mac)

	result, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Remove Printer",
		Message:       fmt.Sprintf("Are you sure you want to remove the Bluetooth printer %s?", mac),
		Buttons:       []string{"Cancel", "Confirm"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		logger.Infof("failed to show confirmation dialog: %v", err)
		return false, fmt.Errorf("failed to show confirmation dialog: %w", err)
	}
	if result == "Confirm" || result == "Yes" {
		if err := a.config.RemoveBluetoothPrinter(mac); err != nil {
			logger.Errorf("Failed to remove Bluetooth printer: %v", err)
			return false, fmt.Errorf("failed to remove Bluetooth printer: %v", err)
		}
		logger.Debugf("Bluetooth printer removed successfully")
		return true, nil
	}
	logger.Debugf("Remove Bluetooth printer cancelled")
	return false, nil
}

func (a *App) CheckBluetoothPrinterStatus(mac string) bool {
	logger.Debugf("Checking Bluetooth printer status: %s", mac)
	channel := a.config.GetBluetoothPrinterChannel(mac)
	if err := printer.CheckBluetoothPrinter(mac, channel); err != nil {
		return false
	}
	if ch, ok := printer.GetCachedRFCOMMChannel(mac); ok {
		if channel != ch {
			logger.Infof("BT: updating config channel for %s from %d to %d", mac, channel, ch)
			_ = a.config.UpdateBluetoothChannel(mac, ch)
		}
	}
	return true
}
