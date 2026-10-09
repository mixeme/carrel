package photo

import (
	"bytes"
	"image"
	"image/jpeg"
	"math"

	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// Render cuts the frame from a source and writes it as a square JPEG: the
// orientation applied, then the quarter turn, then the crop, scaled to side
// or to the crop itself when that is smaller — a small photo is not enlarged.
// Nothing of the source's metadata survives: the output is new pixels,
// encoded with no APP segments. Transparency lands on white.
func Render(src Source, crop Crop, side, quality int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(src.Data))
	if err != nil {
		return nil, ErrUnreadable
	}
	out := frame(img, src.Orientation, crop.Clamp(), side)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Fit scales a source to fit a square of side and writes it as a JPEG: the
// whole image, its proportions kept, the orientation applied — the thumbnail
// of a file (S-17.1). A smaller image is not enlarged. As with Render,
// nothing of the metadata survives and transparency lands on white.
func Fit(src Source, side, quality int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(src.Data))
	if err != nil {
		return nil, ErrUnreadable
	}
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	ow, oh := w, h // seen
	if src.Orientation >= 5 {
		ow, oh = h, w
	}
	scale := math.Max(1, math.Max(ow, oh)/float64(side))
	nw, nh := max(1, int(math.Round(ow/scale))), max(1, int(math.Round(oh/scale)))
	d2s := aff{1, 0, float64(b.Min.X), 0, 1, float64(b.Min.Y)}.
		then(orient(src.Orientation, w, h)).
		then(aff{ow / float64(nw), 0, 0, 0, oh / float64(nh), 0})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, sample(img, d2s, nw, nh), &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// aff maps a point as x' = a·x + b·y + c, y' = d·x + e·y + f. Coordinates are
// continuous: pixel i covers [i, i+1).
type aff [6]float64

// then is m applied after n.
func (m aff) then(n aff) aff {
	return aff{
		m[0]*n[0] + m[1]*n[3], m[0]*n[1] + m[1]*n[4], m[0]*n[2] + m[1]*n[5] + m[2],
		m[3]*n[0] + m[4]*n[3], m[3]*n[1] + m[4]*n[4], m[3]*n[2] + m[4]*n[5] + m[5],
	}
}

func (m aff) apply(x, y float64) (float64, float64) {
	return m[0]*x + m[1]*y + m[2], m[3]*x + m[4]*y + m[5]
}

func (m aff) invert() aff {
	det := m[0]*m[4] - m[1]*m[3]
	return aff{
		m[4] / det, -m[1] / det, (m[1]*m[5] - m[2]*m[4]) / det,
		-m[3] / det, m[0] / det, (m[2]*m[3] - m[0]*m[5]) / det,
	}
}

// orient maps a point of the image as it should be seen to the image as
// stored, for EXIF orientation o of a w×h image.
func orient(o int, w, h float64) aff {
	switch o {
	case 2: // mirrored
		return aff{-1, 0, w, 0, 1, 0}
	case 3: // upside down
		return aff{-1, 0, w, 0, -1, h}
	case 4: // mirrored upside down
		return aff{1, 0, 0, 0, -1, h}
	case 5: // transposed
		return aff{0, 1, 0, 1, 0, 0}
	case 6: // to be turned clockwise
		return aff{0, 1, 0, -1, 0, h}
	case 7: // transversed
		return aff{0, -1, w, -1, 0, h}
	case 8: // to be turned counter-clockwise
		return aff{0, -1, w, 1, 0, 0}
	default:
		return aff{1, 0, 0, 0, 1, 0}
	}
}

// turn maps a point of the image turned clockwise by deg to the unturned
// w×h image.
func turn(deg int, w, h float64) aff {
	switch deg {
	case 90:
		return aff{0, 1, 0, -1, 0, h}
	case 180:
		return aff{-1, 0, w, 0, -1, h}
	case 270:
		return aff{0, -1, w, 1, 0, 0}
	default:
		return aff{1, 0, 0, 0, 1, 0}
	}
}

func frame(img image.Image, o int, crop Crop, side int) *image.RGBA {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	ow, oh := w, h // seen
	if o >= 5 {
		ow, oh = h, w
	}
	fw, fh := ow, oh // seen and turned
	if crop.Rotate == 90 || crop.Rotate == 270 {
		fw, fh = oh, ow
	}
	s := math.Min(fw, fh) * 100 / float64(crop.Zoom)
	left := (fw - s) / 2 * (1 + float64(crop.OffsetX)/100)
	top := (fh - s) / 2 * (1 + float64(crop.OffsetY)/100)
	n := side
	if float64(n) > s {
		n = max(1, int(s))
	}
	scale := s / float64(n)

	// Output pixel → turned → seen → stored, then the image's own origin.
	d2s := aff{1, 0, float64(b.Min.X), 0, 1, float64(b.Min.Y)}.
		then(orient(o, w, h)).
		then(turn(crop.Rotate, ow, oh)).
		then(aff{scale, 0, left, 0, scale, top})

	return sample(img, d2s, n, n)
}

// sample draws nw by nh pixels of img, each read where d2s puts it in the
// stored image, on white.
func sample(img image.Image, d2s aff, nw, nh int) *image.RGBA {
	b := img.Bounds()
	// The output covers an axis-aligned rectangle of the stored image.
	x0, y0 := d2s.apply(0, 0)
	x1, y1 := d2s.apply(float64(nw), float64(nh))
	r := image.Rect(int(math.Floor(math.Min(x0, x1))), int(math.Floor(math.Min(y0, y1))),
		int(math.Ceil(math.Max(x0, x1))), int(math.Ceil(math.Max(y0, y1)))).Intersect(b)

	// A kernel shrinking by more than four reads more pixels than it needs
	// to: halve first, each halving a 2×2 average, then let Catmull-Rom take
	// the last factor of two to four.
	// An image that cannot give a part of itself is not halved: the halving
	// would shrink all of it, not the rectangle.
	var cur image.Image = img
	sub, halve := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if halve {
		cur = sub.SubImage(r)
	}
	sr := r
	// Source pixels per output pixel, by area: the same whichever way the
	// orientation turns the rectangle.
	per := func() float64 {
		cb := cur.Bounds()
		return math.Sqrt(float64(cb.Dx()) * float64(cb.Dy()) / (float64(nw) * float64(nh)))
	}
	for halve && per() >= 4 && cur.Bounds().Dx() >= 2 && cur.Bounds().Dy() >= 2 {
		cb := cur.Bounds()
		half := image.NewRGBA(image.Rect(0, 0, (cb.Dx()+1)/2, (cb.Dy()+1)/2))
		draw.ApproxBiLinear.Scale(half, half.Bounds(), cur, cb, draw.Src, nil)
		cur, sr = half, half.Bounds()
	}
	if sr != r {
		kx, ky := float64(sr.Dx())/float64(r.Dx()), float64(sr.Dy())/float64(r.Dy())
		d2s = aff{kx, 0, -float64(r.Min.X) * kx, 0, ky, -float64(r.Min.Y) * ky}.then(d2s)
	}

	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
	draw.CatmullRom.Transform(out, f64.Aff3(d2s.invert()), cur, sr, draw.Over, nil)
	return out
}
