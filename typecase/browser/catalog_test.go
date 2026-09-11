package browser_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chromedp/chromedp"

	"carrel.dev/typecase"
	"carrel.dev/typecase/browser"
	"carrel.dev/typecase/catalog"
)

// catalogServer serves the catalog page plus the assets it links.
func catalogServer(t *testing.T, withJS bool) *httptest.Server {
	t.Helper()
	css, err := typecase.CSS()
	if err != nil {
		t.Fatalf("css: %v", err)
	}
	themeJS, err := typecase.ThemeJS()
	if err != nil {
		t.Fatalf("theme js: %v", err)
	}
	js, err := typecase.JS()
	if err != nil {
		t.Fatalf("js: %v", err)
	}
	page := catalog.PageHTML("Typecase catalog", catalog.Entries(), withJS)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})
	mux.HandleFunc("/typecase.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(css))
	})
	mux.HandleFunc("/catalog.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(catalog.CSS()))
	})
	mux.HandleFunc("/theme.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(themeJS)
	})
	mux.HandleFunc("/tc.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(js)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// escapedJS reports catalog cells whose positioned children spill outside
// the cell, plus the theme the boot script settled on.
const escapedJS = `(() => {
  const bad = [];
  document.querySelectorAll('.catalog-demo').forEach(cell => {
    const c = cell.getBoundingClientRect();
    for (const el of cell.querySelectorAll('*')) {
      const pos = getComputedStyle(el).position;
      if (pos !== 'fixed' && pos !== 'absolute') continue;
      const r = el.getBoundingClientRect();
      if (r.width === 0 && r.height === 0) continue;
      if (r.top < c.top - 2 || r.left < c.left - 2 || r.bottom > c.bottom + 2 || r.right > c.right + 2) {
        bad.push(cell.closest('.catalog-entry').id + ' / ' + el.className);
      }
    }
  });
  return {
    entries: document.querySelectorAll('.catalog-entry').length,
    theme: document.documentElement.getAttribute('data-tc-theme'),
    escaped: bad
  };
})()`

// inkOverflowJS flags text nodes whose font bounding box exceeds the line box
// inside overflow-hidden ancestors. scrollHeight misses ink overflow.
const inkOverflowJS = `(() => {
  const bad = [];
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d');
  for (const el of document.querySelectorAll('*')) {
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
      bad.push(el.className || el.tagName);
    }
  }
  return bad;
})()`

type catalogProbe struct {
	Entries int      `json:"entries"`
	Theme   string   `json:"theme"`
	Escaped []string `json:"escaped"`
}

// TestCatalogCellsContainOverlays keeps every state readable on one page.
// Scrims, overlays and the rail drawer are fixed to the viewport by design, so
// without a per-cell containing block a single open scrim hides the whole
// catalog.
func TestCatalogCellsContainOverlays(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("catalog layout check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var probe catalogProbe
	if err := h.Run(srv.URL, 1200, 900, chromedp.Evaluate(escapedJS, &probe)); err != nil {
		t.Fatal(err)
	}
	if probe.Entries == 0 {
		t.Fatal("catalog rendered no entries")
	}
	if probe.Theme != "light" && probe.Theme != "dark" {
		t.Errorf("theme boot script did not run before paint: data-tc-theme=%q", probe.Theme)
	}
	for _, e := range probe.Escaped {
		t.Errorf("escapes its cell: %s", e)
	}
}

// TestCatalogFocusOnLoad ensures pre-open overlays do not steal focus at load.
func TestCatalogFocusOnLoad(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("catalog focus check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var onBody bool
	if err := h.Run(srv.URL, 1200, 900, chromedp.Evaluate(
		`document.activeElement === document.body || document.activeElement === document.documentElement`,
		&onBody,
	)); err != nil {
		t.Fatal(err)
	}
	if !onBody {
		t.Fatalf("focus should stay on document body after catalog load, got %v", onBody)
	}
}

func TestCatalogTextInkFitsLineBox(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("catalog ink check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var bad []string
	if err := h.Run(srv.URL, 1280, 900, chromedp.Evaluate(inkOverflowJS, &bad)); err != nil {
		t.Fatal(err)
	}
	for _, cls := range bad {
		t.Errorf("font box exceeds line box: %s", cls)
	}
}

func TestCatalogRowTextStacked(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("catalog row layout check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var stacked bool
	js := `(() => {
	  const cell = document.getElementById('tc-row--avatar-default');
	  if (!cell) return false;
	  const title = cell.querySelector('.tc-row__title');
	  const subtitle = cell.querySelector('.tc-row__subtitle');
	  if (!title || !subtitle) return false;
	  const tr = title.getBoundingClientRect();
	  const sr = subtitle.getBoundingClientRect();
	  return sr.top >= tr.bottom - 1;
	})()`
	if err := h.Run(srv.URL, 1280, 900, chromedp.Evaluate(js, &stacked)); err != nil {
		t.Fatal(err)
	}
	if !stacked {
		t.Fatal("tc-row--avatar title and subtitle share one line")
	}
}

func TestCatalogTreeLabelWidth(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("catalog tree layout check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var ok bool
	js := `(() => {
	  const cell = document.getElementById('tc-tree-default');
	  if (!cell) return false;
	  const label = cell.querySelector('.tc-tree__label');
	  if (!label) return false;
	  const text = label.textContent.trim();
	  const canvas = document.createElement('canvas');
	  const ctx = canvas.getContext('2d');
	  ctx.font = getComputedStyle(label).font;
	  const need = ctx.measureText(text).width;
	  const have = label.getBoundingClientRect().width;
	  return have >= need - 1;
	})()`
	if err := h.Run(srv.URL, 1280, 900, chromedp.Evaluate(js, &ok)); err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("tc-tree label narrower than its text with children open")
	}
}

func TestCatalogQueueStatesDiffer(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("catalog queue check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var distinct int
	js := `(() => {
	  const states = ['idle', 'running', 'done', 'failed', 'partial', 'cancelled'];
	  const colours = new Set();
	  for (const st of states) {
	    const row = document.querySelector('#tc-queue-' + st + ' .tc-queue__row');
	    if (!row) continue;
	    colours.add(getComputedStyle(row.querySelector('.tc-queue__label')).color);
	  }
	  return colours.size;
	})()`
	if err := h.Run(srv.URL, 1280, 900, chromedp.Evaluate(js, &distinct)); err != nil {
		t.Fatal(err)
	}
	if distinct < 4 {
		t.Fatalf("tc-queue states look alike: only %d distinct label colours", distinct)
	}
}
