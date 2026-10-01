package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"strconv"
	"sync"
)

// Installing Carrel as an application (U-11, the lead's decision П3): the
// web manifest and its PNG icons come from the binary, as every file does
// (C-5.9). The icons are drawn here from the mark's master geometry — the
// lamp of docs/visual/carrel-icon.html, version 1.1: the shade an accent
// half-disc with its skirt, the hardware one ink contour, on the light
// plate; the maskable one on the canvas of 120 the reference widens it to,
// the mark untouched. Drawn once, on the first request, and kept: their
// address carries the hash of what was drawn, so they are cached for good.
// The link to the manifest and the worker of the shell are the library's
// (Typecase, task 91).

// The colours of the icon reference (§6, the light plate) and of the page
// the application opens on.
var (
	iconPlate  = color.RGBA{0xE8, 0xDF, 0xCB, 0xff}
	iconShade  = color.RGBA{0x4A, 0x6B, 0x52, 0xff}
	iconInk    = color.RGBA{0x17, 0x21, 0x1C, 0xff}
	themeColor = "#F5F1E8" // the background of the brand (--bg)
)

// iconFile is one icon of the manifest.
type iconFile struct {
	name     string
	size     int
	maskable bool
}

var iconFiles = []iconFile{
	{"icon-192.png", 192, false},
	{"icon-512.png", 512, false},
	{"icon-maskable-512.png", 512, true},
}

// icons are the drawn files by name, and the version their address carries.
type icons struct {
	once    sync.Once
	files   map[string][]byte
	version string
}

func (ic *icons) load() {
	ic.once.Do(func() {
		ic.files = map[string][]byte{}
		sum := sha256.New()
		for _, f := range iconFiles {
			var buf bytes.Buffer
			if err := png.Encode(&buf, drawIcon(f.size, f.maskable)); err != nil {
				panic(err) // an image in memory always encodes
			}
			ic.files[f.name] = buf.Bytes()
			sum.Write(buf.Bytes())
		}
		ic.version = hex.EncodeToString(sum.Sum(nil))[:12]
	})
}

// inMark says which part of the mark the point (x, y) of the master square
// of 96 is in: 1 the shade, 2 the hardware, 0 the plate (§3 of the
// reference: the shade is the half-circle of R30 about 48,42 above 42 and
// the skirt 18…78 × 42…48; the hub 42…54 × 48…54, the post 45…51 × 54…75,
// the base 27…69 × 75…84 with corners of 3).
func inMark(x, y float64) int {
	switch {
	case y <= 42 && (x-48)*(x-48)+(y-42)*(y-42) <= 30*30:
		return 1
	case x >= 18 && x <= 78 && y >= 42 && y <= 48:
		return 1
	case x >= 42 && x <= 54 && y >= 48 && y <= 54:
		return 2
	case x >= 45 && x <= 51 && y >= 54 && y <= 75:
		return 2
	case x >= 27 && x <= 69 && y >= 75 && y <= 84:
		// The corners of the base are quarter circles of 3.
		cx, cy := math.Max(30, math.Min(66, x)), math.Max(78, math.Min(81, y))
		if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= 9 {
			return 2
		}
	}
	return 0
}

// drawIcon draws the mark on its plate at size pixels: the square of 96,
// or, maskable, the canvas of 120 about it (from −12). Each pixel is the
// mean of 8 × 8 samples, so the arc is smoothed and the straight edges —
// on whole units of the master — stay sharp at these sizes.
func drawIcon(size int, maskable bool) *image.RGBA {
	from, span := 0.0, 96.0
	if maskable {
		from, span = -12, 120
	}
	const n = 8
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	colours := [3]color.RGBA{iconPlate, iconShade, iconInk}
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b int
			for sy := 0; sy < n; sy++ {
				for sx := 0; sx < n; sx++ {
					x := from + (float64(px)+(float64(sx)+0.5)/n)*span/float64(size)
					y := from + (float64(py)+(float64(sy)+0.5)/n)*span/float64(size)
					c := colours[inMark(x, y)]
					r, g, b = r+int(c.R), g+int(c.G), b+int(c.B)
				}
			}
			img.SetRGBA(px, py, color.RGBA{uint8(r / (n * n)), uint8(g / (n * n)), uint8(b / (n * n)), 0xff})
		}
	}
	return img
}

// iconPath is where an icon is served: under its version (C-5.9).
func (s *Server) iconPath(name string) string {
	s.icons.load()
	return s.route("/static/icon/" + s.icons.version + "/" + name)
}

// webManifest is the manifest: the name, the address the application opens
// at and the one it keeps to, how it shows, its colours, and the icons.
type webManifest struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	Display         string         `json:"display"`
	BackgroundColor string         `json:"background_color"`
	ThemeColor      string         `json:"theme_color"`
	Icons           []manifestIcon `json:"icons"`
}

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

// getManifest answers the web manifest (C-5.9). It names the icons by
// their versioned addresses and is itself asked again each time
// (no-cache): a new build draws new icons under a new address.
func (s *Server) getManifest(w http.ResponseWriter, r *http.Request) {
	m := webManifest{
		Name: "Carrel", ShortName: "Carrel",
		StartURL: s.route("/app/"), Scope: s.route("/"), Display: "standalone",
		BackgroundColor: themeColor, ThemeColor: themeColor,
	}
	for _, f := range iconFiles {
		purpose := "any"
		if f.maskable {
			purpose = "maskable"
		}
		size := strconv.Itoa(f.size)
		m.Icons = append(m.Icons, manifestIcon{Src: s.iconPath(f.name), Sizes: size + "x" + size, Type: "image/png", Purpose: purpose})
	}
	body, err := json.Marshal(m)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}

// getIcon serves one icon of the manifest under the version drawn; another
// version or name is not there (C-5.9: 404 with no-store).
func (s *Server) getIcon(w http.ResponseWriter, r *http.Request) {
	s.icons.load()
	body, ok := s.icons.files[r.PathValue("file")]
	if !ok || r.PathValue("version") != s.icons.version {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(body)
}
