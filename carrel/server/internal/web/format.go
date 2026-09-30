package web

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"

	"carrel.dev/carrel/server/view"
)

// Format helpers implement C-10 at the request layer.

func text(s string) view.Text { return view.Text(s) }

// line is a sentence of the server with nothing marked in it: one run, none
// when it is empty (C-6.2 Runs).
func line(s string) view.Runs {
	if s == "" {
		return nil
	}
	return view.Runs{{Text: view.Text(s)}}
}

// runs is a sentence in pieces, the marked ones among them (C-6.2 Runs):
// the pieces are written in order, an empty one left out.
func runs(parts ...view.Run) view.Runs {
	var out view.Runs
	for _, p := range parts {
		if p.Text != "" {
			out = append(out, p)
		}
	}
	return out
}

func plainRun(s string) view.Run  { return view.Run{Text: view.Text(s)} }
func monoRun(s string) view.Run   { return view.Run{Text: view.Text(s), Mark: view.RunMono} }
func strongRun(s string) view.Run { return view.Run{Text: view.Text(s), Mark: view.RunStrong} }

func formatSize(n int64) view.Size {
	if n <= 0 {
		return "0 B"
	}
	const unit = 1024.0
	if n < unit {
		return view.Size(fmt.Sprintf("%d B", n))
	}
	exp := int(math.Log(float64(n)) / math.Log(unit))
	suffix := []string{"B", "KB", "MB", "GB", "TB"}[exp]
	val := float64(n) / math.Pow(unit, float64(exp))
	return view.Size(fmt.Sprintf("%.1f %s", val, suffix))
}

// collectionPalette is the mockup's six tones (s-profile-7, C-10): the colour
// derived from an address, and the only colours the collection sheets offer.
// Lower case, as hexColour writes what a server sent.
var collectionPalette = []view.Colour{
	"#4a6b52", "#3f6e8c", "#6b4e7a", "#a05a2c", "#8c6239", "#9a9280",
}

func collectionColour(path string) view.Colour {
	h := sha256.Sum256([]byte(path))
	return collectionPalette[int(h[0])%len(collectionPalette)]
}

func stamp(account, collection string) view.Stamp {
	return view.Stamp(fmt.Sprintf("%s · %s", account, collection))
}

func encodePathSegment(seg string) string {
	return strings.ReplaceAll(seg, "/", "%2F")
}
