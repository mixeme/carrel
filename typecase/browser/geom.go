package browser

import "math"

const pxTolerance = 2.0

// Intersects reports whether two rectangles overlap.
func Intersects(a, b Rect) bool {
	return a.Left < b.Right && a.Right > b.Left && a.Top < b.Bottom && a.Bottom > b.Top
}

// CloseEnough compares two lengths within tolerance.
func CloseEnough(a, b float64) bool {
	return math.Abs(a-b) <= pxTolerance
}

// RectsEqual compares geometry ignoring color.
func RectsEqual(a, b Rect) bool {
	return CloseEnough(a.Top, b.Top) &&
		CloseEnough(a.Left, b.Left) &&
		CloseEnough(a.Width, b.Width) &&
		CloseEnough(a.Height, b.Height)
}

// FullyVisible reports whether a rect fits inside the viewport.
func FullyVisible(r Rect, vw, vh float64) bool {
	return r.Top >= 0 && r.Left >= 0 && r.Bottom <= vh+pxTolerance && r.Right <= vw+pxTolerance
}
