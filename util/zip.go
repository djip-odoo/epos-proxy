package util

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// CreateZip creates a zip archive containing all files from the provided paths.
//
// Example:
//
//	CreateZip("diagnostics.zip", map[string]string{
//		"logs":   "/var/log/eposproxy",
//		"config": "/home/user/.config/EposProxy",
//	})
func CreateZip(targetZip string, paths map[string]string) error {
	file, err := os.Create(targetZip)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	for zipRoot, sourcePath := range paths {
		if err := addPathToZip(zw, zipRoot, sourcePath); err != nil {
			return err
		}
	}

	return nil
}

// CreateTempZip creates a diagnostic zip inside the system temp directory
// and returns the generated file path.
func CreateTempZip(paths map[string]string) (string, error) {
	zipPath := filepath.Join(
		os.TempDir(),
		fmt.Sprintf(
			"eposproxy-diagnostics-%s.zip",
			time.Now().Format("20060102-150405"),
		),
	)

	if err := CreateZip(zipPath, paths); err != nil {
		return "", err
	}

	return zipPath, nil
}

func addPathToZip(zw *zip.Writer, zipRoot, sourcePath string) error {
	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("relative path: %w", err)
		}

		zipPath := filepath.Join(zipRoot, relPath)

		if err := addFileToZip(zw, path, zipPath); err != nil {
			return err
		}

		return nil
	})
}

func addFileToZip(zw *zip.Writer, sourcePath, zipPath string) error {
	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", sourcePath, err)
	}
	defer src.Close()

	writer, err := zw.Create(filepath.ToSlash(zipPath))
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", zipPath, err)
	}

	if _, err := io.Copy(writer, src); err != nil {
		return fmt.Errorf("copy %s: %w", sourcePath, err)
	}

	return nil
}
