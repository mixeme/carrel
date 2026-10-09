package photo

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"carrel.dev/carrel/server/internal/conf"
)

var (
	red  = color.RGBA{220, 20, 20, 255}
	blue = color.RGBA{20, 20, 220, 255}
)

// halves is a w×h image, red on the left half and blue on the right.
func halves(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				img.Set(x, y, red)
			} else {
				img.Set(x, y, blue)
			}
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withEXIF puts an APP1 segment after SOI: IFD0 with the orientation, in the
// byte order given ("II" or "MM"), followed by a GPS-looking string that must
// never reach a card.
func withEXIF(data []byte, order string, o uint16) []byte {
	var bo binary.ByteOrder = binary.LittleEndian
	if order == "MM" {
		bo = binary.BigEndian
	}
	tiff := make([]byte, 8+2+12+4)
	copy(tiff, order)
	bo.PutUint16(tiff[2:], 42)
	bo.PutUint32(tiff[4:], 8)
	bo.PutUint16(tiff[8:], 1)
	bo.PutUint16(tiff[10:], 0x0112)
	bo.PutUint16(tiff[12:], 3)
	bo.PutUint32(tiff[14:], 1)
	bo.PutUint16(tiff[18:], o)
	payload := append(append([]byte("Exif\x00\x00"), tiff...), []byte("GPS 55.7558N 37.6173E")...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	out := append([]byte{}, data[:2]...)
	out = append(append(out, seg...), payload...)
	return append(out, data[2:]...)
}

func decodeJPEG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output is not a JPEG: %v", err)
	}
	return img
}

func isRed(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r>>8 > 150 && g>>8 < 90 && b>>8 < 90
}

func isBlue(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return b>>8 > 150 && r>>8 < 90 && g>>8 < 90
}

func TestMediaType(t *testing.T) {
	for declared, want := range map[string]string{
		"JPEG": "image/jpeg", "jpg": "image/jpeg", "image/jpeg": "image/jpeg",
		"PNG": "image/png", "image/png; charset=binary": "image/png",
		"gif": "image/gif", "image/webp": "image/webp",
	} {
		if got, ok := MediaType(declared); !ok || got != want {
			t.Errorf("MediaType(%q) = %q, %v; want %q", declared, got, ok, want)
		}
	}
	for _, declared := range []string{"", "TIFF", "image/svg+xml", "text/html", "application/octet-stream"} {
		if got, ok := MediaType(declared); ok {
			t.Errorf("MediaType(%q) = %q, want refused", declared, got)
		}
	}
}

func TestOrientationInEitherByteOrder(t *testing.T) {
	plain := encodeJPEG(t, halves(8, 6))
	for _, order := range []string{"II", "MM"} {
		if got := orientation(withEXIF(plain, order, 6)); got != 6 {
			t.Errorf("%s: orientation = %d, want 6", order, got)
		}
	}
	if got := orientation(plain); got != 1 {
		t.Errorf("no EXIF: orientation = %d, want 1", got)
	}
	if got := orientation(withEXIF(plain, "II", 9)); got != 1 {
		t.Errorf("orientation 9 is not a value: got %d, want 1", got)
	}
	broken := withEXIF(plain, "MM", 6)
	broken[4], broken[5] = 0xFF, 0xFF // segment longer than the file
	if got := orientation(broken); got != 1 {
		t.Errorf("broken segment: orientation = %d, want 1", got)
	}
}

