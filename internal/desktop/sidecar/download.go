// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sidecar

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	ErrChecksumMismatch = errors.New("sidecar: checksum mismatch")
	ErrChecksumMissing  = errors.New("sidecar: checksum not found")
	ErrDownload         = errors.New("sidecar: download failed")
	ErrInstallDir       = errors.New("sidecar: install directory not writable")
)

// Downloader fetches release archives from GitHub.
type Downloader struct {
	Repo       string
	HTTPClient *http.Client
}

func (d *Downloader) client() *http.Client {
	if d.HTTPClient != nil {
		return d.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

// Download installs the sidecar for version into installDir.
func (d *Downloader) Download(ctx context.Context, version, installDir string) error {
	repo := d.Repo
	if repo == "" {
		repo = DefaultReleaseRepo
	}
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	archive := ArchiveNameFor(version, goos, goarch)
	base := ReleaseBaseURL(repo, version)
	return d.downloadFromBase(ctx, version, installDir, base, archive, goos)
}

func (d *Downloader) downloadFromBase(ctx context.Context, version, installDir, baseURL, archive, goos string) error {
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("%w: %v", ErrInstallDir, err)
	}
	if err := checkWritable(installDir); err != nil {
		return err
	}

	checksum, err := d.fetchChecksum(ctx, baseURL, archive)
	if err != nil {
		return err
	}

	tmpArchive, err := os.CreateTemp(installDir, "carrel-download-*.tmp")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDownload, err)
	}
	tmpArchivePath := tmpArchive.Name()
	cleanupArchive := func() { _ = os.Remove(tmpArchivePath) }
	defer cleanupArchive()

	assetURL := AssetURL(baseURL, archive)
	if err := d.fetchFile(ctx, assetURL, tmpArchive); err != nil {
		_ = tmpArchive.Close()
		return err
	}
	if err := tmpArchive.Close(); err != nil {
		return fmt.Errorf("%w: close archive: %v", ErrDownload, err)
	}

	if err := verifySHA256File(tmpArchivePath, checksum); err != nil {
		return err
	}

	binaryName := BinaryName(goos)
	extracted, err := extractArchive(tmpArchivePath, archive, goos, binaryName)
	if err != nil {
		return fmt.Errorf("%w: extract: %v", ErrDownload, err)
	}
	defer os.Remove(extracted)

	dst := filepath.Join(installDir, binaryName)
	mode := os.FileMode(0o755)
	if goos == "windows" {
		mode = 0o755
	}
	if err := atomicReplaceFile(dst, extracted, mode); err != nil {
		return fmt.Errorf("%w: install binary: %v", ErrInstallDir, err)
	}
	versionPath := filepath.Join(installDir, "version.json")
	if err := WriteVersion(versionPath, version); err != nil {
		return err
	}
	return nil
}

func (d *Downloader) fetchChecksum(ctx context.Context, baseURL, archive string) (string, error) {
	url := AssetURL(baseURL, "checksums.txt")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := d.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: checksums.txt: %v", ErrDownload, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: checksums.txt: HTTP %d", ErrDownload, resp.StatusCode)
	}
	sum, err := parseChecksum(resp.Body, archive)
	if err != nil {
		return "", err
	}
	return sum, nil
}

func parseChecksum(r io.Reader, archive string) (string, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := fields[0]
		name := fields[len(fields)-1]
		if strings.TrimPrefix(name, "*") == archive {
			hash = strings.TrimPrefix(strings.ToLower(hash), "sha256:")
			if len(hash) != 64 {
				return "", fmt.Errorf("%w: bad hash for %s", ErrChecksumMissing, archive)
			}
			return hash, nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%w: %s", ErrChecksumMissing, archive)
}

func (d *Downloader) fetchFile(ctx context.Context, url string, dst *os.File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := d.client().Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrDownload, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s: HTTP %d", ErrDownload, url, resp.StatusCode)
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrDownload, url, err)
	}
	return nil
}

func verifySHA256File(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%w: got %s want %s", ErrChecksumMismatch, got, want)
	}
	return nil
}

func checkWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInstallDir, err)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}
