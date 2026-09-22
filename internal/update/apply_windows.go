//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"epos-proxy/internal/logger"
)

// noWindowFlag = CREATE_NO_WINDOW: the helper console is never shown.
const noWindowFlag = 0x08000000

// Apply schedules a detached, hidden helper that waits for the running app to
// exit, runs the downloaded NSIS installer silently and then relaunches the
// app. The app quits right after Apply returns so the installer can replace
// the binary without hitting a file that is still in use by the process.
func Apply(downloaded string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("apply failed: cannot locate executable: %w", err)
	}

	logger.Infof("Scheduling installer for %s", downloaded)

	// Give the app a moment to shut down, then install silently and relaunch.
	script := fmt.Sprintf(
		`timeout /t 2 /nobreak >nul & start "" /wait "%[1]s" /S & start "" "%[2]s"`,
		downloaded, exe,
	)

	cmd := exec.Command("cmd.exe", "/C", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: noWindowFlag}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("apply failed: cannot launch updater: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}