// The stored image is red left, blue right. Seen with orientation 6 it is
// turned clockwise, so red is on top; orientation 8 turns it the other way;
// a quarter turn chosen on the server adds to what the camera said.
func TestRenderAppliesOrientationThenTurn(t *testing.T) {
	stored := encodeJPEG(t, halves(80, 60))
	cases := []struct {
		name     string
		o        uint16
		rotate   int
		redAt    image.Point
		blueAt   image.Point
		wantSide int
	}{
		{"as stored", 1, 0, image.Pt(2, 16), image.Pt(29, 16), 32},
		{"orientation 6", 6, 0, image.Pt(16, 2), image.Pt(16, 29), 32},
		{"orientation 8", 8, 0, image.Pt(16, 29), image.Pt(16, 2), 32},
		{"turned 90", 1, 90, image.Pt(16, 2), image.Pt(16, 29), 32},
		{"orientation 6 turned 90", 6, 90, image.Pt(29, 16), image.Pt(2, 16), 32},
		{"turned 180", 1, 180, image.Pt(29, 16), image.Pt(2, 16), 32},
	}
	for _, tc := range cases {
		src, err := inspect(withEXIF(stored, "II", tc.o), 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		out, err := Render(src, Crop{Zoom: 100, Rotate: tc.rotate}, 32, 90)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		img := decodeJPEG(t, out)
		if b := img.Bounds(); b.Dx() != tc.wantSide || b.Dy() != tc.wantSide {
			t.Errorf("%s: %v, want %d×%d", tc.name, b, tc.wantSide, tc.wantSide)
		}
		if c := img.At(tc.redAt.X, tc.redAt.Y); !isRed(c) {
			t.Errorf("%s: at %v %v, want red", tc.name, tc.redAt, c)
		}
		if c := img.At(tc.blueAt.X, tc.blueAt.Y); !isBlue(c) {
			t.Errorf("%s: at %v %v, want blue", tc.name, tc.blueAt, c)
		}
	}
}

func TestRenderZoomAndOffset(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 400; x++ {
			c := blue
			if x < 100 && y < 100 {
				c = red
			}
			img.Set(x, y, c)
		}
	}
	src, err := inspect(encodePNG(t, img), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	// A quarter of the side, pushed to the top left corner, is all red.
	out, err := Render(src, Crop{Zoom: 400, OffsetX: -100, OffsetY: -100}, 50, 90)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeJPEG(t, out)
	for _, p := range []image.Point{{3, 3}, {46, 3}, {3, 46}, {46, 46}, {25, 25}} {
		if c := got.At(p.X, p.Y); !isRed(c) {
			t.Errorf("at %v %v, want red", p, c)
		}
	}
	// Pushed to the bottom right corner, none of it is.
	out, _ = Render(src, Crop{Zoom: 400, OffsetX: 100, OffsetY: 100}, 50, 90)
	if c := decodeJPEG(t, out).At(3, 3); !isBlue(c) {
		t.Errorf("bottom right frame at (3,3) %v, want blue", c)
	}
}

// A 4000-pixel side shrunk to 64 goes through the halving and still lands
// the colours where they belong.
func TestRenderLargeSource(t *testing.T) {
	src, err := inspect(encodePNG(t, halves(4000, 3000)), 100_000_000)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Render(src, DefaultCrop, 64, 90)
	if err != nil {
		t.Fatal(err)
	}
	img := decodeJPEG(t, out)
	if !isRed(img.At(4, 32)) || !isBlue(img.At(59, 32)) {
		t.Errorf("left %v right %v", img.At(4, 32), img.At(59, 32))
	}
}

func TestRenderStripsMetadataAndDoesNotEnlarge(t *testing.T) {
	src, err := inspect(withEXIF(encodeJPEG(t, halves(40, 40)), "MM", 1), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Render(src, DefaultCrop, 512, 85)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Exif")) || bytes.Contains(out, []byte("GPS")) {
		t.Error("metadata of the upload is in the output")
	}
	if b := decodeJPEG(t, out).Bounds(); b.Dx() != 40 || b.Dy() != 40 {
		t.Errorf("a 40 px photo came out %v", b)
	}
}

// pngHeader is a PNG that claims w×h and holds no pixels: DecodeConfig reads
// it, a decoder would fail on it.
func pngHeader(w, h uint32) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1, 1))); err != nil {
		panic(err)
	}
	data := buf.Bytes()
	ihdr := data[8+4 : 8+8+13] // chunk type and data
	binary.BigEndian.PutUint32(ihdr[4:], w)
	binary.BigEndian.PutUint32(ihdr[8:], h)
	binary.BigEndian.PutUint32(data[8+8+13:], crc32.ChecksumIEEE(ihdr))
	return data[:8+8+13+4] // signature and IHDR, nothing after
}

// noise does not compress: its JPEG is mostly scan data, so cutting it cuts
// pixels, not the header.
func noise(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range img.Pix {
		img.Pix[i] = byte(rng.UintN(256))
	}
	return img
}

// setTemp points os.TempDir at a directory of the test's own.
func setTemp(t *testing.T) string {
	dir := t.TempDir()
	for _, k := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(k, dir)
	}
	if os.TempDir() != dir {
		t.Skipf("os.TempDir() = %q, not the directory set", os.TempDir())
	}
	return dir
}

func assertEmpty(t *testing.T, dir, after string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("after %s the temporary file is still there: %v", after, entries[0].Name())
	}
}

