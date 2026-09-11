package browser_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/chromedp/chromedp"

	"carrel.dev/typecase/browser"
	"carrel.dev/typecase/screens"
)

type lookProbe struct {
	NavHeight     float64  `json:"navHeight"`
	TickRowHeight float64  `json:"tickRowHeight"`
	ListBorder    float64  `json:"listBorder"`
	BrandHasSVG   bool     `json:"brandHasSVG"`
	SideWidth     float64  `json:"sideWidth"`
	SymbolMissing []string `json:"symbolMissing"`
	NavEmoji      []string `json:"navEmoji"`
	Rank2Buttons  int      `json:"rank2Buttons"`
}

const lookProbeJS = `(() => {
  const nav = document.querySelector('.tc-nav__item.is-current');
  const tick = document.querySelector('.tc-tickrow');
  const list = document.querySelector('.tc-list');
  const brandSVG = document.querySelector('.tc-brand svg');
  const side = document.querySelector('.tc-side');
  const missing = [];
  document.querySelectorAll('use[href^="#"]').forEach((u) => {
    const id = (u.getAttribute('href') || '').slice(1);
    if (id && !document.getElementById(id)) missing.push(id);
  });
  const navEmoji = [];
  document.querySelectorAll('.tc-nav__icon-svg').forEach((el) => {
    const t = el.textContent.trim();
    if (t) navEmoji.push(t);
  });
  return {
    navHeight: nav ? nav.getBoundingClientRect().height : 0,
    tickRowHeight: tick ? tick.getBoundingClientRect().height : 0,
    listBorder: list ? parseFloat(getComputedStyle(list).borderTopWidth) || 0 : -1,
    brandHasSVG: !!brandSVG,
    sideWidth: side ? side.getBoundingClientRect().width : 0,
    symbolMissing: missing,
    navEmoji,
    rank2Buttons: document.querySelectorAll('[data-tc-bar-rank2] .tc-btn').length,
  };
})()`

func TestScreensLook(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("screen look probe needs Chrome")
	}
	srv := screensServer(t)
	h := browser.NewHarness(t)
	for _, sc := range screens.All() {
		sc := sc
		t.Run(sc.ID, func(t *testing.T) {
			if sc.ID == "states" {
				var n int
				url := screenURL(srv.URL, sc.ID, "")
				if err := h.Run(url, 1280, 900, chromedp.Evaluate(`document.querySelectorAll('.screens-frame__mock').length`, &n)); err != nil {
					t.Fatal(err)
				}
				if n < 5 {
					t.Fatalf("states frames: got %d want >= 5", n)
				}
				return
			}
			var raw json.RawMessage
			url := screenURL(srv.URL, sc.ID, "")
			if err := h.Run(url, 1280, 900, layoutSettle(), chromedp.Evaluate(lookProbeJS, &raw)); err != nil {
				t.Fatal(err)
			}
			var p lookProbe
			if err := json.Unmarshal(raw, &p); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if p.NavHeight > 0 && !closeEnough(p.NavHeight, 29) {
				t.Fatalf(".tc-nav__item height %.1f want 29", p.NavHeight)
			}
			if p.TickRowHeight > 0 && !closeEnough(p.TickRowHeight, 26) {
				t.Fatalf(".tc-tickrow height %.1f want 26", p.TickRowHeight)
			}
			if p.ListBorder > 0.5 {
				t.Fatalf(".tc-list border %.1f want 0", p.ListBorder)
			}
			if !p.BrandHasSVG {
				t.Fatal(".tc-brand svg missing")
			}
			if len(p.SymbolMissing) > 0 {
				t.Fatalf("missing sprite symbols: %v", p.SymbolMissing)
			}
			if len(p.NavEmoji) > 0 {
				t.Fatalf("emoji in nav icons: %v", p.NavEmoji)
			}
			if sc.ID == "notes" && p.SideWidth > 0 && !closeEnough(p.SideWidth, 440) {
				t.Fatalf("/notes .tc-side width %.1f want 440", p.SideWidth)
			}
			if sc.ID == "notes" && p.Rank2Buttons < 2 {
				t.Fatalf("Rank2 buttons %d want >= 2 at 1280", p.Rank2Buttons)
			}
		})
	}
}

func layoutSettle() chromedp.Action {
	return chromedp.Evaluate(`new Promise(function(resolve) {
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
})`, nil)
}

func closeEnough(got, want float64) bool {
	return math.Abs(got-want) <= 1
}
