package main

import (
	"C"
	"embed"

	"epos-proxy/config"
	"epos-proxy/logger"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
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
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "epos-proxy-single-instance",
		},
	})

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "ePOS Proxy",
		Width:            800,
		Height:           600,
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
	})

	createMenu(wailsApp, svc)

	err := wailsApp.Run()
	if err != nil {
		logger.Errorf("Application crashed: %v", err)
	}
}