func TestLoadRefusesAndLeavesNoFile(t *testing.T) {
	dir := setTemp(t)
	good := encodeJPEG(t, halves(64, 48))

	if _, err := Load(bytes.NewReader(good), int64(len(good))-1, 1<<20); !errors.Is(err, ErrTooLarge) {
		t.Errorf("one byte over the cap: %v, want ErrTooLarge", err)
	}
	assertEmpty(t, dir, "too large")

	// The dimensions are refused before decoding: this PNG has no pixels,
	// and a decoder would have said so instead.
	if _, err := Load(bytes.NewReader(pngHeader(100_000, 100_000)), 1<<20, 100_000_000); !errors.Is(err, ErrTooManyPixels) {
		t.Errorf("100 000 × 100 000: %v, want ErrTooManyPixels", err)
	}
	assertEmpty(t, dir, "too many pixels")

	if _, err := Load(strings.NewReader("<svg xmlns='http://www.w3.org/2000/svg'/>"), 1<<20, 1<<20); !errors.Is(err, ErrFormat) {
		t.Errorf("SVG: %v, want ErrFormat", err)
	}
	assertEmpty(t, dir, "wrong format")

	noisy := encodeJPEG(t, noise(256, 256))
	if _, err := Load(bytes.NewReader(noisy[:len(noisy)*2/3]), 1<<20, 1<<20); !errors.Is(err, ErrUnreadable) {
		t.Errorf("truncated JPEG: %v, want ErrUnreadable", err)
	}
	assertEmpty(t, dir, "truncated")

	src, err := Load(bytes.NewReader(withEXIF(good, "II", 6)), 1<<20, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if src.MediaType != "image/jpeg" || src.Width != 64 || src.Height != 48 || src.Orientation != 6 {
		t.Errorf("source = %s %d×%d orientation %d", src.MediaType, src.Width, src.Height, src.Orientation)
	}
	assertEmpty(t, dir, "success")
}

func TestOpenHoldsTheDeclaredType(t *testing.T) {
	pngData := encodePNG(t, halves(8, 8))
	if _, err := Open(pngData, "PNG", 1<<20); err != nil {
		t.Errorf("PNG declared PNG: %v", err)
	}
	if _, err := Open(pngData, "JPEG", 1<<20); !errors.Is(err, ErrFormat) {
		t.Errorf("PNG declared JPEG: %v, want ErrFormat", err)
	}
	if _, err := Open(pngData, "", 1<<20); !errors.Is(err, ErrFormat) {
		t.Errorf("nothing declared: %v, want ErrFormat", err)
	}
	if _, err := Open(pngData, "PNG", 63); !errors.Is(err, ErrTooManyPixels) {
		t.Errorf("64 px over a cap of 63: %v, want ErrTooManyPixels", err)
	}
	// The smallest lossless WebP: the fourth format reads.
	webp, _ := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if src, err := Open(webp, "image/webp", 1<<20); err != nil || src.Width != 1 {
		t.Errorf("WebP: %+v %v", src, err)
	}
}

func TestFetchTakesOnlyImagesOverHTTP(t *testing.T) {
	pngData := encodePNG(t, halves(8, 8))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/p.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngData)
		case "/page":
			w.Header().Set("Content-Type", "text/html")
			w.Write(pngData)
		case "/lies.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(pngData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx := t.Context()
	if src, err := Fetch(ctx, srv.Client(), srv.URL+"/p.png", 1<<20); err != nil || src.MediaType != "image/png" {
		t.Errorf("image/png: %+v %v", src.MediaType, err)
	}
	if _, err := Fetch(ctx, srv.Client(), srv.URL+"/page", 1<<20); !errors.Is(err, ErrFormat) {
		t.Errorf("text/html: %v, want ErrFormat", err)
	}
	if _, err := Fetch(ctx, srv.Client(), srv.URL+"/lies.jpg", 1<<20); !errors.Is(err, ErrFormat) {
		t.Errorf("PNG sent as image/jpeg: %v, want ErrFormat", err)
	}
	if _, err := Fetch(ctx, srv.Client(), srv.URL+"/missing", 1<<20); err == nil {
		t.Error("404 was taken as a photo")
	}
	for _, link := range []string{"ftp://example.com/p.png", "file:///etc/passwd", "cid:photo", "//example.com/p.png"} {
		if _, err := Fetch(ctx, srv.Client(), link, 1<<20); !errors.Is(err, ErrLink) {
			t.Errorf("%s: %v, want ErrLink", link, err)
		}
	}
}

func TestCropClamp(t *testing.T) {
	got := Crop{Zoom: 50, OffsetX: -300, OffsetY: 300, Rotate: -90}.Clamp()
	want := Crop{Zoom: 100, OffsetX: -100, OffsetY: 100, Rotate: 270}
	if got != want {
		t.Errorf("Clamp = %+v, want %+v", got, want)
	}
	if got := (Crop{Zoom: 9000, Rotate: 45}).Clamp(); got.Zoom != conf.PhotoMaxZoom || got.Rotate != 0 {
		t.Errorf("Clamp = %+v", got)
	}
}

