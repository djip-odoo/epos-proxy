package main

import (
	"epos-proxy/buildinfo"
	"epos-proxy/logger"

	"github.com/wailsapp/wails/v2/pkg/menu"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func createMenu(app *App) *menu.Menu {
	mainMenu := menu.NewMenu()
	settingsMenu := mainMenu.AddSubmenu("Settings")

	settingsMenu.AddText("Download Logs", nil, func(_ *menu.CallbackData) {
		app.DownloadLogs()
	})

	settingsMenu.AddCheckbox("Auto Start", app.IsAutostartEnabled(), nil, func(cb *menu.CallbackData) {
		handleAutoStartToggle(app, cb)
	})

	settingsMenu.AddText("Network Printing", nil, func(_ *menu.CallbackData) {
		logger.Infof("Network Printing menu item clicked")
		wailsruntime.EventsEmit(app.ctx, "open-firewall-prompt")
	})

	enabled := false
	if app != nil && app.config != nil {
		enabled = app.config.Data.SupportMode
	}
	settingsMenu.AddCheckbox("Support Mode", enabled, nil, func(cb *menu.CallbackData) {
		handleSupportModeToggle(app, cb)
	})

	settingsMenu.AddText("About", nil, func(_ *menu.CallbackData) {
		showAboutDialog(app)
	})

	settingsMenu.AddText("Quit", nil, func(_ *menu.CallbackData) {
		logger.Infof("Quit requested by user")
		wailsruntime.Quit(app.ctx)
	})

	return mainMenu
}

func handleAutoStartToggle(app *App, cb *menu.CallbackData) {
	if checked := cb.MenuItem.Checked; checked {
		if err := app.EnableAutostart(); err != nil {
			logger.Errorf("Failed to enable autostart: %v", err)
		}
		return
	}

	if err := app.DisableAutostart(); err != nil {
		logger.Errorf("Failed to disable autostart: %v", err)
	}
}

func (app *App) ConfirmQuit() bool {
	result, err := wailsruntime.MessageDialog(app.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Quit ePOS Proxy",
		Message:       "Stopping the proxy will prevent POS from printing receipts.\n\nAre you sure you want to quit?",
		Buttons:       []string{"Cancel", "Quit"},
		DefaultButton: "Cancel",
	})

	if err != nil {
		logger.Errorf("Failed to show quit dialog: %v", err)
		return false
	}

	// linux doesn't use Buttons overrides and uses No | Yes for question dialog
	if result != "Yes" && result != "Quit" {
		return false
	}

	return true
}

func showAboutDialog(app *App) {
	_, err := wailsruntime.MessageDialog(app.ctx, wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.InfoDialog,
		Title:   "About Printer Manager",
		Message: buildinfo.GetVersionInfo(),
	})

	if err != nil {
		logger.Errorf("Failed to show about dialog: %v", err)
	}
}

func handleSupportModeToggle(app *App, cb *menu.CallbackData) {
	checked := cb.MenuItem.Checked
	logger.Infof("Support Mode toggled: %v", checked)
	if err := app.config.SetSupportMode(checked); err != nil {
		logger.Errorf("Failed to save support mode configuration: %v", err)
	}
	logger.SetSupportMode(checked)
}
