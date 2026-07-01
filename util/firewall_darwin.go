package util

import (
	"errors"
)

// ErrAuthCancelled is declared on all platforms for interface parity.
var ErrAuthCancelled = errors.New("authentication cancelled")

// allowPortOS is a no-op on macOS: the OS uses an application-level firewall,
// not port-based rules. The application must be allowed in
// System Settings → Privacy & Security → Firewall.
// The UI surfaces this guidance to the user when running on macOS.
func allowApplicationOS() error {
	return nil
}

func allowPortOS(port int) error {
	return nil
}

func blockPortOS(port int) error {
	return nil
}
