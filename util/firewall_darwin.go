package util

import (
	"errors"

	"epos-proxy/logger"
)

// ErrAuthCancelled is declared on all platforms for interface parity.
var ErrAuthCancelled = errors.New("authentication cancelled")

// allowPortOS is a no-op on macOS: the OS uses an application-level firewall,
// not port-based rules. The application must be allowed in
// System Settings → Privacy & Security → Firewall.
// The UI surfaces this guidance to the user when running on macOS.
func allowPortOS(port int) error {
	logger.Infof(
		"macOS uses an application-based firewall; no explicit port rule is required (port %d). "+
			"Ensure this application is allowed in System Settings → Privacy & Security → Firewall.",
		port,
	)
	return nil
}

// blockPortOS is a no-op on macOS for the same reason as allowPortOS.
func blockPortOS(port int) error {
	logger.Infof(
		"macOS uses an application-based firewall; no port rule removal is required (port %d).",
		port,
	)
	return nil
}
