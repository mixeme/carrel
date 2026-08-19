// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sidecar

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseChecksum(t *testing.T) {
	hash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	body := hash + "  carrel_0.10.0_linux_amd64.tar.gz\n"
	got, err := parseChecksum(bytes.NewBufferString(body), "carrel_0.10.0_linux_amd64.tar.gz")
	if err != nil || got != hash {
		t.Fatalf("parseChecksum() = %q, %v", got, err)
	}
}

func TestDownloadInstall(t *testing.T) {
	installDir := t.TempDir()
	version := "0.10.0"
	archive := ArchiveNameFor(version, runtime.GOOS, runtime.GOARCH)
	payload := []byte("#!/bin/sh\necho carrel\n")
	archiveBytes, err := buildTestArchive(t, archive, BinaryName(runtime.GOOS), payload)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archiveBytes)
	checksums := hex.EncodeToString(sum[:]) + "  " + archive + "\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/releases/download/v0.10.0/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	mux.HandleFunc("/releases/download/v0.10.0/"+archive, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archiveBytes)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	base := srv.URL + "/releases/download/v0.10.0"
	dl := &Downloader{
		HTTPClient: srv.Client(),
	}
	// Override URLs by using a custom download that points at test server.
	if err := dl.downloadFromBase(context.Background(), version, installDir, base, archive, runtime.GOOS); err != nil {
		t.Fatalf("downloadFromBase() error: %v", err)
	}

	bin := filepath.Join(installDir, BinaryName(runtime.GOOS))
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("binary content mismatch")
	}
	info, err := ReadVersion(filepath.Join(installDir, "version.json"))
	if err != nil || info.Version != "0.10.0" {
		t.Fatalf("version %+v, %v", info, err)
	}
}

func TestEnsureSkipsWhenPresent(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, BinaryName(runtime.GOOS))
	if err := os.WriteFile(bin, []byte("ok"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteVersion(filepath.Join(dir, "version.json"), "0.10.0"); err != nil {
		t.Fatal(err)
	}
	called := false
	err := Ensure(context.Background(), EnsureOptions{
		Paths: InstallPathsFrom(dir, bin, filepath.Join(dir, "version.json")),
		Version: "0.10.0",
		Downloader: &Downloader{
			HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				called = true
				return nil, errors.New("should not download")
			})},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("downloaded when sidecar already present")
	}
}

func TestVerifySHA256Mismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := verifySHA256File(path, stringsRepeat("a", 64))
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func buildTestArchive(t *testing.T, archiveName, binaryName string, payload []byte) ([]byte, error) {
	t.Helper()
	if strings.HasSuffix(strings.ToLower(archiveName), ".zip") {
		return buildZip(payload, binaryName)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: binaryName, Mode: 0o755, Size: int64(len(payload)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write(payload); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildZip(payload []byte, name string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(payload); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func stringsRepeat(s string, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = s[0]
	}
	return string(b)
}
