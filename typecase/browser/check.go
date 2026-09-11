package browser

import (
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Name identifies one L-9.2 browser invariant.
type Name string

const (
	MenuLabelArrow Name = "menu-label-arrow"
	PopoverAnchor  Name = "popover-anchor"
	PopoverFlip    Name = "popover-flip"
	Ellipsis       Name = "ellipsis"
	BarOneRow      Name = "bar-one-row"
	TapTarget      Name = "tap-target"
	FocusReturn    Name = "focus-return"
	EscLayers      Name = "esc-layers"
	Contrast       Name = "contrast"
	ThemeGeometry  Name = "theme-geometry"
)

// AllChecks lists every L-9.2 invariant in stable order.
var AllChecks = []Name{
	MenuLabelArrow, PopoverAnchor, PopoverFlip, Ellipsis, BarOneRow,
	TapTarget, FocusReturn, EscLayers, Contrast, ThemeGeometry,
}

// Fixture holds a page body and viewport for one invariant.
type Fixture struct {
	Body   string
	Width  int
	Height int
	Extra  string
}

// Check runs one browser invariant.
func Check(name Name, h *Harness, fix Fixture) error {
	html := PageHTML(fix.Body, fix.Extra)
	url := h.RegisterPage(string(name), html)
	w, ht := fix.Width, fix.Height
	switch name {
	case MenuLabelArrow:
		return checkMenuLabelArrow(h, url, w, ht)
	case PopoverAnchor:
		return checkPopoverAnchor(h, url, w, ht)
	case PopoverFlip:
		return checkPopoverFlip(h, url, w, ht)
	case Ellipsis:
		return checkEllipsis(h, url, w, ht)
	case BarOneRow:
		return checkBarOneRow(h, url, w, ht)
	case TapTarget:
		return checkTapTarget(h, url, w, ht)
	case FocusReturn:
		return checkFocusReturn(h, url, w, ht)
	case EscLayers:
		return checkEscLayers(h, url, w, ht)
	case Contrast:
		return checkContrast(h, url, w, ht)
	case ThemeGeometry:
		return checkThemeGeometry(h, url, w, ht)
	default:
		return fmt.Errorf("browser: unknown check %q", name)
	}
}

func rectAction(sel string, dest *Rect) chromedp.Action {
	js := fmt.Sprintf(`(() => { const el = document.querySelector(%q); if (!el) return null; const r = el.getBoundingClientRect(); return {x:r.x,y:r.y,width:r.width,height:r.height,top:r.top,left:r.left,bottom:r.bottom,right:r.right}; })()`, sel)
	return chromedp.Evaluate(js, dest)
}

func checkMenuLabelArrow(h *Harness, url string, width, height int) error {
	var label, arrow Rect
	err := h.Run(url, width, height,
		chromedp.Click(".tc-menu__control", chromedp.ByQuery),
		chromedp.Sleep(80*time.Millisecond),
		rectAction(".tc-menu__label", &label),
		rectAction(".tc-menu__arrow", &arrow),
	)
	if err != nil {
		return err
	}
	if label.Width == 0 || arrow.Width == 0 {
		return fmt.Errorf("menu-label-arrow: missing elements")
	}
	if Intersects(label, arrow) {
		return fmt.Errorf("menu-label-arrow: label overlaps arrow")
	}
	return nil
}

func checkPopoverAnchor(h *Harness, url string, width, height int) error {
	var btn, pop Rect
	err := h.Run(url, width, height,
		chromedp.Click(".tc-menu__control", chromedp.ByQuery),
		chromedp.Sleep(80*time.Millisecond),
		rectAction(".tc-menu__control", &btn),
		rectAction(".tc-menu__popover", &pop),
	)
	if err != nil {
		return err
	}
	if btn.Width == 0 || pop.Width == 0 {
		return fmt.Errorf("popover-anchor: missing elements")
	}
	if !CloseEnough(btn.Left, pop.Left) {
		return fmt.Errorf("popover-anchor: left edges differ (btn=%.1f pop=%.1f)", btn.Left, pop.Left)
	}
	if pop.Top < btn.Bottom && !CloseEnough(pop.Top, btn.Bottom+2) {
		return fmt.Errorf("popover-anchor: popover not below control (btn.bottom=%.1f pop.top=%.1f)", btn.Bottom, pop.Top)
	}
	return nil
}

func checkPopoverFlip(h *Harness, url string, width, height int) error {
	var btn, pop Rect
	var vw, vh float64
	err := h.Run(url, width, height,
		chromedp.Click(".tc-menu__control", chromedp.ByQuery),
		chromedp.Sleep(80*time.Millisecond),
		rectAction(".tc-menu__control", &btn),
		rectAction(".tc-menu__popover", &pop),
		chromedp.Evaluate(`window.innerWidth`, &vw),
		chromedp.Evaluate(`window.innerHeight`, &vh),
	)
	if err != nil {
		return err
	}
	if !FullyVisible(pop, vw, vh) {
		return fmt.Errorf("popover-flip: popover not fully visible")
	}
	if pop.Bottom > btn.Top+pxTolerance {
		return fmt.Errorf("popover-flip: expected popover above control near bottom edge")
	}
	return nil
}

func checkEllipsis(h *Harness, url string, width, height int) error {
	var clipped bool
	err := h.Run(url, width, height,
		chromedp.Evaluate(`(() => { const el = document.querySelector(".tc-fields__value--path"); if (!el) return false; const cs = getComputedStyle(el); return cs.textOverflow === "ellipsis" && cs.overflow !== "visible" && el.scrollWidth > el.clientWidth + 1; })()`, &clipped),
	)
	if err != nil {
		return err
	}
	if !clipped {
		return fmt.Errorf("ellipsis: long path not clipped with ellipsis")
	}
	return nil
}

func checkBarOneRow(h *Harness, url string, width, height int) error {
	var barH float64
	var tops []float64
	err := h.Run(url, width, height,
		chromedp.Evaluate(`(() => {
  if (window.__tc && window.__tc.layoutUpdates) window.__tc.layoutUpdates();
  const b = document.querySelector(".tc-bar");
  return b ? b.getBoundingClientRect().height : 0;
})()`, &barH),
		chromedp.Evaluate(`(() => {
  if (window.__tc && window.__tc.layoutUpdates) window.__tc.layoutUpdates();
  return Array.from(document.querySelectorAll(".tc-bar .tc-btn")).filter((el) => {
    if (el.closest("[data-tc-bar-stash]")) return false;
    const r = el.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  }).map((el) => el.getBoundingClientRect().top);
})()`, &tops),
	)
	if err != nil {
		return err
	}
	if barH == 0 {
		return fmt.Errorf("bar-one-row: missing bar")
	}
	if barH > 40 {
		return fmt.Errorf("bar-one-row: bar height %.1f exceeds single row", barH)
	}
	for i, top := range tops {
		if i > 0 && !CloseEnough(top, tops[0]) {
			return fmt.Errorf("bar-one-row: buttons wrap to multiple rows")
		}
	}
	return nil
}

// tapHitJS probes the hit area rather than the drawn box: four points 21 px
// from the centre must land on the control itself. A pseudo-element that
// widens the zone is hit-tested as its owner, so this sees the zone a finger
// sees — and a control that merely grew its paint passes for the wrong reason,
// which the drawn-size probe below catches.
const tapHitJS = `(() => {
  const sels = [".tc-tick", ".tc-tickrow__tick", ".tc-close", ".tc-check", ".tc-nav--bottom .tc-nav__item"];
  const bad = [];
  for (const sel of sels) {
    const el = document.querySelector(sel);
    if (!el) { bad.push(sel + ": missing"); continue; }
    const r = el.getBoundingClientRect();
    const cx = r.x + r.width / 2, cy = r.y + r.height / 2;
    for (const [x, y] of [[cx - 21, cy], [cx + 21, cy], [cx, cy - 21], [cx, cy + 21]]) {
      const hit = document.elementFromPoint(x, y);
      if (!hit || !(el === hit || el.contains(hit))) {
        bad.push(sel + ": no hit at " + Math.round(x - cx) + "," + Math.round(y - cy));
        break;
      }
    }
  }
  return bad;
})()`

// tapDrawnJS reports the drawn rectangle of the marks that must stay small in
// the normal density: 13 px for a tick, 24 px for a close.
const tapDrawnJS = `(() => {
  const want = { ".tc-tick": 13, ".tc-tickrow__tick": 13, ".tc-close": 24 };
  const bad = [];
  for (const sel of Object.keys(want)) {
    const el = document.querySelector(sel);
    if (!el) { bad.push(sel + ": missing"); continue; }
    const r = el.getBoundingClientRect();
    if (Math.abs(r.width - want[sel]) > 1 || Math.abs(r.height - want[sel]) > 1) {
      bad.push(sel + ": " + r.width.toFixed(1) + "x" + r.height.toFixed(1) + ", want " + want[sel]);
    }
  }
  return bad;
})()`

func checkTapTarget(h *Harness, url string, width, height int) error {
	var missed []string
	if err := h.Run(url, width, height, chromedp.Evaluate(tapHitJS, &missed)); err != nil {
		return err
	}
	if len(missed) > 0 {
		return fmt.Errorf("tap-target: hit area under 44px: %s", strings.Join(missed, "; "))
	}
	var grown []string
	if err := h.Run(url, 1280, 900, chromedp.Evaluate(tapDrawnJS, &grown)); err != nil {
		return err
	}
	if len(grown) > 0 {
		return fmt.Errorf("tap-target: mark drawn at tap size: %s", strings.Join(grown, "; "))
	}
	return nil
}

const keyEscapeJS = `document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", code: "Escape", bubbles: true }))`

func checkFocusReturn(h *Harness, url string, width, height int) error {
	var active string
	err := h.Run(url, width, height,
		chromedp.Click(".tc-menu__control", chromedp.ByQuery),
		chromedp.Sleep(80*time.Millisecond),
		chromedp.Evaluate(keyEscapeJS, nil),
		chromedp.Sleep(80*time.Millisecond),
		chromedp.Evaluate(`document.activeElement ? document.activeElement.className : ""`, &active),
	)
	if err != nil {
		return err
	}
	if active != "tc-menu__control" {
		return fmt.Errorf("focus-return: focus not on menu control (%q)", active)
	}
	return nil
}

func checkEscLayers(h *Harness, url string, width, height int) error {
	var dialogHidden, sheetHidden bool
	err := h.Run(url, width, height,
		chromedp.Evaluate(keyEscapeJS, nil),
		chromedp.Sleep(80*time.Millisecond),
		chromedp.Evaluate(`(() => { const el = document.getElementById("top-dialog"); return el ? el.hidden : true; })()`, &dialogHidden),
		chromedp.Evaluate(`(() => { const el = document.getElementById("bottom-sheet"); return el ? el.hidden : true; })()`, &sheetHidden),
	)
	if err != nil {
		return err
	}
	if !dialogHidden {
		return fmt.Errorf("esc-layers: top dialog still open")
	}
	if sheetHidden {
		return fmt.Errorf("esc-layers: bottom sheet closed")
	}
	return nil
}

func checkContrast(h *Harness, url string, width, height int) error {
	type sample struct {
		Fg string `json:"fg"`
		Bg string `json:"bg"`
	}
	var light, dark sample
	sampleJS := `(() => { const el = document.querySelector(".tc-sample"); const cs = getComputedStyle(el); return { fg: cs.color, bg: cs.backgroundColor }; })()`
	err := h.Run(url, width, height,
		chromedp.Evaluate(sampleJS, &light),
		chromedp.Evaluate(`document.documentElement.setAttribute("data-tc-theme","dark")`, nil),
		chromedp.Sleep(50*time.Millisecond),
		chromedp.Evaluate(sampleJS, &dark),
	)
	if err != nil {
		return err
	}
	for _, theme := range []struct {
		name string
		s    sample
	}{
		{"light", light},
		{"dark", dark},
	} {
		fg, err := ParseCSSColor(theme.s.Fg)
		if err != nil {
			return fmt.Errorf("contrast %s fg: %w", theme.name, err)
		}
		bg, err := ParseCSSColor(theme.s.Bg)
		if err != nil {
			return fmt.Errorf("contrast %s bg: %w", theme.name, err)
		}
		ratio := ContrastRatio(fg[0], fg[1], fg[2], bg[0], bg[1], bg[2])
		if !MeetsAA(ratio) {
			return fmt.Errorf("contrast %s: ratio %.2f < %.1f", theme.name, ratio, minContrastAA)
		}
	}
	return nil
}

func checkThemeGeometry(h *Harness, url string, width, height int) error {
	var light, dark Rect
	err := h.Run(url, width, height,
		rectAction(".tc-sample", &light),
		chromedp.Evaluate(`document.documentElement.setAttribute("data-tc-theme","dark")`, nil),
		chromedp.Sleep(50*time.Millisecond),
		rectAction(".tc-sample", &dark),
	)
	if err != nil {
		return err
	}
	if !RectsEqual(light, dark) {
		return fmt.Errorf("theme-geometry: rectangles differ between themes")
	}
	return nil
}
