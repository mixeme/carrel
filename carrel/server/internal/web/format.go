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
