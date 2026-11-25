package web_test

import (
	"image"
	"math/rand/v2"
	"net/http"
	"path/filepath"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Audit №4, А4-3: a contact's photo comes to its step (photo-preview) in a
// form of the edit page, up to CARREL_PHOTO_MAX_UPLOAD_BYTES; the token in
// the body or the form read by the step parsed it with a MiB of memory, and
// the photo past that MiB went to a temporary file — a copy of a person's
// photo on Carrel's disk for as long as the request lived (on
// carrel.mixfed.ru /tmp is a tmpfs of 64 MiB, which two such uploads fill).
// The body has its ceiling already; the form is read within it in memory.
func TestAPhotoStepLeavesNothingOnDisk(t *testing.T) {
	big := image.NewRGBA(image.Rect(0, 0, 800, 800))
	rng := rand.New(rand.NewPCG(3, 4))
	for i := range big.Pix {
		big.Pix[i] = byte(rng.UintN(256))
	}
	large := pngOf(t, big)
	if len(large) <= 1<<20 {
		t.Fatalf("the test file is %d bytes, not over a form's 1 MiB", len(large))
	}
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"
	add := actionHref(t, decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Photo.Actions, "photo.add")
	// The step removes its temporary files before it answers, so what is
	// asked is whether it needs any: the temporary directory is one that
	// does not exist, and a form that would write there fails.
	absent := filepath.Join(t.TempDir(), "absent")
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, absent)
	}
	for _, header := range []bool{false, true} {
		if resp := f.h.do(uploadRequest(add, f.sess, header, "noise.png", large)); resp.Code != http.StatusSeeOther {
			t.Errorf("token in header %v, no temporary directory: %d %.200s", header, resp.Code, resp.Body.String())
		}
	}
}
