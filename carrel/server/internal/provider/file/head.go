package file

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/gif"
	"image/png"
	"io"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
)

// Dimensions reads the head of an image and says its width and height
// (S-17): one GET with Range: bytes=0-(conf.ImageHeadBytes-1), as an ordinary
// request — CARREL_DAV_REQUEST_TIMEOUT counts the whole exchange. No server
// of the lineup gives the size in its PROPFIND (docs/servers.md, SV-57), so
// this is how the one file chosen learns it. A server that ignores the range
// and sends the whole file is read no further than the cap either: the
// answer is closed there. ok is false when the header does not say within
// those bytes, or the type is not one of PNG, JPEG, GIF, WebP, BMP, TIFF;
// that is not an error.
func (p *Provider) Dimensions(ctx context.Context, path string) (w, h int, ok bool, err error) {
	body, _, err := p.t.Get(ctx, path, &dav.Range{Start: 0, End: conf.ImageHeadBytes - 1})
	if err != nil {
		return 0, 0, false, err
	}
	defer body.Close()
	w, h, ok = ImageSize(body)
	return w, h, ok, nil
}

// ImageSize reads at most conf.ImageHeadBytes of r and says the width and
// height its header gives: PNG's IHDR, JPEG's frame header past whatever
// APPn segments (Exif) stand before it, GIF's logical screen, WebP's VP8,
// VP8L or VP8X chunk, BMP's header, TIFF's first directory. The decoders of
// image/png, image/gif and of golang.org/x/image for BMP and TIFF read their
// header out of the bytes already read, never out of r. JPEG is walked here:
// image/jpeg reads on to the scan, which may lie past the cap when the frame
// header does not.
func ImageSize(r io.Reader) (w, h int, ok bool) {
	head, err := io.ReadAll(io.LimitReader(r, conf.ImageHeadBytes))
	if err != nil && len(head) == 0 {
		return 0, 0, false
	}
	var cfg image.Config
	switch {
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		cfg, err = png.DecodeConfig(bytes.NewReader(head))
	case bytes.HasPrefix(head, []byte("\xff\xd8")):
		w, h = jpegSize(head)
		return w, h, w > 0 && h > 0
	case bytes.HasPrefix(head, []byte("GIF87a")), bytes.HasPrefix(head, []byte("GIF89a")):
		cfg, err = gif.DecodeConfig(bytes.NewReader(head))
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		w, h = webpSize(head)
		return w, h, w > 0 && h > 0
	case bytes.HasPrefix(head, []byte("BM")):
		cfg, err = bmp.DecodeConfig(bytes.NewReader(head))
	case bytes.HasPrefix(head, []byte("II*\x00")), bytes.HasPrefix(head, []byte("MM\x00*")):
		// A TIFF keeps its directory wherever its writer put it: one that
		// lies past the cap says nothing here.
		cfg, err = tiff.DecodeConfig(bytes.NewReader(head))
	default:
		return 0, 0, false
	}
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

// jpegSize walks the segments of a JPEG to its frame header (SOF0…SOF15
// but DHT, JPG and DAC; ITU T.81 B.1.1.3) and gives its width and height:
// every segment before it — APP1 with Exif, a thumbnail and all — is
// stepped over by its length. A scan or the end of the bytes first: none.
func jpegSize(b []byte) (w, h int) {
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xff {
			return 0, 0
		}
		marker := b[i+1]
		switch {
		case marker == 0xff: // a fill byte
			i++
			continue
		case marker == 0x01 || marker >= 0xd0 && marker <= 0xd7: // TEM, RSTn: no length
			i += 2
			continue
		case marker == 0xd9 || marker == 0xda: // EOI, SOS
			return 0, 0
		}
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		if n < 2 {
			return 0, 0
		}
		if marker >= 0xc0 && marker <= 0xcf && marker != 0xc4 && marker != 0xc8 && marker != 0xcc {
			if i+9 > len(b) {
				return 0, 0
			}
			return int(binary.BigEndian.Uint16(b[i+7:])), int(binary.BigEndian.Uint16(b[i+5:]))
		}
		i += 2 + n
	}
	return 0, 0
}

// webpSize is the canvas of a WebP file from its first chunk (RFC 9649):
// VP8 — the frame header of a key frame, 14 bits each; VP8L — one less
// than each, 14 bits after the signature byte; VP8X — one less than each,
// 24 bits after the flags.
func webpSize(b []byte) (w, h int) {
	if len(b) < 30 {
		return 0, 0
	}
	data := b[20:]
	switch string(b[12:16]) {
	case "VP8 ":
		if data[3] != 0x9d || data[4] != 0x01 || data[5] != 0x2a {
			return 0, 0
		}
		return int(binary.LittleEndian.Uint16(data[6:]) & 0x3fff), int(binary.LittleEndian.Uint16(data[8:]) & 0x3fff)
	case "VP8L":
		if data[0] != 0x2f {
			return 0, 0
		}
		bits := binary.LittleEndian.Uint32(data[1:])
		return int(bits&0x3fff) + 1, int(bits>>14&0x3fff) + 1
	case "VP8X":
		u24 := func(x []byte) int { return int(x[0]) | int(x[1])<<8 | int(x[2])<<16 }
		return u24(data[4:]) + 1, u24(data[7:]) + 1
	}
	return 0, 0
}
