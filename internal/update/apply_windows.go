//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"

	"epos-proxy/internal/logger"
)

// Apply runs the downloaded NSIS installer silently, then relaunches the app.
// The installer replaces the installed binary and start-menu shortcuts.
func Apply(downloaded string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("apply failed: cannot locate executable: %w", err)
	}

	logger.Infof("Running installer %s silently", downloaded)
	cmd := exec.Command("cmd.exe", "/C", `start`, `""`, `/wait`, downloaded, `/S`)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apply failed: installer exited with an error: %w", err)
	}

	logger.Infof("Relaunching %s", exe)
	if err := exec.Command("cmd.exe", "/C", `start`, `""`, exe).Start(); err != nil {
		return fmt.Errorf("apply failed: cannot relaunch app: %w", err)
	}
	return nil
}
