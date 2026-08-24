//go:build !linux

package main

func setNativeMenubarVisible(visible bool) {
	// Handled via Wails MenuSetApplicationMenu on Windows and macOS
}
