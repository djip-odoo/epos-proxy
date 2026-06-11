package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"time"

	"epos-proxy/config"
	"epos-proxy/logger"
	"epos-proxy/printer"
	"epos-proxy/server"

	autostart "github.com/emersion/go-autostart"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx     context.Context
	service *PrinterService
}

func NewApp(assets embed.FS) *App {
	a := &App{}

	autoStart := &autostart.App{
		Name:        "epos-proxy",
		DisplayName: "ePOS Proxy",
		Exec:        []string{os.Args[0]},
	}

	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logger.Debugf("Application startup")

	cfg, err := config.NewManager()
	if err != nil {
		logger.Fatalf("Config initialization failed: %v", err)
	}

	if err := cfg.Load(); err != nil {
		logger.Warnf("Config load warning: %v", err)
	}

	pm := printer.NewManager()
	a.service = NewPrinterService(cfg, pm, autoStart, assets)

	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logger.Debugf("Application startup")

	logger.Debugf("Config loaded from %s", a.service.config.Path())

	port, err := a.service.config.ResolvePort()
	if err != nil {
		logger.Warn("Unable to resolve port, using default")
	}

	host := "127.0.0.1"
	if a.service.config.Data.LANAccessEnabled {
		host = "0.0.0.0"
	}

	a.service.OnCustomerDisplayAction = func(open bool, url string) {
		if open {
			wailsruntime.EventsEmit(a.ctx, "open-customer-display-webview", url)
		} else {
			wailsruntime.EventsEmit(a.ctx, "close-customer-display-webview")
		}
	}

	a.service.StartServer(port, host)

	// Auto-open WebView if an active customer display URL is configured.
	if active := a.service.GetActiveCustomerDisplayURL(); active != nil {
		logger.Infof("Auto-opening customer display WebView for URL: %s", active.URL)
		a.service.SetCustomerDisplayOpen(true)
	}
}

func (a *App) shutdown(ctx context.Context) {
	logger.Infof("Stopping proxy server")
	if err := a.service.StopServer(); err != nil {
		logger.Errorf("Server stop error: %v", err)
	}
}

func (a *App) RestartServer() {
	a.service.RestartServer()
}

func (a *App) GetPrinterIp(id string) string {
	return a.service.GetPrinterIp(id)
}

func (a *App) Status() server.Status {
	status, err := a.service.Status()
	if err != nil {
		logger.Errorf("Failed to retrieve status: %v", err)
		return server.Status{ErrorMsg: err.Error()}
	}
	return status
}

func (a *App) AddLANPrinter(ip string) error {
	return a.service.AddLANPrinter(ip)
}

func (a *App) SetLANPin(pin string) error {
	return a.service.SetLANPin(pin)
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
		if err := a.service.RemoveLANPrinter(ip); err != nil {
			return false, fmt.Errorf("failed to remove LAN printer: %w", err)
		}
		return true, nil
	}
	logger.Infof("Remove LAN printer cancelled, Remove printer dialog result: %s", result)
	return false, nil
}

func (a *App) CheckLANPrinterStatus(ip string) bool {
	return a.service.CheckLANPrinterStatus(ip)
}

func (a *App) DownloadLogs() {
	logger.Debugf("Download logs requested")
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
	if err != nil {
		logger.Errorf("Save file dialog failed: %v", err)
		return
	}
	if savePath == "" {
		logger.Infof("Export cancelled by user")
		return
	}
	err = a.service.ExportLogsToPath(savePath)
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
	return a.service.IsAutostartEnabled()
}

func (a *App) EnableAutostart() error {
	return a.service.EnableAutostart()
}

func (a *App) DisableAutostart() error {
	return a.service.DisableAutostart()
}

func (a *App) SetPrinterSetting(id string, width int, bottomPadding int, protocol string) error {
	return a.service.SetPrinterSetting(id, width, bottomPadding, protocol)
}

func (a *App) GetPrinterSetting(id string) config.PrinterSettingConfig {
	return a.service.GetPrinterSetting(id)
}

// ─── Customer Display WebView ─────────────────────────────────────────────────

func (a *App) GetCustomerDisplayURLs() []config.CustomerDisplayURL {
	return a.service.GetCustomerDisplayURLs()
}

func (a *App) GetActiveCustomerDisplayURL() *config.CustomerDisplayURL {
	return a.service.GetActiveCustomerDisplayURL()
}

func (a *App) AddCustomerDisplayURL(name, rawURL, description string) (config.CustomerDisplayURL, error) {
	return a.service.AddCustomerDisplayURL(name, rawURL, description)
}

func (a *App) UpdateCustomerDisplayURL(id, name, rawURL, description string, enabled bool) error {
	return a.service.UpdateCustomerDisplayURL(id, name, rawURL, description, enabled)
}

func (a *App) SetActiveCustomerDisplayURL(id string) error {
	return a.service.SetActiveCustomerDisplayURL(id)
}

func (a *App) DisableCustomerDisplayURL(id string) error {
	return a.service.DisableCustomerDisplayURL(id)
}

func (a *App) DeleteCustomerDisplayURL(id string) error {
	return a.service.DeleteCustomerDisplayURL(id)
}

func (a *App) ValidateAdminPin(pin string) bool {
	return a.service.ValidateAdminPin(pin)
}

func (a *App) SetWindowFullscreen(fullscreen bool) {
	if fullscreen {
		logger.Infof("Setting window to fullscreen for customer display WebView")

		// Frameless: true,
		// DisableResize: true,
		// StartHidden: true,
		wailsruntime.WindowFullscreen(a.ctx)
	} else {
		logger.Infof("Restoring window from fullscreen")
		wailsruntime.WindowUnfullscreen(a.ctx)
	}
}

func (a *App) IsCustomerDisplayOpen() bool {
	return a.service.IsCustomerDisplayOpen()
}

func (a *App) SetCustomerDisplayOpen(open bool) error {
	return a.service.SetCustomerDisplayOpen(open)
}