// A file's thumbnail (S-17.1): the whole picture, its proportions kept, the
// orientation applied, never enlarged.
func TestFitKeepsTheWholePicture(t *testing.T) {
	stored := encodeJPEG(t, halves(80, 60))
	cases := []struct {
		name          string
		o             uint16
		side          int
		w, h          int
		redAt, blueAt image.Point
	}{
		{"as stored", 1, 40, 40, 30, image.Pt(3, 15), image.Pt(36, 15)},
		{"orientation 6", 6, 40, 30, 40, image.Pt(15, 3), image.Pt(15, 36)},
		{"orientation 8", 8, 40, 30, 40, image.Pt(15, 36), image.Pt(15, 3)},
		{"smaller than the square", 1, 200, 80, 60, image.Pt(5, 30), image.Pt(75, 30)},
	}
	for _, tc := range cases {
		src, err := inspect(withEXIF(stored, "II", tc.o), 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		out, err := Fit(src, tc.side, 90)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if bytes.Contains(out, []byte("GPS")) {
			t.Errorf("%s: the metadata of the source is in the thumbnail", tc.name)
		}
		img := decodeJPEG(t, out)
		if b := img.Bounds(); b.Dx() != tc.w || b.Dy() != tc.h {
			t.Errorf("%s: %v, want %d×%d", tc.name, b, tc.w, tc.h)
			continue
		}
		if c := img.At(tc.redAt.X, tc.redAt.Y); !isRed(c) {
			t.Errorf("%s: at %v %v, want red", tc.name, tc.redAt, c)
		}
		if c := img.At(tc.blueAt.X, tc.blueAt.Y); !isBlue(c) {
			t.Errorf("%s: at %v %v, want blue", tc.name, tc.blueAt, c)
		}
	}
	// A large source goes through the halvings and keeps its proportions.
	big, err := inspect(encodePNG(t, halves(4000, 1000)), 1<<24)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Fit(big, 480, 85)
	if err != nil {
		t.Fatal(err)
	}
	img := decodeJPEG(t, out)
	if b := img.Bounds(); b.Dx() != 480 || b.Dy() != 120 || !isRed(img.At(20, 60)) || !isBlue(img.At(460, 60)) {
		t.Errorf("a wide picture: %v", img.Bounds())
	}
}

// A file's thumbnail is made from every raster format the module reads —
// BMP and TIFF beside the four (S-17.1) —, by the bytes; a contact's photo
// stays one of the four.
func TestOpenPictureTakesBMPAndTIFFAndAPhotoDoesNot(t *testing.T) {
	var bmpData, tiffData bytes.Buffer
	if err := bmp.Encode(&bmpData, halves(80, 60)); err != nil {
		t.Fatal(err)
	}
	if err := tiff.Encode(&tiffData, halves(80, 60), nil); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		data []byte
		mt   string
	}{
		"bmp":  {bmpData.Bytes(), "image/bmp"},
		"tiff": {tiffData.Bytes(), "image/tiff"},
		"png":  {encodePNG(t, halves(80, 60)), "image/png"},
		"jpeg": {encodeJPEG(t, halves(80, 60)), "image/jpeg"},
	} {
		src, err := OpenPicture(tc.data, 1<<20)
		if err != nil || src.MediaType != tc.mt || src.Width != 80 || src.Height != 60 {
			t.Errorf("%s: %+v %v", name, Source{MediaType: src.MediaType, Width: src.Width, Height: src.Height}, err)
			continue
		}
		out, err := Fit(src, 40, 90)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if img := decodeJPEG(t, out); img.Bounds().Dx() != 40 || img.Bounds().Dy() != 30 || !isRed(img.At(3, 15)) || !isBlue(img.At(36, 15)) {
			t.Errorf("%s: the thumbnail is %v", name, img.Bounds())
		}
	}
	if _, err := OpenPicture(bmpData.Bytes(), 4799); !errors.Is(err, ErrTooManyPixels) {
		t.Errorf("4800 px over a cap of 4799: %v, want ErrTooManyPixels", err)
	}
	if _, err := OpenPicture([]byte("II*\x00 not a picture"), 1<<20); err == nil {
		t.Error("a file that only begins as a TIFF was taken")
	}
	// A card is held to the four.
	if _, err := Open(bmpData.Bytes(), "image/bmp", 1<<20); !errors.Is(err, ErrFormat) {
		t.Errorf("a BMP as a photo: %v, want ErrFormat", err)
	}
	if _, err := Open(tiffData.Bytes(), "image/jpeg", 1<<20); !errors.Is(err, ErrFormat) {
		t.Errorf("a TIFF declared a JPEG as a photo: %v, want ErrFormat", err)
	}
	if _, err := Load(bytes.NewReader(tiffData.Bytes()), 1<<20, 1<<20); !errors.Is(err, ErrFormat) {
		t.Errorf("a TIFF uploaded as a photo: %v, want ErrFormat", err)
	}
}
