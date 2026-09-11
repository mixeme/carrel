package browser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const minContrastAA = 4.5

// ParseCSSColor parses a computed CSS color: #rrggbb, rgb(), rgba(), or
// color(srgb …). Alpha is ignored. sRGB channel values in color() are 0–1.
func ParseCSSColor(css string) ([3]float64, error) {
	css = strings.TrimSpace(css)
	switch {
	case strings.HasPrefix(css, "#"):
		r, g, b, err := ParseHexColor(css)
		return [3]float64{r, g, b}, err
	case strings.HasPrefix(css, "rgb("):
		return parseRGBChannels(css[4:len(css)-1], 255)
	case strings.HasPrefix(css, "rgba("):
		return parseRGBChannels(css[5:len(css)-1], 255)
	case strings.HasPrefix(css, "color(srgb"):
		inner := strings.TrimSpace(strings.TrimPrefix(css, "color(srgb"))
		inner = strings.TrimSuffix(inner, ")")
		if i := strings.Index(inner, "/"); i >= 0 {
			inner = inner[:i]
		}
		parts := strings.Fields(inner)
		if len(parts) < 3 {
			return [3]float64{}, fmt.Errorf("bad color(srgb) %q", css)
		}
		return parseRGBChannels(strings.Join(parts[:3], " "), 1)
	default:
		return [3]float64{}, fmt.Errorf("unsupported color %q", css)
	}
}

func parseRGBChannels(inner string, scale float64) ([3]float64, error) {
	inner = strings.TrimSpace(inner)
	var parts []string
	if strings.Contains(inner, ",") {
		parts = strings.Split(inner, ",")
	} else {
		parts = strings.Fields(inner)
	}
	if len(parts) < 3 {
		return [3]float64{}, fmt.Errorf("bad rgb channels %q", inner)
	}
	var out [3]float64
	for i := 0; i < 3; i++ {
		p := strings.TrimSpace(parts[i])
		if strings.HasSuffix(p, "%") {
			v, err := strconv.ParseFloat(strings.TrimSuffix(p, "%"), 64)
			if err != nil {
				return out, err
			}
			out[i] = v / 100
			continue
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return out, err
		}
		out[i] = v / scale
	}
	return out, nil
}

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
