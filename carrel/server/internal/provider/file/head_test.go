package file

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/davtest"
)

// docs/servers.md, SV-57: the one PROPFIND of a folder asks for what a
// server says of each folder inside it — Nextcloud the folders and the files
// it holds, the draft of collection properties DAV:childcount — and the
// entry of a folder carries how many members it has. A server that says
// nothing leaves it unknown, not 0: «0 items» would be a lie.
func TestListTakesWhatAFolderHoldsFromTheSameAnswer(t *testing.T) {
	for _, tc := range []struct {
		name   string
		nc, cc bool
		said   bool
	}{
		{"nextcloud", true, false, true},
		{"childcount", false, true, true},
		{"neither", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, p := davFiles(t)
			srv.FolderCounts, srv.ChildCount = tc.nc, tc.cc
			docs := davtest.FileRoot + "docs/"
			srv.SeedFile(docs+"sub/a.md", "a")
			srv.SeedFile(docs+"sub/b.md", "b")
			srv.SeedFolder(docs + "sub/inner/")
			srv.SeedFolder(docs + "empty/")
			srv.SeedFile(docs+"c.txt", "c")
			entries, err := p.List(t.Context(), docs)
			if err != nil {
				t.Fatal(err)
			}
			byName := map[string]Entry{}
			for _, e := range entries {
				byName[e.Name] = e
			}
			sub, empty, c := byName["sub"], byName["empty"], byName["c.txt"]
			if !tc.said {
				if sub.Items != nil || empty.Items != nil {
					t.Errorf("a server that says nothing, and the folders hold %v, %v", sub.Items, empty.Items)
				}
			} else if sub.Items == nil || *sub.Items != 3 || empty.Items == nil || *empty.Items != 0 {
				t.Errorf("sub holds %v, empty holds %v; want 3 and 0", sub.Items, empty.Items)
			}
			if c.Items != nil {
				t.Errorf("a file holds %v", *c.Items)
			}
			if n := srv.CountRequests("PROPFIND", davtest.FileRoot); n != 1 {
				t.Errorf("PROPFIND requests = %d, want the one of the folder", n)
			}
		})
	}
}

// S-17: the size of an image is read from its header within the first
// conf.ImageHeadBytes — PNG's IHDR, JPEG's SOFn past an Exif segment, GIF's
// screen, WebP's VP8, VP8L and VP8X, BMP's header, TIFF's directory — and
// from nothing else.
func TestImageSizeReadsTheHeader(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		w, h int
		ok   bool
	}{
		{"png", encoded(t, png.Encode, 300, 200), 300, 200, true},
		{"gif", gifOf(t, 7, 5), 7, 5, true},
		{"jpeg", jpegOf(t, 640, 480, 0), 640, 480, true},
		{"jpeg after 20 KB of Exif", jpegOf(t, 4032/8, 3024/8, 20000), 4032 / 8, 3024 / 8, true},
		{"jpeg whose frame is past the cap", jpegOf(t, 64, 48, conf.ImageHeadBytes+1000), 0, 0, false},
		// The frame header within the cap and the scan past it: image/jpeg
		// would read on to the scan and find nothing.
		{"jpeg frame header alone", append([]byte("\xff\xd8\xff\xc2\x00\x11\x08\x0b\xd0\x0f\xc0\x03\x01\x22\x00\x02\x11\x01\x03\x11\x01"),
			make([]byte, conf.ImageHeadBytes)...), 4032, 3024, true},
		{"webp VP8", webpLossy(4032, 3024), 4032, 3024, true},
		{"webp VP8L", webpLossless(1920, 1080), 1920, 1080, true},
		{"webp VP8X", webpExtended(16000, 9), 16000, 9, true},
		{"bmp", encoded(t, bmp.Encode, 31, 17), 31, 17, true},
		{"tiff", encoded(t, func(w io.Writer, m image.Image) error { return tiff.Encode(w, m, nil) }, 640, 20), 640, 20, true},
		// 200 by 200 uncompressed is 160 KB of pixels before the directory.
		{"tiff whose directory is past the cap", encoded(t, func(w io.Writer, m image.Image) error { return tiff.Encode(w, m, nil) }, 200, 200), 0, 0, false},
		{"text", []byte("Заглушка файла из кадров"), 0, 0, false},
		{"png cut short", encoded(t, png.Encode, 3, 3)[:12], 0, 0, false},
		{"nothing", nil, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, h, ok := ImageSize(bytes.NewReader(tc.data))
			if w != tc.w || h != tc.h || ok != tc.ok {
				t.Errorf("ImageSize = %d×%d %v, want %d×%d %v", w, h, ok, tc.w, tc.h, tc.ok)
			}
		})
	}
}

