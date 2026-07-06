package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"epos-proxy/config"
	"epos-proxy/logger"
	"epos-proxy/printer"
	"epos-proxy/server"
	"epos-proxy/util"

	autostart "github.com/emersion/go-autostart"

	"github.com/wailsapp/wails/v3/pkg/application"
)

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

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx
	logger.Debugf("Application startup")
	a.printerManager = printer.NewManager()
	port, err := a.config.ResolvePort()
	if err != nil {
		logger.Warn("Unable to resolve port, using default")
	}

	if err := a.config.CheckPortChange(); err != nil {
		logger.Errorf("Failed to check port change: %v", err)
	}

	a.webserver = server.New(port, a.printerManager)
	return nil
}

func (a *App) domReady(ctx context.Context) {
	logger.Debug("DOM is ready")
	if !a.config.IsFirewallPromptCompleted() {
		if exists, err := util.CheckIfRuleExist(); err == nil && exists {
			logger.Infof("Firewall rule already exists for port %d, skipping prompt")
			if err = a.SkipFirewallPrompt(); err != nil {
				logger.Errorf("Error: %v", err)
			}
		} else {
			logger.Infof("Firewall prompt not completed, triggering frontend event")
			application.Get().Event.Emit("open-firewall-prompt")
		}
	}
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
	ErrorMsg            string               `json:"errorMsg"`
	Printers            []Printer            `json:"printers"`
	UnavailablePrinters []UnavailablePrinter `json:"unavailablePrinters"`
	Os                  string               `json:"os"`
}

func (a *App) GetPrinterIp(id string) string {
	settings := a.GetLANSettings()
	ip := fmt.Sprintf("%s:%d/p/%s", settings.IP, settings.Port, id)
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

	return Status{
		ServerRunning:       a.webserver.Running(),
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
		logger.Errorf("Invalid IP address: %v", err)
		return fmt.Errorf("invalid IP address: %v", err)
	}

	if err := printer.CheckLANPrinter(ip); err != nil {
		logger.Errorf("LAN printer unreachable: %v", err)
		return fmt.Errorf("LAN printer unreachable: %v", err)
	}

	if err := a.config.AddLanEposPrinter(ip); err != nil {
		logger.Errorf("Failed to save LAN printer: %v", err)
		return fmt.Errorf("failed to save LAN printer: %v", err)
	}

	logger.Debugf("LAN printer added successfully: %s", ip)
	return nil
}

func (a *App) ConfirmRemoveLANPrinter(ip string) (bool, error) {
	logger.Debugf("Remove LAN printer requested: %s", ip)

	resultChan := make(chan bool, 1)
	dialog := application.Get().Dialog.Question().
		SetTitle("Remove Printer").
		SetMessage(fmt.Sprintf("Are you sure you want to remove the printer at %s?", ip))
	dialog.AddButton("Cancel").SetAsCancel().OnClick(func() {
		resultChan <- false
	})
	dialog.AddButton("Confirm").SetAsDefault().OnClick(func() {
		resultChan <- true
	})
	dialog.Show()

	confirmed := <-resultChan
	if confirmed {
		if err := a.config.RemoveLANPrinter(ip); err != nil {
			return false, fmt.Errorf("failed to remove LAN printer: %w", err)
		}
		logger.Infof("LAN printer removed successfully: %s", ip)
		return true, nil
	}
	logger.Debugf("Remove LAN printer cancelled")
	return false, nil
}

func (a *App) CheckLANPrinterStatus(ip string) bool {
	logger.Debugf("Checking LAN printer status: %s", ip)
	return printer.CheckLANPrinter(ip) == nil
}

func (a *App) DownloadLogs() {
	logger.Debugf("Download logs requested")
	configDir := a.config.ConfigDirectory()
	zipName := fmt.Sprintf("epos-proxy-logs-%s.zip", time.Now().Format("2006-01-02"))
	logger.Debugf("Creating logs archive: %s", zipName)

	var filePath string
	var err error

	if runtime.GOOS == "android" {
		filePath = filepath.Join("/storage/emulated/0/Download", zipName)
	} else {
		filePath, err = application.Get().Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
			Title:    "Save Archive",
			Filename: zipName,
			Filters:  []application.FileFilter{{DisplayName: "Zip Archives (*.zip)", Pattern: "*.zip"}},
		}).PromptForSingleSelection()
		if err != nil {
			logger.Errorf("Save dialog failed: %v", err)
			return
		}
		if filePath == "" {
			return
		}
	}

	err = util.CreateZip(filePath, map[string]string{"config": configDir})
	if err != nil {
		logger.Errorf("Log export failed: %v", err)
		application.Get().Dialog.Error().
			SetTitle("Download Logs Failed").
			SetMessage(err.Error()).
			Show()
		return
	}
	logger.Infof("Logs successfully exported to: %s", filePath)

	if runtime.GOOS == "android" {
		application.Get().Dialog.Info().
			SetTitle("Logs Exported").
			SetMessage(fmt.Sprintf("Logs successfully exported to:\n\n%s\n\nRetrieve using:\nadb pull %s", filePath, filePath)).
			Show()
	}
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

func (a *App) SetSupportMode(enabled bool) error {
	logger.Infof("Support Mode toggled: %v", enabled)
	if err := a.config.SetSupportMode(enabled); err != nil {
		logger.Errorf("Failed to save support mode configuration: %v", err)
		return err
	}
	logger.SetSupportMode(enabled)
	return nil
}

func (a *App) IsSupportModeEnabled() bool {
	if a.config == nil {
		return false
	}
	return a.config.Data.SupportMode
}

func (a *App) Quit() {
	logger.Infof("Quit requested by user")
	application.Get().Quit()
}

type LANSettings struct {
	Enabled bool   `json:"enabled"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
}

func (a *App) GetLANSettings() LANSettings {
	return LANSettings{
		Enabled: a.config.IsFirewallAccepted(),
		IP:      util.GetLocalIP(),
		Port:    a.config.GetPort(),
	}
}

func (a *App) ConfigureFirewall() error {
	logger.Infof("Configuring firewall")
	err := util.SetFirewallRule(a.config.GetPort(), a.config.GetOldPort())
	if err != nil {
		if err == util.ErrAuthCancelled {
			logger.Warnf("Firewall configuration cancelled by user")
			if err := a.config.UpdateFirewallPreference(true, false); err != nil {
				logger.Errorf("Failed to save config: %v", err)
				return fmt.Errorf("config error: %v", err)
			}
			return err
		}
		logger.Errorf("Failed to configure firewall: %v", err)
		return err
	}

	logger.Infof("Firewall configured successfully")
	if err := a.config.UpdateFirewallPreference(true, true); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		return fmt.Errorf("config error: %v", err)
	}

	return nil
}

func (a *App) SkipFirewallPrompt() error {
	logger.Infof("User skipped firewall prompt ('Not Now')")
	if err := a.config.UpdateFirewallPreference(true, false); err != nil {
		logger.Errorf("Failed to save config: %v", err)
		return fmt.Errorf("config error: %v", err)
	}
	return nil
}
