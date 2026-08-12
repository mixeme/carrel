// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package photo

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rwcarlsen/goexif/exif"
	_ "golang.org/x/image/webp"
)

// Options configure output size and safety limits (§11).
type Options struct {
	MaxSide     int
	JPEGQuality int
	MaxPixels   int64
	ThumbSide   int
}

func (o Options) withDefaults() Options {
	if o.MaxSide <= 0 {
		o.MaxSide = 512
	}
	if o.JPEGQuality <= 0 {
		o.JPEGQuality = 85
	}
	if o.MaxPixels <= 0 {
		o.MaxPixels = 100_000_000
	}
	if o.ThumbSide <= 0 {
		o.ThumbSide = 96
	}
	return o
}

// CropParams describe a square crop over an oriented source image.
type CropParams struct {
	PanX   float64 // -1..1 offset of crop centre from image centre
	PanY   float64
	Zoom   float64 // 1 = fit longest side into square; larger = zoom in
	Rotate int     // degrees clockwise, multiples of 90
}

func (c CropParams) normalised() CropParams {
	if c.Zoom < 1 {
		c.Zoom = 1
	}
	if c.Zoom > 8 {
		c.Zoom = 8
	}
	if c.PanX < -1 {
		c.PanX = -1
	}
	if c.PanX > 1 {
		c.PanX = 1
	}
	if c.PanY < -1 {
		c.PanY = -1
	}
	if c.PanY > 1 {
		c.PanY = 1
	}
	c.Rotate = ((c.Rotate % 360) + 360) % 360
	c.Rotate = (c.Rotate / 90) * 90
	return c
}

// ProcessFile reads an upload from path, applies EXIF orientation, strips
// metadata by re-encoding, and writes a cropped JPEG to out using params.
func ProcessFile(path string, params CropParams, opts Options) ([]byte, error) {
	opts = opts.withDefaults()
	params = params.normalised()

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("photo: read dimensions: %w", err)
	}
	pixels := int64(cfg.Width) * int64(cfg.Height)
	if pixels > opts.MaxPixels {
		return nil, fmt.Errorf("photo: image is %d×%d (%.0f MP); the limit is %d MP",
			cfg.Width, cfg.Height, float64(pixels)/1e6, opts.MaxPixels/1e6)
	}
	_ = format

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("photo: decode: %w", err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	img = applyOrientation(img, readOrientation(f))

	img = rotateImage(img, params.Rotate)
	cropped := cropSquare(img, params)
	resized := resizeMax(cropped, opts.MaxSide)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: opts.JPEGQuality}); err != nil {
		return nil, fmt.Errorf("photo: encode JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// Thumbnail shrinks a JPEG/PNG/GIF/WebP to a square thumb.
func Thumbnail(src []byte, side int) ([]byte, string, error) {
	if side <= 0 {
		side = 96
	}
	img, format, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, "", err
	}
	_ = format
	sq := cropSquare(img, CropParams{Zoom: 1})
	out := resizeMax(sq, side)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 80}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/jpeg", nil
}

// AllowedMediaType reports whether the type may be served (§11).
func AllowedMediaType(mt string) bool {
	switch strings.ToLower(strings.TrimSpace(mt)) {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

// SniffMediaType returns a content type from the image header.
func SniffMediaType(data []byte) string {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "image/" + format
	}
}

// PlaceholderSVG returns a deterministic SVG avatar from UID and display name.
func PlaceholderSVG(uid, name string) []byte {
	initials := initialsOf(name, uid)
	bg, fg := colorsOf(uid)
	return []byte(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="0 0 256 256" role="img" aria-label="%s">`+
			`<rect width="256" height="256" fill="%s"/>`+
			`<text x="128" y="128" dy="0.35em" text-anchor="middle" font-family="Georgia, serif" font-size="96" fill="%s">%s</text>`+
			`</svg>`,
		xmlEscape(initials), bg, fg, xmlEscape(initials),
	))
}

func initialsOf(name, uid string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = uid
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '_'
	})
	var b strings.Builder
	for _, p := range parts {
		r, _ := utf8.DecodeRuneInString(p)
		if r == utf8.RuneError {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
		if b.Len() >= 2 {
			break
		}
	}
	if b.Len() == 0 {
		return "?"
	}
	return b.String()
}

func colorsOf(uid string) (bg, fg string) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(uid))
	n := h.Sum32()
	// Muted greens/browns matching the brand palette.
	palette := []string{
		"#4a6b52", "#7fa085", "#6b6355", "#9a9280",
		"#3d5a45", "#5c7a62", "#8a7f6a", "#5a6e58",
	}
	bg = palette[int(n)%len(palette)]
	return bg, "#fffdf7"
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func readOrientation(r io.Reader) int {
	x, err := exif.Decode(r)
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	n, err := tag.Int(0)
	if err != nil {
		return 1
	}
	return n
}

func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return flipHorizontal(img)
	case 3:
		return rotateImage(img, 180)
	case 4:
		return flipVertical(img)
	case 5:
		return rotateImage(flipHorizontal(img), 270)
	case 6:
		return rotateImage(img, 90)
	case 7:
		return rotateImage(flipHorizontal(img), 90)
	case 8:
		return rotateImage(img, 270)
	default:
		return img
	}
}

func rotateImage(img image.Image, degrees int) image.Image {
	degrees = ((degrees % 360) + 360) % 360
	if degrees == 0 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	switch degrees {
	case 90:
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(h-1-y, x, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		return dst
	case 180:
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(w-1-x, h-1-y, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		return dst
	case 270:
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(y, w-1-x, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		return dst
	default:
		return img
	}
}

func flipHorizontal(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func flipVertical(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, h-1-y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func cropSquare(img image.Image, params CropParams) image.Image {
	params = params.normalised()
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	side := math.Min(w, h) / params.Zoom
	if side < 1 {
		side = 1
	}
	cx := w/2 + params.PanX*((w-side)/2)
	cy := h/2 + params.PanY*((h-side)/2)
	x0 := int(math.Round(cx - side/2))
	y0 := int(math.Round(cy - side/2))
	x1 := x0 + int(math.Round(side))
	y1 := y0 + int(math.Round(side))
	if x0 < 0 {
		x1 -= x0
		x0 = 0
	}
	if y0 < 0 {
		y1 -= y0
		y0 = 0
	}
	if x1 > b.Dx() {
		x0 -= x1 - b.Dx()
		x1 = b.Dx()
	}
	if y1 > b.Dy() {
		y0 -= y1 - b.Dy()
		y1 = b.Dy()
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	src := image.Rect(b.Min.X+x0, b.Min.Y+y0, b.Min.X+x1, b.Min.Y+y1)
	dst := image.NewRGBA(image.Rect(0, 0, src.Dx(), src.Dy()))
	draw.Draw(dst, dst.Bounds(), img, src.Min, draw.Src)
	return dst
}

func resizeMax(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSide && h <= maxSide {
		return img
	}
	scale := float64(maxSide) / math.Max(float64(w), float64(h))
	nw := int(math.Max(1, math.Round(float64(w)*scale)))
	nh := int(math.Max(1, math.Round(float64(h)*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			sx := b.Min.X + int(float64(x)/float64(nw)*float64(w))
			sy := b.Min.Y + int(float64(y)/float64(nh)*float64(h))
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	return dst
}

// SolidJPEG is a tiny test helper that builds a JPEG of one colour.
func SolidJPEG(c color.Color, size int) ([]byte, error) {
	if size <= 0 {
		size = 8
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
