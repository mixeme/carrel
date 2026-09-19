package card

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

func parseCard(t *testing.T, doc string) *model.Object {
	t.Helper()
	obj, err := Parse("/c/p.vcf", "e", strings.NewReader(strings.ReplaceAll(doc, "\n", "\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

func openPhoto(t *testing.T, obj *model.Object) (string, string) {
	t.Helper()
	rc, declared, err := PhotoOpen(obj)
	if err != nil {
		t.Fatalf("PhotoOpen: %v", err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	return string(b), declared
}

// vCard 4.0 keeps a photo as a data: URI and links one without VALUE=uri.
func TestPhotoForms(t *testing.T) {
	obj := parseCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:a\nPHOTO:data:image/png;base64,UGhvdG8=\nEND:VCARD\n")
	if _, linked := PhotoURI(obj); linked {
		t.Error("a data: URI was taken for a link")
	}
	if body, declared := openPhoto(t, obj); body != "Photo" || declared != "image/png" {
		t.Errorf("data URI: body %q, type %q", body, declared)
	}

	obj = parseCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:a\nPHOTO;VALUE=uri:data:image/gif;base64,UGhvdG8\nEND:VCARD\n")
	if body, declared := openPhoto(t, obj); body != "Photo" || declared != "image/gif" {
		t.Errorf("VALUE=uri data URI without padding: body %q, type %q", body, declared)
	}

	obj = parseCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:a\nPHOTO;MEDIATYPE=image/jpeg:https://example.com/p.jpg\nEND:VCARD\n")
	if link, linked := PhotoURI(obj); !linked || link != "https://example.com/p.jpg" {
		t.Errorf("4.0 link without VALUE=uri: %q %v", link, linked)
	}
	if _, _, err := PhotoOpen(obj); err == nil {
		t.Error("a link was opened as a photo")
	}

	obj = parseCard(t, "BEGIN:VCARD\nVERSION:3.0\nUID:a\nPHOTO:UGhvdG8=\nEND:VCARD\n")
	if body, declared := openPhoto(t, obj); body != "Photo" || declared != "" {
		t.Errorf("bare base64 without TYPE: body %q, type %q — nothing may be guessed", body, declared)
	}
}

// go-vcard escapes the comma of a data: URI; it must leave as written, for
// Carrel's photo and for another client's on an unrelated edit. A text comma
// written fresh is escaped: NOTE is changed here, so it is written anew.
func TestEncodeKeepsTheDataURIComma(t *testing.T) {
	obj := parseCard(t, "BEGIN:VCARD\nVERSION:4.0\nUID:a\nPHOTO:data:image/png;base64,UGhvdG8=\nNOTE:old\nEND:VCARD\n")
	patch, err := model.NewPatch(model.SetText{Prop: "NOTE", Value: "one, two"})
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	doc := buf.String()
	if !strings.Contains(doc, "PHOTO:data:image/png;base64,UGhvdG8=\r\n") || strings.Contains(doc, `base64\,`) {
		t.Errorf("photo line changed:\n%s", doc)
	}
	if !strings.Contains(doc, `NOTE:one\, two`) {
		t.Errorf("a text comma lost its escape:\n%s", doc)
	}
	back, err := Parse("/c/p.vcf", "e", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if body, _ := openPhoto(t, back); body != "Photo" {
		t.Errorf("round trip: %q", body)
	}
}

func TestPhotoOpsFollowTheVersion(t *testing.T) {
	jpeg := []byte("Photo")
	cases := map[string]string{
		"4.0": "PHOTO:data:image/jpeg;base64,UGhvdG8=\r\n",
		"3.0": "PHOTO;ENCODING=b;TYPE=JPEG:UGhvdG8=\r\n",
		"2.1": "PHOTO;ENCODING=BASE64;TYPE=JPEG:UGhvdG8=\r\n",
	}
	for version, want := range cases {
		obj := parseCard(t, "BEGIN:VCARD\nVERSION:"+version+"\nUID:a\nPHOTO;VALUE=uri;TYPE=PNG:https://example.com/old.png\nX-KEEP:yes\nEND:VCARD\n")
		patch, err := model.NewPatch(PhotoOps(obj, jpeg)...)
		if err != nil {
			t.Fatal(err)
		}
		if err := patch.Apply(obj); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Encode(&buf, obj); err != nil {
			t.Fatal(err)
		}
		doc := buf.String()
		if !strings.Contains(doc, want) || strings.Count(doc, "PHOTO") != 1 {
			t.Errorf("%s: want %q in\n%s", version, want, doc)
		}
		if !strings.Contains(doc, "VERSION:"+version) || !strings.Contains(doc, "X-KEEP:yes") || strings.Contains(doc, "old.png") {
			t.Errorf("%s: the rest of the card changed:\n%s", version, doc)
		}
	}
}
