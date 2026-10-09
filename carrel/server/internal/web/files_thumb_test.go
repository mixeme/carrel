package web_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/view"
)

// The owner's decision (said again on 09.10.2026): the thumbnail of a file is
// made by the server — the original does not travel for it — and it is made
// for a picture of up to 16 MiB (S-17.1; s-files-5 draws pictures of 640 KB,
// 712 KB and 2.8 MB as thumbnails, and «too large» on a TIFF of 84 MB). The row and the panel name the address
// of the thumbnail; the answer is a small JPEG that fits a square of
// conf.FileThumbSide, kept by the version of the file; a picture over the
// ceiling has none and says «too large»; one that does not decode answers
// 404, and its row keeps the icon.
func TestFileThumbnailIsMadeByTheServer(t *testing.T) {
	f := filesFixture(t, true)
	noise := image.NewRGBA(image.Rect(0, 0, 1600, 1200))
	rand.New(rand.NewSource(1)).Read(noise.Pix)
	photo := jpegOf(t, noise)
	if len(photo) < 1<<20 {
		t.Fatalf("the picture of the check is %d bytes, want over a megabyte", len(photo))
	}
	f.srv.SeedFile(filesDocs+"photo.jpg", string(photo))
	f.srv.SeedFile(filesDocs+"huge.png", "png")
	f.srv.Grow(filesDocs+"huge.png", conf.FileThumbMaxBytes+1)
	f.srv.SeedFile(filesDocs+"edge.png", "png")
	f.srv.Grow(filesDocs+"edge.png", conf.FileThumbMaxBytes)
	f.srv.SeedFile(filesDocs+"broken.png", "not a picture")
	var scan, chart bytes.Buffer
	if err := tiff.Encode(&scan, noise.SubImage(image.Rect(0, 0, 800, 200)), nil); err != nil {
		t.Fatal(err)
	}
	if err := bmp.Encode(&chart, noise.SubImage(image.Rect(0, 0, 200, 400))); err != nil {
		t.Fatal(err)
	}
	f.srv.SeedFile(filesDocs+"scan.tif", scan.String())
	f.srv.SeedFile(filesDocs+"chart.bmp", chart.String())
	f.srv.SeedFile(filesDocs+"panorama.tiff", "tiff")
	f.srv.Grow(filesDocs+"panorama.tiff", conf.FileThumbMaxBytes+1)
	f.srv.SeedFile(filesDocs+"drawing.svg", "<svg xmlns='http://www.w3.org/2000/svg'/>")

	page := decode[view.FilesPage](t, f.h.do(get(f.at("")+"?select=photo.jpg", f.sess)))
	row := fileRow(t, page, "photo.jpg")
	if !strings.HasPrefix(row.Thumb, f.at("download", "photo.jpg")+"?") || !strings.Contains(row.Thumb, "thumb=1") || strings.Contains(row.Thumb, "inline") || row.Note != "" {
		t.Fatalf("a picture of %d bytes: thumb %q, note %q", len(photo), row.Thumb, row.Note)
	}
	if d := page.Details; d == nil || d.Preview == nil || d.Preview.Href != row.Thumb {
		t.Errorf("the panel's preview = %+v, want the row's thumbnail %q", d, row.Thumb)
	}
	if huge := fileRow(t, page, "huge.png"); huge.Thumb != "" || huge.Note != "too large" {
		t.Errorf("a picture over the ceiling: thumb %q, note %q", huge.Thumb, huge.Note)
	}
	if edge := fileRow(t, page, "edge.png"); edge.Thumb == "" || edge.Note != "" {
		t.Errorf("a picture of the ceiling exactly: thumb %q, note %q", edge.Thumb, edge.Note)
	}
	// Every raster format the server reads without a dependency of its own
	// (the owner's decision of 09.10.2026): a TIFF and a BMP too, which no
	// browser shows — the thumbnail is a JPEG all the same.
	for name, want := range map[string]image.Point{"scan.tif": {X: conf.FileThumbSide, Y: conf.FileThumbSide / 4}, "chart.bmp": {X: 200, Y: 400}} {
		one := fileRow(t, page, name)
		if !strings.Contains(one.Thumb, "thumb=1") || one.Note != "" || one.Icon != "i-img" {
			t.Errorf("%s: thumb %q, note %q, icon %q", name, one.Thumb, one.Note, one.Icon)
			continue
		}
		rec := f.h.do(get(one.Thumb, f.sess))
		img, err := jpeg.Decode(bytes.NewReader(rec.Body.Bytes()))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" || err != nil || img.Bounds().Size() != want {
			t.Errorf("%s: %d %s %v, want a JPEG of %v", name, rec.Code, rec.Header().Get("Content-Type"), err, want)
		}
	}
	if pano := fileRow(t, page, "panorama.tiff"); pano.Thumb != "" || pano.Note != "too large" {
		t.Errorf("a TIFF over the ceiling: thumb %q, note %q", pano.Thumb, pano.Note)
	}
	if svg := fileRow(t, page, "drawing.svg"); svg.Thumb != "" || svg.Note != "" {
		t.Errorf("a drawing the server has no decoder for: thumb %q, note %q", svg.Thumb, svg.Note)
	}
	// The width and height of the one chosen are read from its head.
	if dims := fileRow(t, decode[view.FilesPage](t, f.h.do(get(f.at("")+"?select=chart.bmp", f.sess))), "chart.bmp").Dimensions; dims != "200×400" {
		t.Errorf("a BMP chosen says %q", dims)
	}

	before := f.srv.CountRequests(http.MethodGet, filesDocs+"photo.jpg")
	rec := f.h.do(get(row.Thumb, f.sess))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" ||
		!strings.HasPrefix(rec.Header().Get("Content-Disposition"), "inline") || rec.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("thumbnail: %d, headers %v", rec.Code, rec.Header())
	}
	small, err := jpeg.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("the thumbnail is not a JPEG: %v", err)
	}
	if b := small.Bounds(); b.Dx() != conf.FileThumbSide || b.Dy() != conf.FileThumbSide*3/4 {
		t.Errorf("the thumbnail is %v, want %d by %d", b, conf.FileThumbSide, conf.FileThumbSide*3/4)
	}
	if rec.Body.Len() >= len(photo)/4 {
		t.Errorf("the thumbnail is %d bytes of a picture of %d", rec.Body.Len(), len(photo))
	}
	// Made once: the next look at the same version reads nothing.
	if again := f.h.do(get(row.Thumb, f.sess)); again.Code != http.StatusOK || again.Body.Len() != rec.Body.Len() {
		t.Errorf("the thumbnail asked again: %d, %d bytes", again.Code, again.Body.Len())
	}
	if n := f.srv.CountRequests(http.MethodGet, filesDocs+"photo.jpg") - before; n != 1 {
		t.Errorf("the picture was read %d times for two looks at its thumbnail, want 1", n)
	}
	// The address names the version of the file, so the browser keeps the
	// thumbnail and does not ask again: the thumbnails of a folder outlive
	// the session on the person's own machine.
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "private") || !strings.Contains(cc, "immutable") || !strings.Contains(cc, "max-age=31536000") {
		t.Errorf("Cache-Control of a thumbnail = %q", cc)
	}
	// And the copy it holds stands without anything being read or made —
	// here for a picture this session never made a thumbnail of.
	edge := fileRow(t, page, "edge.png").Thumb
	known := get(edge, f.sess)
	known.Header.Set("If-None-Match", `"t-`+thumbVersion(t, edge)+`"`)
	if rec := f.h.do(known); rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("a thumbnail the browser holds: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if n := f.srv.CountRequests(http.MethodGet, filesDocs+"edge.png"); n != 0 {
		t.Errorf("a picture was read %d times to say its thumbnail has not changed", n)
	}
	if tag := rec.Header().Get("ETag"); tag != `"t-`+thumbVersion(t, row.Thumb)+`"` {
		t.Errorf("ETag %q of the thumbnail at %q", tag, row.Thumb)
	}
	// An address without the version, or with another, is not kept: the
	// thumbnail is of the file as it is now.
	for _, at := range []string{f.at("download", "photo.jpg") + "?thumb=1", f.at("download", "photo.jpg") + "?thumb=1&v=stale"} {
		loose := f.h.do(get(at, f.sess))
		if loose.Code != http.StatusOK || strings.Contains(loose.Header().Get("Cache-Control"), "immutable") {
			t.Errorf("%s: %d, Cache-Control %q", at, loose.Code, loose.Header().Get("Cache-Control"))
		}
	}

	// A file that only says it is a picture: no thumbnail, and it is not
	// read again for the asking.
	broken := fileRow(t, page, "broken.png").Thumb
	if rec := f.h.do(get(broken, f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("a picture that does not decode: %d", rec.Code)
	}
	f.h.do(get(broken, f.sess))
	if n := f.srv.CountRequests(http.MethodGet, filesDocs+"broken.png"); n != 1 {
		t.Errorf("a picture that does not decode was read %d times, want 1", n)
	}
	// Not a picture: there is no thumbnail to ask for.
	f.srv.SeedFile(filesDocs+"notes.txt", "text")
	if rec := f.h.do(get(f.at("download", "notes.txt")+"?thumb=1", f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("the thumbnail of a text: %d", rec.Code)
	}
	// The original is still what Open and Download give.
	if rec := f.h.do(get(f.at("download", "photo.jpg")+"?inline=1", f.sess)); rec.Code != http.StatusOK || rec.Body.Len() != len(photo) {
		t.Errorf("the original shown in place: %d, %d bytes", rec.Code, rec.Body.Len())
	}
}

// thumbVersion is the version of the file an address of a thumbnail names.
func thumbVersion(t *testing.T, href string) string {
	t.Helper()
	u, err := url.Parse(href)
	if err != nil || u.Query().Get("v") == "" {
		t.Fatalf("the thumbnail's address %q names no version", href)
	}
	return u.Query().Get("v")
}
