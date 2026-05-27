package util

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func ZipPaths(targetZip string, paths map[string]string) error {
	zipFile, err := os.Create(targetZip)
	if err != nil {
		return fmt.Errorf("failed to create zip file %s: %w", targetZip, err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for zipRoot, sourcePath := range paths {
		err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(sourcePath, path)
			if err != nil {
				return err
			}

			zipPath := filepath.Join(zipRoot, relPath)

			return zipEntry(zipWriter, path, zipPath)
		})

		if err != nil {
			return fmt.Errorf("failed to zip path %s: %w", sourcePath, err)
		}
	}

	return nil
}

func zipEntry(zipWriter *zip.Writer, sourcePath, zipPath string) error {
	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", sourcePath, err)
	}
	defer src.Close()

	w, err := zipWriter.Create(filepath.ToSlash(zipPath))
	if err != nil {
		return fmt.Errorf("failed to create zip entry %s: %w", zipPath, err)
	}

	if _, err = io.Copy(w, src); err != nil {
		return fmt.Errorf("failed to copy file contents for %s: %w", sourcePath, err)
	}

	return nil
}
