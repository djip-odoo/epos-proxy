package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"epos-proxy/internal/logger"
)

func runServerMode(app *App) {
	logger.Infof("Starting application in server/kiosk mode")

	// Ensure auto-start on Windows startup is enabled in server mode
	if runtime.GOOS == "windows" {
		if !app.IsAutostartEnabled() {
			if err := app.EnableAutostart(); err != nil {
				logger.Warnf("Failed to configure auto-start on Windows startup: %v", err)
			} else {
				logger.Infof("Configured auto-start on Windows startup for server mode")
			}
		}
	}

	bindHost := "127.0.0.1"
	if app.config.IsNetworkPrintingEnabled() {
		bindHost = "0.0.0.0"
	}

	port, err := app.startBackend(bindHost)
	if err != nil {
		logger.Errorf("HTTP server startup failure: %v", err)
		return
	}

	logger.Infof("Starting HTTP server on %s:%d", bindHost, port)

	// Wait for server to become ready
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.webserver.WaitReady(ctx); err != nil {
		logger.Errorf("HTTP server startup failure: %v", err)
		return
	}

	logger.Infof("HTTP server ready")

	// Keep process alive until termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Infof("Shutting down server mode")
	if app.webserver != nil {
		if err := app.webserver.Stop(); err != nil {
			logger.Errorf("Server stop error: %v", err)
		}
	}
}
