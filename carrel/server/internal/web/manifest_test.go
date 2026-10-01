package web_test

import (
	"bytes"
	"encoding/json"
	"image/color"
	"image/png"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/render"
)

// U-11, the lead's decision П3: the web manifest and its PNG icons come from
// the binary (C-5.9), anonymous like every file of it: the name, the
// address the application opens at, standalone, the colours, and the icons
// — 192 and 512 on the light plate and a maskable 512 on the canvas of 120
// of the icon reference — each at its versioned address, cached for good;
// another version is 404.
func TestTheManifestAndItsIcons(t *testing.T) {
	h := newHarness(t)
	rec := h.do(get(render.ManifestPath, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/manifest+json" {
		t.Fatalf("manifest: %d %q %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
	var m struct {
		Name, StartURL, Display, ThemeColor string
		Icons                               []struct{ Src, Sizes, Type, Purpose string }
	}
	raw := strings.NewReplacer(`"start_url"`, `"StartURL"`, `"theme_color"`, `"ThemeColor"`).Replace(rec.Body.String())
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "Carrel" || m.StartURL != "/app/" || m.Display != "standalone" || m.ThemeColor == "" || len(m.Icons) != 3 {
		t.Fatalf("manifest = %+v", m)
	}
	ink, plate, shade := color.RGBA{0x17, 0x21, 0x1C, 0xff}, color.RGBA{0xE8, 0xDF, 0xCB, 0xff}, color.RGBA{0x4A, 0x6B, 0x52, 0xff}
	for _, ic := range m.Icons {
		rec := h.do(get(ic.Src, nil))
		img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
		if rec.Code != http.StatusOK || err != nil || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("%s: %d %v %q", ic.Src, rec.Code, err, rec.Header().Get("Cache-Control"))
		}
		size := img.Bounds().Dx()
		if ic.Sizes != strconv.Itoa(size)+"x"+strconv.Itoa(size) || ic.Type != "image/png" {
			t.Errorf("%s: drawn %d, said %s %s", ic.Src, size, ic.Sizes, ic.Type)
		}
		// Where the reference puts the masses: the plate in the corner, the
		// shade above its centre, the post below it — in the square of 96, or
		// of 120 about it (−12) for the maskable one.
		at := func(x, y float64) color.RGBA {
			from, span := 0.0, 96.0
			if ic.Purpose == "maskable" {
				from, span = -12, 120
			}
			return color.RGBAModel.Convert(img.At(int((x-from)*float64(size)/span), int((y-from)*float64(size)/span))).(color.RGBA)
		}
		if at(2, 2) != plate || at(48, 30) != shade || at(48, 65) != ink || at(30, 65) != plate {
			t.Errorf("%s (%s): plate %v, shade %v, post %v, beside the post %v", ic.Src, ic.Purpose, at(2, 2), at(48, 30), at(48, 65), at(30, 65))
		}
		if strings.HasSuffix(ic.Src, ".png") {
			other := strings.Replace(ic.Src, "/static/icon/", "/static/icon/000000000000", 1)
			if rec := h.do(get(other, nil)); rec.Code != http.StatusNotFound {
				t.Errorf("%s: %d", other, rec.Code)
			}
		}
	}
	purposes := m.Icons[0].Purpose + " " + m.Icons[1].Purpose + " " + m.Icons[2].Purpose
	if purposes != "any any maskable" {
		t.Errorf("purposes %s", purposes)
	}
}
