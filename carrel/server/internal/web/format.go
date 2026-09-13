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

func collectionColour(path string) view.Colour {
	h := sha256.Sum256([]byte(path))
	// Eight muted tones from architecture.md — index from hash.
	palette := []view.Colour{
		"#5c6b73", "#6b5c73", "#736b5c", "#5c736b",
		"#6b735c", "#735c6b", "#5c6773", "#67735c",
	}
	return palette[int(h[0])%len(palette)]
}

func stamp(account, collection string) view.Stamp {
	return view.Stamp(fmt.Sprintf("%s · %s", account, collection))
}

func encodePathSegment(seg string) string {
	return strings.ReplaceAll(seg, "/", "%2F")
}
