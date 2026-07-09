package main

import (
	"C"
	"embed"
	"epos-proxy/config"
	"epos-proxy/logger"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	os.Setenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "1")
	cfg := config.InitConfig()
	logger.InitLogger(cfg)
	svc := NewApp(cfg)

	wailsApp := application.New(application.Options{
		Name:        "ePOS Proxy",
		Description: "Expose USB and network printers as HTTP endpoints",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "epos-proxy-single-instance",
		},
	})

	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "ePOS Proxy",
		Width:            800,
		Height:           600,
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
	})

	mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		mainWindow.Hide()
	})

	mainWindow.OnWindowEvent(events.Common.WindowRuntimeReady, func(event *application.WindowEvent) {
		svc.domReady(wailsApp.Context())
	})

	createMenu(wailsApp, svc)
	setupSystemTray(wailsApp, svc, mainWindow)

	err := wailsApp.Run()
	if err != nil {
		logger.Errorf("Application crashed: %v", err)
	}
}
