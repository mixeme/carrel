package web_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/view"
)

// ФБ11 (решение владельца 03.10.2026): «folder · 34 items» and «12 items»
// of the frames (s-files-1, -5) without a request per row. The folder's one
// PROPFIND says what each folder inside holds where its server does
// (Nextcloud, docs/servers.md SV-57); elsewhere a folder this session has
// read already is counted from what was read; any other folder has no count
// — not «0 items».
func TestFolderRowsSayWhatTheyHold(t *testing.T) {
	seed := func(f davFixture) {
		f.srv.SeedFile(filesDocs+"2026-07/a.md", "a")
		f.srv.SeedFile(filesDocs+"2026-07/b.md", "b")
		f.srv.SeedFolder(filesDocs + "2026-07/inner/")
		f.srv.SeedFile(filesDocs+"one/x.md", "x")
		f.srv.SeedFolder(filesDocs + "empty/")
		f.srv.SeedFile(filesDocs+"c.txt", "c")
	}

	nc := filesFixture(t, true)
	nc.srv.FolderCounts = true
	seed(nc)
	page := decode[view.FilesPage](t, nc.h.do(get(nc.at(""), nc.sess)))
	for name, want := range map[string]view.Text{"2026-07": "3 items", "one": "1 item", "empty": "0 items", "c.txt": ""} {
		if got := fileRow(t, page, name).Items; got != want {
			t.Errorf("Nextcloud: %s holds %q, want %q", name, got, want)
		}
	}
	for _, sub := range []string{"2026-07/", "one/", "empty/"} {
		if n := nc.srv.CountRequests("PROPFIND", filesDocs+sub); n != 0 {
			t.Errorf("Nextcloud: %d PROPFIND of %s for its row", n, sub)
		}
	}

	plain := filesFixture(t, true)
	seed(plain)
	page = decode[view.FilesPage](t, plain.h.do(get(plain.at(""), plain.sess)))
	for _, name := range []string{"2026-07", "one", "empty"} {
		if got := fileRow(t, page, name).Items; got != "" {
			t.Errorf("a server that says nothing: %s holds %q", name, got)
		}
	}
	// The person opens «one» and comes back: what was read counts it, and
	// the parent is read again with its one PROPFIND, nothing more.
	if rec := plain.h.do(get(plain.at("", "one", ""), plain.sess)); rec.Code != 200 {
		t.Fatalf("one: %d", rec.Code)
	}
	page = decode[view.FilesPage](t, plain.h.do(get(plain.at(""), plain.sess)))
	if got := fileRow(t, page, "one").Items; got != "1 item" {
		t.Errorf("a folder this session read holds %q, want «1 item»", got)
	}
	if got := fileRow(t, page, "2026-07").Items; got != "" {
		t.Errorf("a folder nobody read holds %q", got)
	}
	if n := plain.srv.CountRequests("PROPFIND", filesDocs+"2026-07/"); n != 0 {
		t.Errorf("%d PROPFIND of a folder for its row", n)
	}
}

// ФБ11: «4032×3024» of the chosen tile (s-files-5) and of its panel. The
// PROPFIND of the lineup gives no size of an image (docs/servers.md,
// SV-57), so the one file chosen is read once from its head — Range:
// bytes=0-65535 — and the size goes to its row and to its panel; another
// view of the same version reads nothing. A server that sends the whole
// file is read to the cap; a header past it leaves no size, and the panel
// stands.
func TestChosenImageSaysItsSize(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"board.jpg", string(cameraJPEG(t, 640, 480, 20000))+strings.Repeat(".", 3<<20))
	f.srv.SeedFile(filesDocs+"other.jpg", string(cameraJPEG(t, 64, 48, 0)))
	page := decode[view.FilesPage](t, f.h.do(get(f.at("")+"?select=board.jpg", f.sess)))
	if got := fileRow(t, page, "board.jpg").Dimensions; got != "640×480" {
		t.Errorf("chosen row = %q, want 640×480", got)
	}
	if got := fileRow(t, page, "other.jpg").Dimensions; got != "" {
		t.Errorf("a row not chosen = %q", got)
	}
	if page.Details == nil {
		t.Fatal("no panel")
	}
	var terms []string
	for _, fl := range page.Details.Fields {
		terms = append(terms, string(fl.Label))
	}
	if strings.Join(terms, ",") != "Size,Type,Dimensions,Changed,Path" || page.Details.Fields[2].Value != "640×480" || !page.Details.Fields[2].Mono {
		t.Errorf("panel fields = %+v", page.Details.Fields)
	}
	if got := f.srv.Ranges(); len(got) != 1 || got[0] != "bytes=0-65535" {
		t.Errorf("GETs of the folder = %q, want one of the head", got)
	}
	again := decode[view.FilesPage](t, f.h.do(get(f.at("")+"?select=board.jpg&view=tiles&details=0", f.sess)))
	if got := fileRow(t, again, "board.jpg").Dimensions; got != "640×480" {
		t.Errorf("the tile chosen without its panel = %q", got)
	}
	if got := f.srv.Ranges(); len(got) != 1 {
		t.Errorf("the same version read again: %q", got)
	}

	whole := filesFixture(t, true)
	whole.srv.FilesIgnoreRange = true
	whole.srv.SeedFile(filesDocs+"far.jpg", string(cameraJPEG(t, 640, 480, conf.ImageHeadBytes))+strings.Repeat(".", 3<<20))
	page = decode[view.FilesPage](t, whole.h.do(get(whole.at("")+"?select=far.jpg", whole.sess)))
	if got := fileRow(t, page, "far.jpg").Dimensions; got != "" {
		t.Errorf("a header past the cap = %q", got)
	}
	if page.Details == nil || len(page.Details.Fields) != 4 {
		t.Errorf("the panel without a size = %+v", page.Details)
	}
}

// cameraJPEG is a JPEG with an APP1 Exif segment of exif bytes before its
// frame header, as a camera writes one.
func cameraJPEG(t *testing.T, w, h, exif int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	raw := b.Bytes()
	out := append([]byte(nil), raw[:2]...)
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
