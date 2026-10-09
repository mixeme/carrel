package web_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"math/rand"
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/view"
)

// The owner's decision (said again on 09.10.2026): the thumbnail of a file is
// made by the server — the original does not travel for it — and it is made
// for a picture of up to 16 MiB (S-17.1; s-files-5 draws pictures of 640 KB,
// 712 KB and 2.8 MB as thumbnails). The row and the panel name the address
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
	f.srv.SeedFile(filesDocs+"scan.tif", "tiff")

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
	if tif := fileRow(t, page, "scan.tif"); tif.Thumb != "" || tif.Note != "" {
		t.Errorf("a picture no browser shows: thumb %q, note %q", tif.Thumb, tif.Note)
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
	// The browser's copy of this version stands.
	known := get(row.Thumb, f.sess)
	known.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	if rec.Header().Get("ETag") == "" || f.h.do(known).Code != http.StatusNotModified {
		t.Errorf("ETag %q, and the same asked with it is not 304", rec.Header().Get("ETag"))
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
