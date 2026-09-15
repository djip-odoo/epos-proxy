package server

import (
	"testing"

	"epos-proxy/internal/config"
	"epos-proxy/internal/printer"
	"epos-proxy/internal/printer/lan"
	"epos-proxy/internal/testutil"
)

// provides a centralized, clean test server setup for all server package tests.
func newTestServer(t testing.TB, cfg *config.Manager) (*Server, *printer.Manager) {
	t.Helper()
	if cfg == nil {
		t.Setenv("HOME", t.TempDir())
		var err error
		cfg, err = config.NewManager()
		testutil.ExpectedNoError(t, err)
	}
	if cfg.Data.Port == 0 {
		cfg.Data.Port = testutil.GetFreePort(t)
	}
	mgr := printer.NewManager(lan.NewDriver(cfg))
	s := New(cfg.Data.Port, mgr)
	t.Cleanup(func() {
		_ = s.Stop()
		mgr.Close()
	})
	return s, mgr
}

func TestServer_Lifecycle(t *testing.T) {
	s, _ := newTestServer(t, nil)
	testutil.ExpectedTrue(t, s.Running(), "Expected server to be running after New()")
	testutil.ExpectedTrue(t, s.Port > 0)
}
