package browser

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"carrel.dev/typecase"
	"carrel.dev/typecase/gate"
)

// Rect is a DOM bounding box in CSS pixels.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Bottom float64 `json:"bottom"`
	Right  float64 `json:"right"`
}

// Harness serves typecase assets and runs chromedp actions.
type Harness struct {
	t       *testing.T
	server  *httptest.Server
	pages   map[string]string
	browser *instance
}

// NewHarness starts a test server with bundled CSS and JS.
func NewHarness(t *testing.T) *Harness {
	t.Helper()
	css, err := typecase.CSS()
	if err != nil {
		t.Fatalf("css: %v", err)
	}
	theme, err := typecase.ThemeJS()
	if err != nil {
		t.Fatalf("theme js: %v", err)
	}
	js, err := typecase.JS()
	if err != nil {
		t.Fatalf("js: %v", err)
	}

	h := &Harness{t: t, pages: make(map[string]string)}
	mux := http.NewServeMux()
	mux.HandleFunc("/typecase.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(css))
	})
	mux.HandleFunc("/theme.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(theme)
	})
	mux.HandleFunc("/tc.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(js)
	})
	mux.HandleFunc("/page/", func(w http.ResponseWriter, r *http.Request) {
		id, err := url.PathUnescape(r.URL.Path[len("/page/"):])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		html, ok := h.pages[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	h.server = httptest.NewUnstartedServer(mux)
	h.server.Listener = ln
	h.server.Start()
	t.Cleanup(h.server.Close)

	in, err := launchChrome()
	if err != nil {
		t.Fatalf("launch %s: %v", ChromePath(), err)
	}
	h.browser = in
	t.Cleanup(in.stop)
	return h
}

// RegisterPage stores HTML and returns its URL.
func (h *Harness) RegisterPage(id, html string) string {
	h.pages[id] = html
	return h.server.URL + "/page/" + url.PathEscape(id)
}

// Run executes chromedp actions against a registered page URL.
//
// Each call gets a fresh tab in the harness browser; the viewport is set with
// device metrics rather than a window-size flag, so one process serves every
// check.
func (h *Harness) Run(pageURL string, width, height int, actions ...chromedp.Action) error {
	h.t.Helper()
	if width <= 0 {
		width = 800
	}
	if height <= 0 {
		height = 600
	}

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), h.browser.endpoint)
	defer cancelAlloc()
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()
	ctx, cancelTimeout := context.WithTimeout(ctx, 45*time.Second)
	defer cancelTimeout()

	all := []chromedp.Action{
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body"),
	}
	all = append(all, actions...)
	if err := chromedp.Run(ctx, all...); err != nil {
		return fmt.Errorf("chromedp: %w", err)
	}
	return nil
}

// PageHTML builds a minimal document around a demo fragment.
func PageHTML(body string, extraCSS string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-tc-root>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<script src="/theme.js"></script>
<link rel="stylesheet" href="/typecase.css">
<style>
body { margin: 0; padding: 16px; background: var(--tc-bg); color: var(--tc-ink); font-family: var(--tc-font-sans); }
.demo { max-width: 100%%; }
%s
</style>
</head>
<body data-tc-root>
<div class="demo">%s</div>
<script src="/tc.js"></script>
</body>
</html>`, extraCSS, body)
}

// ModuleRoot returns the typecase module directory.
func ModuleRoot(t *testing.T) string {
	t.Helper()
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}
