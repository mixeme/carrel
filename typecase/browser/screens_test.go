package browser_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"

	"carrel.dev/typecase/browser"
	"carrel.dev/typecase/screens"
)

func screensServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux, err := screens.Mux("")
	if err != nil {
		t.Fatalf("screens mux: %v", err)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type screenProbe struct {
	Theme       string   `json:"theme"`
	HScroll     float64  `json:"hScroll"`
	OutOfBounds []string `json:"outOfBounds"`
	InkOverflow []string `json:"inkOverflow"`
	Contrast    []sample `json:"contrast"`
}

type sample struct {
	Selector string  `json:"selector"`
	Fg       string  `json:"fg"`
	Bg       string  `json:"bg"`
	Ratio    float64 `json:"ratio"`
}

const screenProbeJS = `(() => {
  const label = (el) => {
    const c = el.className;
    if (typeof c === 'string' && c) return c.slice(0, 80);
    if (c && typeof c.baseVal === 'string' && c.baseVal) return c.baseVal.slice(0, 80);
    return el.tagName;
  };
  const root = document.documentElement;
  const vw = root.clientWidth;
  const stage = document.querySelector('.screens-stage');
  const scope = stage || document.body;
  const hScroll = Math.max(scope.scrollWidth - scope.clientWidth, 0);
  const clipRect = (el) => {
    let left = -Infinity, right = Infinity;
    let p = el.parentElement;
    while (p && p !== document.documentElement) {
      const cs = getComputedStyle(p);
      if (cs.overflowX !== 'visible') {
        const pr = p.getBoundingClientRect();
        left = Math.max(left, pr.left);
        right = Math.min(right, pr.right);
      }
      p = p.parentElement;
    }
    const r = el.getBoundingClientRect();
    return {
      left: Math.max(r.left, left === -Infinity ? r.left : left),
      right: Math.min(r.right, right === Infinity ? r.right : right)
    };
  };
  const outOfBounds = [];
  for (const el of scope.querySelectorAll('*')) {
    const cs = getComputedStyle(el);
    if (cs.display === 'none' || cs.visibility === 'hidden') continue;
    if (el.hidden) continue;
    const box = el.getBoundingClientRect();
    if (box.width === 0 && box.height === 0) continue;
    // Chrome of the page must itself fit the viewport (L-4.4, tc-crumbs).
    // Other nodes use the visible (clipped) box so overflow:auto in tc-main
    // does not flag every table cell.
    const chrome = el.matches('.tc-bar, .tc-crumbs, .tc-head, .tc-top, .tc-docbar');
    const r = chrome ? box : clipRect(el);
    if (!chrome && r.right - r.left <= 1) continue;
    if (r.right > vw + 1 || r.left < -1) {
      if (el.closest('.tc-shell--narrow .tc-shell__rail')) continue;
      outOfBounds.push(label(el));
    }
  }
  const inkOverflow = [];
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d');
  for (const el of scope.querySelectorAll('*')) {
    const cs = getComputedStyle(el);
    if (cs.overflow === 'visible') continue;
    const text = el.textContent.trim();
    if (!text) continue;
    const lh = parseFloat(cs.lineHeight);
    if (!lh) continue;
    ctx.font = cs.font;
    const m = ctx.measureText(text);
    const fontBox = (m.fontBoundingBoxAscent || m.actualBoundingBoxAscent || 0) +
      (m.fontBoundingBoxDescent || m.actualBoundingBoxDescent || 0);
    if (fontBox > lh + 0.5) {
      inkOverflow.push(label(el));
    }
  }
  const contrast = [];
  const sels = ['.tc-head__title', '.tc-empty__body', '.tc-row__title', '.tc-tickrow__label', '.tc-runbar__summary', '.tc-msg__body'];
  for (const sel of sels) {
    const el = document.querySelector(sel);
    if (!el) continue;
    const cs = getComputedStyle(el);
    let bg = cs.backgroundColor;
    let node = el;
    while ((!bg || bg === 'rgba(0, 0, 0, 0)' || bg === 'transparent') && node.parentElement) {
      node = node.parentElement;
      bg = getComputedStyle(node).backgroundColor;
    }
    contrast.push({ selector: sel, fg: cs.color, bg: bg || 'rgb(255, 255, 255)' });
  }
  return {
    theme: root.getAttribute('data-tc-theme'),
    hScroll,
    outOfBounds,
    inkOverflow,
    contrast
  };
})()`

func screenURL(base, id, theme string) string {
	url := base + "/" + id
	if theme == "dark" {
		url += "?theme=dark"
	}
	return url
}

func TestScreensRenders(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("screen layout check needs Chrome")
	}
	srv := screensServer(t)
	h := browser.NewHarness(t)

	for _, sc := range screens.All() {
		for _, view := range []struct {
			name   string
			width  int
			height int
			theme  string
		}{
			{"desktop-light", 1280, 900, ""},
			{"desktop-dark", 1280, 900, "dark"},
			{"phone-light", 375, 812, ""},
			{"phone-dark", 375, 812, "dark"},
		} {
			t.Run(sc.ID+"/"+view.name, func(t *testing.T) {
				var probe screenProbe
				url := screenURL(srv.URL, sc.ID, view.theme)
				if err := h.Run(url, view.width, view.height,
					chromedp.Evaluate(`new Promise(function(resolve) {
  (function tick(n) {
    if (window.__tc && window.__tc.layoutUpdates) {
      window.__tc.layoutUpdates();
      requestAnimationFrame(function() {
        window.__tc.layoutUpdates();
        requestAnimationFrame(resolve);
      });
      return;
    }
    if (n > 200) { resolve(); return; }
    setTimeout(function() { tick(n + 1); }, 25);
  })(0);
})`, nil),
					chromedp.Evaluate(screenProbeJS, &probe),
				); err != nil {
					t.Fatal(err)
				}
				wantTheme := "light"
				if view.theme == "dark" {
					wantTheme = "dark"
				}
				if probe.Theme != wantTheme {
					t.Errorf("theme boot: data-tc-theme=%q want %q", probe.Theme, wantTheme)
				}
				if probe.HScroll > 1 {
					t.Errorf("horizontal page scroll %.1fpx", probe.HScroll)
				}
				if len(probe.OutOfBounds) > 0 {
					t.Errorf("nodes outside viewport: %s", strings.Join(probe.OutOfBounds[:min(5, len(probe.OutOfBounds))], "; "))
				}
				if len(probe.InkOverflow) > 0 {
					t.Errorf("ink overflow: %s", strings.Join(probe.InkOverflow[:min(5, len(probe.InkOverflow))], "; "))
				}
				for _, s := range probe.Contrast {
					fg, err := browser.ParseCSSColor(s.Fg)
					if err != nil {
						t.Errorf("contrast %s fg: %v", s.Selector, err)
						continue
					}
					bg, err := browser.ParseCSSColor(s.Bg)
					if err != nil {
						t.Errorf("contrast %s bg: %v", s.Selector, err)
						continue
					}
					ratio := browser.ContrastRatio(fg[0], fg[1], fg[2], bg[0], bg[1], bg[2])
					if !browser.MeetsAA(ratio) {
						t.Errorf("contrast %s: ratio %.2f < 4.5", s.Selector, ratio)
					}
				}
			})
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
