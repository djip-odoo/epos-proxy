//go:build windows

package assets

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed SumatraPDF-3.6-32.exe
var sumatraBinary []byte

func GetSumatraPDFPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config dir: %w", err)
	}

	appDir := filepath.Join(dir, "epos-proxy")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create app dir: %w", err)
	}

	exePath := filepath.Join(appDir, "SumatraPDF.exe")
	if _, err := os.Stat(exePath); err == nil {
		return exePath, nil
	}
	if err := os.WriteFile(exePath, sumatraBinary, 0755); err != nil {
		return "", fmt.Errorf("failed to write SumatraPDF executable: %w", err)
	}

	return exePath, nil
}
