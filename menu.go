package main

import (
	_ "embed"
	"runtime"

	"epos-proxy/buildinfo"
	"epos-proxy/logger"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed build/appicon.png
var appIcon []byte

func createMenu(wailsApp *application.App, svc *App) {
	menu := wailsApp.NewMenu()
	if runtime.GOOS == "darwin" {
		menu.AddRole(application.AppMenu)
	}
	menu.AddRole(application.FileMenu)

	appMenu := menu.AddSubmenu("App")

	appMenu.Add("Download Logs").OnClick(func(_ *application.Context) {
		svc.DownloadLogs()
	})

	appMenu.AddCheckbox("Auto Start", svc.IsAutostartEnabled()).OnClick(func(ctx *application.Context) {
		if ctx.ClickedMenuItem().Checked() {
			if err := svc.EnableAutostart(); err != nil {
				logger.Errorf("Failed to enable autostart: %v", err)
			}
		} else {
			if err := svc.DisableAutostart(); err != nil {
				logger.Errorf("Failed to disable autostart: %v", err)
			}
		}
	})

	appMenu.Add("Network Printing").OnClick(func(_ *application.Context) {
		logger.Infof("Network Printing menu item clicked")
		application.Get().Event.Emit("open-firewall-prompt")
	})

	enabled := false
	if svc.config != nil {
		enabled = svc.config.Data.SupportMode
	}
	appMenu.AddCheckbox("Support Mode", enabled).OnClick(func(ctx *application.Context) {
		checked := ctx.ClickedMenuItem().Checked()
		logger.Infof("Support Mode toggled: %v", checked)
		if err := svc.config.SetSupportMode(checked); err != nil {
			logger.Errorf("Failed to save support mode configuration: %v", err)
		}
		logger.SetSupportMode(checked)
	})

	appMenu.Add("About").OnClick(func(_ *application.Context) {
		application.Get().Dialog.Info().
			SetTitle("About ePOS Proxy").
			SetMessage(buildinfo.GetVersionInfo()).
			Show()
	})

	appMenu.Add("Quit").OnClick(func(_ *application.Context) {
		logger.Infof("Quit requested by user")

		resultChan := make(chan bool, 1)
		dialog := application.Get().Dialog.Question().
			SetTitle("Quit ePOS Proxy").
			SetMessage("Stopping the proxy will prevent POS from printing receipts.\n\nAre you sure you want to quit?")
		dialog.AddButton("Cancel").SetAsDefault().SetAsCancel().OnClick(func() {
			resultChan <- false
		})
		dialog.AddButton("Quit").OnClick(func() {
			resultChan <- true
		})
		dialog.Show()

		if <-resultChan {
			application.Get().Quit()
		}
	})

	wailsApp.Menu.Set(menu)
}

func setupSystemTray(wailsApp *application.App, svc *App, mainWindow application.Window) {
	systray := wailsApp.SystemTray.New()
	systray.SetIcon(appIcon)
	systray.SetTooltip("ePOS Proxy")
	systray.AttachWindow(mainWindow)

	// Custom click handlers to toggle window visibility
	systray.OnClick(func() {
		if mainWindow.IsVisible() {
			mainWindow.Hide()
		} else {
			mainWindow.Show().Focus()
		}
	})
	systray.OnDoubleClick(func() {
		mainWindow.Show().Focus()
	})

	trayMenu := wailsApp.NewMenu()

	trayMenu.Add("Show").OnClick(func(_ *application.Context) {
		mainWindow.Show().Focus()
	})

	trayMenu.AddSeparator()

	trayMenu.Add("Quit").OnClick(func(_ *application.Context) {
		logger.Infof("Quit requested by user from system tray")

		resultChan := make(chan bool, 1)
		dialog := application.Get().Dialog.Question().
			SetTitle("Quit ePOS Proxy").
			SetMessage("Stopping the proxy will prevent POS from printing receipts.\n\nAre you sure you want to quit?")
		dialog.AddButton("Cancel").SetAsDefault().SetAsCancel().OnClick(func() {
			resultChan <- false
		})
		dialog.AddButton("Quit").OnClick(func() {
			resultChan <- true
		})
		dialog.Show()

		if <-resultChan {
			application.Get().Quit()
		}
	})

	systray.SetMenu(trayMenu)
}
