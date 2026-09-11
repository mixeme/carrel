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

// escapedJS reports catalog cells whose viewport-fixed children spill outside
// the cell, plus the theme the boot script settled on.
const escapedJS = `(() => {
  const bad = [];
  document.querySelectorAll('.catalog-demo').forEach(cell => {
    const c = cell.getBoundingClientRect();
    for (const el of cell.querySelectorAll('*')) {
      if (getComputedStyle(el).position !== 'fixed') continue;
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
