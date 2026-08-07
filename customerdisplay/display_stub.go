//go:build !windows && !darwin && !linux

// Package customerdisplay — stub for unsupported platforms / binding generators.
// This file satisfies the linker when no real platform file is selected.
package customerdisplay

func platformInit()                      {}
func platformGetMonitors() []MonitorInfo { return nil }
func platformOpen(monitorID, url string) {}
func platformClose()                     {}
func platformReload()                    {}
func platformNavigate(url string)        {}
func platformIdentify()                  {}
func platformTest(monitorID string)      {}
