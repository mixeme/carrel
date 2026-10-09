// Package photo is the contact photo pipeline (S-13): what an upload may be,
// the frame cut from it, the JPEG that goes into the card, and the fetch of a
// photo a card only links to. It reads bytes and returns bytes. The vCard
// property is the card provider's; pages, drafts and the patch are web's.
package photo

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // GIF is one of the four formats taken in (S-13)
	_ "image/jpeg" // JPEG is read and written
	_ "image/png"
	"io"
	"os"
	"strings"

	_ "golang.org/x/image/bmp"  // BMP and TIFF: a file's thumbnail only (S-17.1)
	_ "golang.org/x/image/tiff" // never a contact's photo
	_ "golang.org/x/image/webp"

	"carrel.dev/carrel/server/internal/conf"
)

var (
	// ErrFormat: not a JPEG, PNG, GIF or WebP image, or not the format the
	// card or the far server declared for it.
	ErrFormat = errors.New("photo: not a JPEG, PNG, GIF or WebP image")
	// ErrTooManyPixels: the dimensions, read before decoding, exceed
	// CARREL_PHOTO_MAX_PIXELS. A byte cap does not stop a compression bomb.
	ErrTooManyPixels = errors.New("photo: more pixels than CARREL_PHOTO_MAX_PIXELS")
	// ErrTooLarge: the upload is longer than CARREL_PHOTO_MAX_UPLOAD_BYTES.
	ErrTooLarge = errors.New("photo: larger than CARREL_PHOTO_MAX_UPLOAD_BYTES")
	// ErrUnreadable: the header was fine, the image itself does not decode.
	ErrUnreadable = errors.New("photo: the image does not decode")
)

// formats maps the decoder's format name to the media type served for it.
var formats = map[string]string{
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"gif":  "image/gif",
	"webp": "image/webp",
}

// MediaType reads a declared type — vCard TYPE=JPEG, MEDIATYPE=image/jpeg, a
// data: URI's type, or an HTTP Content-Type — as one of the four media types.
// It names what was declared; it never looks at the bytes. JPEG in a vCard is
// not a valid HTTP type, image/jpeg is.
func MediaType(declared string) (string, bool) {
	t := strings.ToLower(strings.TrimSpace(declared))
	if i := strings.IndexByte(t, ';'); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	t = strings.TrimPrefix(t, "image/")
	if t == "jpg" || t == "pjpeg" {
		t = "jpeg"
	}
	mt, ok := formats[t]
	return mt, ok
}

// pictures are the formats a file's thumbnail is made from (S-17.1): the
// four a photo may be, and BMP and TIFF — every raster format this module
// already has a decoder for (the owner's decision of 09.10.2026).
var pictures = map[string]string{
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"gif":  "image/gif",
	"webp": "image/webp",
	"bmp":  "image/bmp",
	"tiff": "image/tiff",
}

// OpenPicture checks a file that is in hand for its thumbnail: one of the
// pictures by its bytes, whatever its name says, within the pixel cap. It is
// not how a photo is taken in — Open and Load hold a card to four formats.
func OpenPicture(data []byte, maxPixels int64) (Source, error) {
	cfg, mt, err := config(bytes.NewReader(data), maxPixels, pictures)
	if err != nil {
		return Source{}, err
	}
	src := Source{Data: data, MediaType: mt, Width: cfg.Width, Height: cfg.Height, Orientation: 1}
	switch mt {
	case "image/jpeg":
		src.Orientation = orientation(data)
	case "image/tiff":
		src.Orientation = tiffOrientation(data)
	}
	return src, nil
}

// Source is an image checked for format and size, not yet decoded.
type Source struct {
	Data          []byte
	MediaType     string
	Width, Height int // as stored, before the orientation
	Orientation   int // EXIF 1…8; 1 when there is none
}

// Load takes an upload. The body goes to a temporary file, not to memory, and
// the file is removed whatever happens. The dimensions are read from the
// header before anything is decoded; only an image within both caps is read
// into memory, and it has to decode whole.
func Load(r io.Reader, maxBytes, maxPixels int64) (Source, error) {
	f, err := os.CreateTemp("", "carrel-photo-*")
	if err != nil {
		return Source{}, err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	n, err := io.Copy(f, io.LimitReader(r, maxBytes+1))
	if err != nil {
		return Source{}, err
	}
	if n > maxBytes {
		return Source{}, ErrTooLarge
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Source{}, err
	}
	if _, _, err := config(bufio.NewReader(f), maxPixels, formats); err != nil {
		return Source{}, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Source{}, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return Source{}, err
	}
	src, err := inspect(data, maxPixels)
	if err != nil {
		return Source{}, err
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return Source{}, ErrUnreadable
	}
	return src, nil
}

// Open checks a photo that is already in hand: one kept in a card, or one a
// far server sent. Its bytes have to be the format it was declared as.
func Open(data []byte, declared string, maxPixels int64) (Source, error) {
	mt, ok := MediaType(declared)
	if !ok {
		return Source{}, fmt.Errorf("%w: declared as %q", ErrFormat, declared)
	}
	src, err := inspect(data, maxPixels)
	if err != nil {
		return Source{}, err
	}
	if src.MediaType != mt {
		return Source{}, fmt.Errorf("%w: declared as %s, the bytes are %s", ErrFormat, mt, src.MediaType)
	}
	return src, nil
}

func inspect(data []byte, maxPixels int64) (Source, error) {
	cfg, mt, err := config(bytes.NewReader(data), maxPixels, formats)
	if err != nil {
		return Source{}, err
	}
	src := Source{Data: data, MediaType: mt, Width: cfg.Width, Height: cfg.Height, Orientation: 1}
	if mt == "image/jpeg" {
		src.Orientation = orientation(data)
	}
	return src, nil
}

func config(r io.Reader, maxPixels int64, formats map[string]string) (image.Config, string, error) {
	cfg, format, err := image.DecodeConfig(r)
	if err != nil {
		return image.Config{}, "", ErrFormat
	}
	mt, ok := formats[format]
	if !ok {
		return image.Config{}, "", ErrFormat
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return image.Config{}, "", ErrUnreadable
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return image.Config{}, "", ErrTooManyPixels
	}
	return cfg, mt, nil
}

// Crop is the frame, chosen on the server in steps (S-13). Zoom is in percent
// of the largest square that fits: 100 is that square, conf.PhotoMaxZoom the
// closest. OffsetX and OffsetY are in percent of the room left on each side:
// -100 is the left or top edge, 0 the centre, 100 the right or bottom edge.
// Rotate is a quarter turn clockwise: 0, 90, 180 or 270.
type Crop struct {
	Zoom    int
	OffsetX int
	OffsetY int
	Rotate  int
}

// DefaultCrop is the square in the centre.
var DefaultCrop = Crop{Zoom: 100}

// Clamp keeps a frame inside the image: zoom and offsets to their ranges, a
// rotation that is not a quarter turn to none.
func (c Crop) Clamp() Crop {
	c.Zoom = clamp(c.Zoom, 100, conf.PhotoMaxZoom)
	c.OffsetX = clamp(c.OffsetX, -100, 100)
	c.OffsetY = clamp(c.OffsetY, -100, 100)
	c.Rotate = ((c.Rotate % 360) + 360) % 360
	if c.Rotate%90 != 0 {
		c.Rotate = 0
	}
	return c
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
