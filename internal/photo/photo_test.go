// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package photo

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessFileStripsMetadataAndSquares(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src.jpg")
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 6), G: 40, B: 80, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	f.Close()

	out, err := ProcessFile(path, CropParams{Zoom: 1}, Options{MaxSide: 32, JPEGQuality: 85, MaxPixels: 1e6})
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if format != "jpeg" {
		t.Fatalf("format = %s", format)
	}
	if cfg.Width != cfg.Height {
		t.Fatalf("got %dx%d, want square", cfg.Width, cfg.Height)
	}
	if cfg.Width > 32 {
		t.Fatalf("side = %d, want <= 32", cfg.Width)
	}
}

func TestProcessFileRejectsHugePixels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.jpg")
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = jpeg.Encode(f, img, nil)
	f.Close()

	_, err = ProcessFile(path, CropParams{Zoom: 1}, Options{MaxPixels: 100})
	if err == nil {
		t.Fatal("expected pixel limit error")
	}
}

func TestPlaceholderSVGDeterministic(t *testing.T) {
	a := PlaceholderSVG("uid-1", "Ada Lovelace")
	b := PlaceholderSVG("uid-1", "Ada Lovelace")
	if !bytes.Equal(a, b) {
		t.Fatal("placeholder not deterministic")
	}
	if !bytes.Contains(a, []byte("AL")) {
		t.Fatalf("initials missing: %s", a)
	}
}

func TestAllowedMediaType(t *testing.T) {
	if !AllowedMediaType("image/jpeg") || AllowedMediaType("image/svg+xml") {
		t.Fatal("media type gate wrong")
	}
}
