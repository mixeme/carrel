package card

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

func TestParsePreservesForeign(t *testing.T) {
	data, err := os.ReadFile("testdata/ada.vcf")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := Parse("/c/ada.vcf", "e1", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := obj.PropText("FN"); len(got) != 1 || got[0] != "Ada Lovelace" {
		t.Fatalf("FN = %v", got)
	}
	foreign := Foreign(obj)
	if len(foreign) != 1 || string(foreign[0].Label) != "X-CUSTOM" {
		t.Fatalf("foreign = %+v", foreign)
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	obj := model.NewObject("/c/x.vcf", "1", map[string][]model.RawValue{
		"VERSION": {{Text: "4.0"}},
		"FN":      {{Text: "Test"}},
		"UID":     {{Text: "u1"}},
	})
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	back, err := Parse("/c/x.vcf", "1", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if back.PropText("FN")[0] != "Test" {
		t.Fatalf("FN = %v", back.PropText("FN"))
	}
}

// N is structured — Family;Given;Middle;Prefix;Suffix — and it travels as one
// flat value. go-vcard escapes a backslash, a newline and a comma, and neither
// its encoder nor its decoder touches the semicolon, so the components stay
// components. Nothing covered this, and the editor is about to start writing
// two of the five (U-6.1, docs/servers.md SV-21): the three it will not offer
// have to come back untouched.
func TestEncodeKeepsStructuredName(t *testing.T) {
	const name = "Lovelace;Ada;Byron;;Countess"
	obj := model.NewObject("/c/n.vcf", "1", map[string][]model.RawValue{
		"VERSION": {{Text: "4.0"}},
		"UID":     {{Text: "u1"}},
		"FN":      {{Text: "Ada Lovelace"}},
		"N":       {{Text: name}},
	})
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "N:"+name) {
		t.Fatalf("the card on the wire does not hold N:%s\n%s", name, buf.String())
	}
	back, err := Parse("/c/n.vcf", "1", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if got := back.PropText("N"); len(got) != 1 || got[0] != name {
		t.Fatalf("N = %v, want %q", got, name)
	}

	// A comma inside a component is escaped on the way out and given back on
	// the way in: the round trip is what matters, not the bytes between.
	comma := model.NewObject("/c/c.vcf", "1", map[string][]model.RawValue{
		"VERSION": {{Text: "4.0"}},
		"UID":     {{Text: "u2"}},
		"N":       {{Text: "Lovelace, Jr.;Ada;;;"}},
	})
	buf.Reset()
	if err := Encode(&buf, comma); err != nil {
		t.Fatal(err)
	}
	back, err = Parse("/c/c.vcf", "1", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if got := back.PropText("N"); len(got) != 1 || got[0] != "Lovelace, Jr.;Ada;;;" {
		t.Fatalf("N with a comma = %v", got)
	}
}

func TestPhotoInline(t *testing.T) {
	data, _ := os.ReadFile("testdata/photo-inline.vcf")
	obj, err := Parse("/c/p.vcf", "e", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	rc, ct, err := PhotoOpen(obj)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "Photo" || ct != "JPEG" {
		t.Fatalf("body=%q ct=%q", b, ct)
	}
}

func TestPhotoURINotFetched(t *testing.T) {
	data, _ := os.ReadFile("testdata/photo-uri.vcf")
	obj, err := Parse("/c/p.vcf", "e", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if uri, ok := PhotoURI(obj); !ok || !strings.Contains(uri, "example.com") {
		t.Fatalf("uri=%q ok=%v", uri, ok)
	}
	_, _, err = PhotoOpen(obj)
	if err == nil {
		t.Fatal("expected error opening uri photo")
	}
}

func TestUpdateRejectsEmptyETag(t *testing.T) {
	p := &Provider{}
	_, err := p.Update(t.Context(), model.NewObject("/c/x.vcf", "", nil))
	if err != ErrEmptyETag {
		t.Fatalf("got %v", err)
	}
}

func TestLogoIsForeign(t *testing.T) {
	vcf := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u\r\nFN:Ada\r\nLOGO:http://example.com/l.png\r\nEND:VCARD\r\n"
	obj, err := Parse("/c/l.vcf", "e", strings.NewReader(vcf))
	if err != nil {
		t.Fatal(err)
	}
	foreign := Foreign(obj)
	if len(foreign) != 1 || string(foreign[0].Label) != "LOGO" {
		t.Fatalf("foreign = %+v", foreign)
	}
}

func TestPatchPreservesForeign(t *testing.T) {
	data, _ := os.ReadFile("testdata/ada.vcf")
	obj, err := Parse("/c/ada.vcf", "e1", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	patch, err := model.NewPatch(model.SetText{Prop: "FN", Value: "Grace"})
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	if len(Foreign(obj)) != 1 {
		t.Fatalf("lost foreign: %+v", Foreign(obj))
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	back, _ := Parse("/c/ada.vcf", "e1", &buf)
	if back.PropText("X-CUSTOM")[0] != "foreign-value" {
		t.Fatal("foreign property lost on encode")
	}
}

// docs/servers.md, SV-31: go-vcard writes a property of any length on one
// line, so Carrel folds what goes out. A photo is the case that made it
// matter: a data: URI of hundreds of kilobytes on a single line.
func TestEncodeFoldsLongLines(t *testing.T) {
	photo := "data:image/png;base64," + strings.Repeat("Q", 40<<10)
	obj := model.NewObject("/b/ada.vcf", "", map[string][]model.RawValue{
		"VERSION": {{Text: "4.0"}},
		"UID":     {{Text: "ada"}},
		"FN":      {{Text: "Ada Lovelace"}},
		"PHOTO":   {{Text: photo}},
	})
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	doc := buf.String()
	for i, line := range strings.Split(strings.TrimSuffix(doc, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Fatalf("line %d is %d octets", i+1, len(line))
		}
	}
	// And it is the same card: the library unfolds it back whole.
	back, err := Parse("/b/ada.vcf", "", strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if got := back.PropText("PHOTO"); len(got) != 1 || got[0] != photo {
		t.Errorf("the photo came back as %d octets", len(strings.Join(got, "")))
	}
}
