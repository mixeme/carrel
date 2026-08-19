// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sidecar

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func extractArchive(archivePath, archiveName, goos, binaryName string) (string, error) {
	if strings.HasSuffix(strings.ToLower(archiveName), ".zip") {
		return extractZip(archivePath, binaryName)
	}
	return extractTarGz(archivePath, binaryName)
}

func extractTarGz(path, binaryName string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	dir := filepath.Dir(path)
	out, err := os.CreateTemp(dir, "carrel-sidecar-*")
	if err != nil {
		return "", err
	}
	outPath := out.Name()
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = out.Close()
			_ = os.Remove(outPath)
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(hdr.Name)
		if name != binaryName {
			continue
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			_ = os.Remove(outPath)
			return "", err
		}
		found = true
		break
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	if !found {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("sidecar: %q not found in archive", binaryName)
	}
	if err := os.Chmod(outPath, 0o755); err != nil && !isUnsupportedChmod(err) {
		_ = os.Remove(outPath)
		return "", err
	}
	return outPath, nil
}

func extractZip(path, binaryName string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()
	dir := filepath.Dir(path)
	var zf *zip.File
	for _, f := range r.File {
		if filepath.Base(f.Name) == binaryName {
			zf = f
			break
		}
	}
	if zf == nil {
		return "", fmt.Errorf("sidecar: %q not found in archive", binaryName)
	}
	rc, err := zf.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	out, err := os.CreateTemp(dir, "carrel-sidecar-*")
	if err != nil {
		return "", err
	}
	outPath := out.Name()
	if _, err := io.Copy(out, rc); err != nil {
		_ = out.Close()
		_ = os.Remove(outPath)
		return "", err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	return outPath, nil
}
