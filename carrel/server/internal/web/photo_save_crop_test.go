package web_test

import (
	"image"
	"image/color"
	"net/http"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Replace now waits for Save. Crop must frame that local selection rather
// than quietly reopening the old photo still stored in the card.
func TestCropUsesThePhotoSelectedInTheForm(t *testing.T) {
	old := jpegOf(t, halvesImage(100, 100))
	f := bookFixture(t, nil, "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:c1\r\nFN:Before\r\nPHOTO;ENCODING=b;TYPE=JPEG:"+b64(old)+"\r\nEND:VCARD\r\n")
	p := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/c1/edit", f.sess)))
	img := image.NewRGBA(image.Rect(0, 0, 80, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 80; x++ {
			img.Set(x, y, color.RGBA{10, 220, 10, 255})
		}
	}
	before := body(t, f, "c1.vcf")
	r := f.h.do(saveFiles(actionHref(t, p.Photo.Actions, "photo.crop"), f.sess, formFields(p.Form), "photo", jpegOf(t, img)))
	if r.Code != http.StatusSeeOther {
		t.Fatalf("Crop: %d", r.Code)
	}
	preview := decode[view.PhotoPreviewPage](t, f.h.do(get(r.Header().Get("Location"), f.sess)))
	shown := f.h.do(get(preview.Href, f.sess))
	got, _, err := image.Decode(shown.Body)
	if err != nil {
		t.Fatal(err)
	}
	rc, gc, bc, _ := got.At(got.Bounds().Dx()/2, got.Bounds().Dy()/2).RGBA()
	if gc < 150<<8 || rc > 90<<8 || bc > 90<<8 {
		t.Fatal("Crop reopened the old photo instead of the selected replacement")
	}
	if body(t, f, "c1.vcf") != before {
		t.Fatal("opening Crop wrote the card")
	}
}
