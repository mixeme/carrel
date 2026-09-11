package browser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const minContrastAA = 4.5

// ParseHexColor parses #rrggbb.
func ParseHexColor(hex string) (r, g, b float64, err error) {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("browser: invalid color %q", hex)
	}
	rv, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}
	gv, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}
	bv, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}
	return float64(rv) / 255, float64(gv) / 255, float64(bv) / 255, nil
}

func channelLinear(c float64) float64 {
	if c <= 0.03928 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// RelativeLuminance computes WCAG relative luminance.
func RelativeLuminance(r, g, b float64) float64 {
	return 0.2126*channelLinear(r) + 0.7152*channelLinear(g) + 0.0722*channelLinear(b)
}

// ContrastRatio returns WCAG contrast between two sRGB colors.
func ContrastRatio(r1, g1, b1, r2, g2, b2 float64) float64 {
	l1 := RelativeLuminance(r1, g1, b1)
	l2 := RelativeLuminance(r2, g2, b2)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// MeetsAA reports whether contrast meets WCAG AA for normal text.
func MeetsAA(ratio float64) bool {
	return ratio+1e-9 >= minContrastAA
}