// A server that ignores Range sends the whole file: the reader stops at the
// cap all the same, whatever follows (S-17: never a whole file for this).
func TestImageSizeReadsNoFurtherThanTheCap(t *testing.T) {
	body := &counted{r: io.MultiReader(strings.NewReader("\xff\xd8\xff\xe1"), strings.NewReader(strings.Repeat("x", 20<<20)))}
	if _, _, ok := ImageSize(body); ok {
		t.Error("a size out of no header")
	}
	if body.n > conf.ImageHeadBytes {
		t.Errorf("read %d bytes of a file, the cap is %d", body.n, conf.ImageHeadBytes)
	}
}

// One GET with Range: bytes=0-65535 per image, and the size from it; a
// server that answers 200 with the whole file is read to the cap and no
// further, and an image whose header lies past the cap has no size — not an
// error.
func TestDimensionsAskForTheHeadOnly(t *testing.T) {
	for _, whole := range []bool{false, true} {
		srv, p := davFiles(t)
		srv.FilesIgnoreRange = whole
		near := davtest.FileRoot + "near.jpg"
		far := davtest.FileRoot + "far.jpg"
		srv.SeedFile(near, string(jpegOf(t, 640, 480, 20000))+strings.Repeat(".", 3<<20))
		srv.SeedFile(far, string(jpegOf(t, 640, 480, conf.ImageHeadBytes))+strings.Repeat(".", 3<<20))
		w, h, ok, err := p.Dimensions(t.Context(), near)
		if err != nil || !ok || w != 640 || h != 480 {
			t.Errorf("whole=%v: near = %d×%d %v %v", whole, w, h, ok, err)
		}
		if _, _, ok, err := p.Dimensions(t.Context(), far); err != nil || ok {
			t.Errorf("whole=%v: far = %v %v, want no size and no error", whole, ok, err)
		}
		if got := srv.Ranges(); len(got) != 2 || got[0] != "bytes=0-65535" || got[1] != "bytes=0-65535" {
			t.Errorf("whole=%v: ranges asked = %q", whole, got)
		}
	}
}

type counted struct {
	r io.Reader
	n int
}

func (c *counted) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

func encoded(t *testing.T, enc func(io.Writer, image.Image) error, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := enc(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func gifOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := gif.Encode(&b, image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{color.Black}), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// jpegOf is a JPEG with an APP1 Exif segment of exif bytes before its frame
// header, as a camera writes one (0 — none).
func jpegOf(t *testing.T, w, h, exif int) []byte {
	t.Helper()
	raw := encoded(t, func(wr io.Writer, m image.Image) error { return jpeg.Encode(wr, m, nil) }, w, h)
	if exif == 0 {
		return raw
	}
	var out []byte
	out = append(out, raw[:2]...)
	for exif > 0 {
		n := min(exif, 65533-6)
		payload := append([]byte("Exif\x00\x00"), make([]byte, n)...)
		out = append(out, 0xFF, 0xE1)
		out = binary.BigEndian.AppendUint16(out, uint16(len(payload)+2))
		out = append(out, payload...)
		exif -= n
	}
	return append(out, raw[2:]...)
}

func riff(chunk string, data []byte) []byte {
	var b []byte
	b = append(b, "RIFF"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(4+8+len(data)))
	b = append(b, "WEBP"...)
	b = append(b, chunk...)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(data)))
	return append(b, data...)
}

func webpLossy(w, h int) []byte {
	d := []byte{0x50, 0x02, 0x00, 0x9d, 0x01, 0x2a}
	d = binary.LittleEndian.AppendUint16(d, uint16(w))
	d = binary.LittleEndian.AppendUint16(d, uint16(h))
	return riff("VP8 ", append(d, make([]byte, 16)...))
}

func webpLossless(w, h int) []byte {
	d := []byte{0x2f}
	d = binary.LittleEndian.AppendUint32(d, uint32(w-1)|uint32(h-1)<<14)
	return riff("VP8L", append(d, make([]byte, 16)...))
}

func webpExtended(w, h int) []byte {
	d := []byte{0x10, 0, 0, 0, byte(w - 1), byte((w - 1) >> 8), byte((w - 1) >> 16), byte(h - 1), byte((h - 1) >> 8), byte((h - 1) >> 16)}
	return riff("VP8X", d)
}
