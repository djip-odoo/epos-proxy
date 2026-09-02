package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"epos-proxy/internal/config"
	"epos-proxy/internal/testutil"
)

func TestDetermineLaunchMode(t *testing.T) {
	tests := []struct {
		name         string
		isService    bool
		forceKiosk   bool
		kioskEnabled bool
		expectedMode LaunchMode
	}{
		{
			name:         "Service mode takes priority over everything",
			isService:    true,
			forceKiosk:   false,
			kioskEnabled: false,
			expectedMode: ModeService,
		},
		{
			name:         "Service mode when kiosk enabled is also ModeService",
			isService:    true,
			forceKiosk:   true,
			kioskEnabled: true,
			expectedMode: ModeService,
		},
		{
			name:         "Interactive: kiosk.enabled missing/false -> ModeNormal",
			isService:    false,
			forceKiosk:   false,
			kioskEnabled: false,
			expectedMode: ModeNormal,
		},
		{
			name:         "Interactive: kiosk.enabled true -> ModeServer",
			isService:    false,
			forceKiosk:   false,
			kioskEnabled: true,
			expectedMode: ModeServer,
		},
		{
			name:         "Interactive: CLI flag --kiosk -> ModeServer",
			isService:    false,
			forceKiosk:   true,
			kioskEnabled: false,
			expectedMode: ModeServer,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mode := determineLaunchMode(tc.isService, tc.forceKiosk, tc.kioskEnabled)
			testutil.ExpectedEqual(t, mode, tc.expectedMode)
		})
	}
}

func TestConfigKioskModeResolution(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Missing kiosk section -> normal mode
	cfgFile1 := filepath.Join(tempDir, "config1.json")
	err := os.WriteFile(cfgFile1, []byte(`{"port": 4545}`), 0644)
	testutil.ExpectedNoError(t, err)

	var appCfg1 config.AppConfig
	data1, _ := os.ReadFile(cfgFile1)
	_ = json.Unmarshal(data1, &appCfg1)
	testutil.ExpectedEqual(t, appCfg1.Kiosk.Enabled, false)
	testutil.ExpectedEqual(t, determineLaunchMode(false, false, appCfg1.Kiosk.Enabled), ModeNormal)

	// 2. kiosk.enabled = false -> normal mode
	cfgFile2 := filepath.Join(tempDir, "config2.json")
	err = os.WriteFile(cfgFile2, []byte(`{"port": 4545, "kiosk": {"enabled": false}}`), 0644)
	testutil.ExpectedNoError(t, err)

	var appCfg2 config.AppConfig
	data2, _ := os.ReadFile(cfgFile2)
	_ = json.Unmarshal(data2, &appCfg2)
	testutil.ExpectedEqual(t, appCfg2.Kiosk.Enabled, false)
	testutil.ExpectedEqual(t, determineLaunchMode(false, false, appCfg2.Kiosk.Enabled), ModeNormal)

	// 3. kiosk.enabled = true -> server mode
	cfgFile3 := filepath.Join(tempDir, "config3.json")
	err = os.WriteFile(cfgFile3, []byte(`{"port": 4545, "kiosk": {"enabled": true}}`), 0644)
	testutil.ExpectedNoError(t, err)

	var appCfg3 config.AppConfig
	data3, _ := os.ReadFile(cfgFile3)
	_ = json.Unmarshal(data3, &appCfg3)
	testutil.ExpectedEqual(t, appCfg3.Kiosk.Enabled, true)
	testutil.ExpectedEqual(t, determineLaunchMode(false, false, appCfg3.Kiosk.Enabled), ModeServer)

	// 4. Windows Service launched -> always ModeService
	testutil.ExpectedEqual(t, determineLaunchMode(true, false, appCfg3.Kiosk.Enabled), ModeService)
}

func TestBackendHeadlessServiceLifecycle(t *testing.T) {
	// Verify backend can start on 127.0.0.1, respond to HTTP requests,
	// and stop cleanly without Wails or WebView2 initialization.
	app := NewApp()

	port, err := app.startBackend("127.0.0.1")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, port > 0, "Expected positive port")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = app.webserver.WaitReady(ctx)
	testutil.ExpectedNoError(t, err)

	// Verify server responds on 127.0.0.1
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	testutil.ExpectedNoError(t, err)
	defer resp.Body.Close()
	testutil.ExpectedTrue(t, resp.StatusCode > 0, "Expected valid HTTP status code")

	// Verify graceful shutdown
	err = app.webserver.Stop()
	testutil.ExpectedNoError(t, err)

	// Verify server is no longer accessible
	client := &http.Client{Timeout: 500 * time.Millisecond}
	_, err = client.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	testutil.ExpectedTrue(t, err != nil, "Server should be stopped")
}
