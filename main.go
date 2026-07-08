package main

import (
	"C"
	"embed"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"epos-proxy/config"
	"epos-proxy/logger"
	"epos-proxy/printer"
	"epos-proxy/server"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	os.Setenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "1")
	cfg := config.InitConfig()
	logger.InitLogger(cfg)
	svc := NewApp(cfg)

	// If running headless on Linux (no X11 / Wayland), bypass Wails GUI and run HTTP server directly
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		logger.Warn("No display server detected. Running in headless/server-only mode...")
		port, err := cfg.ResolvePort()
		if err != nil {
			logger.Warn("Unable to resolve port, using default")
		}
		if err := cfg.CheckPortChange(); err != nil {
			logger.Errorf("Failed to check port change: %v", err)
		}

		printerManager := printer.NewManager()
		_ = server.New(port, printerManager)

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down ePOS Proxy...")
		os.Exit(0)
	}

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
