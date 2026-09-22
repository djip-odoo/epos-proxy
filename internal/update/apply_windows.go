//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"epos-proxy/internal/logger"
)

// noWindowFlag = CREATE_NO_WINDOW: the helper console is never shown.
const noWindowFlag = 0x08000000

// Apply schedules a hidden helper that waits for the running app to exit, runs
// the downloaded NSIS installer silently and then relaunches the app. The app
// quits right after Apply returns so the installer can replace the binary
// without hitting a file still in use by the process.
//
// The helper invokes the installer and the app directly instead of going
// through `start`, whose quoted-first-argument window-title parsing can turn a
// valid path into a stray "\..." and fail with "Windows cannot find '...'".
func Apply(downloaded string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("apply failed: cannot locate executable: %w", err)
	}

	// Reject paths that would break the &-chained cmd line.
	for _, s := range []string{downloaded, exe} {
		if strings.ContainsAny(s, "&|<>^\"") {
			return fmt.Errorf("apply failed: update path contains characters not supported by the updater: %s", s)
		}
	}

	logger.Infof("Scheduling installer for %s", downloaded)

	// Wait ~2s for this app to shut down, install silently, then relaunch.
	script := `ping -n 3 127.0.0.1 >nul & "` + downloaded + `" /S & "` + exe + `"`

	cmd := exec.Command("cmd.exe", "/C", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: noWindowFlag}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("apply failed: cannot launch updater: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}