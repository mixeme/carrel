package browser_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"carrel.dev/typecase"
	"carrel.dev/typecase/browser"
	"carrel.dev/typecase/catalog"
)

// catalogServer serves the whole catalog site: the page plus every asset it
// links. Both this helper and cmd/catalog build the same mux, so a route the
// catalog forgets is missing here too (В-12).
func catalogServer(t *testing.T, withJS bool) *httptest.Server {
	return catalogServerCSS(t, withJS, "")
}

func catalogServerCSS(t *testing.T, withJS bool, cssOverride string) *httptest.Server {
	t.Helper()
	mux, err := catalog.Mux(cssOverride)
	if err != nil {
		t.Fatalf("catalog mux: %v", err)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// catalogURL is the page to open: with behaviour, or the static one.
func catalogURL(srv *httptest.Server, withJS bool) string {
	if withJS {
		return srv.URL
	}
	return srv.URL + "/static"
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
    const pad = (parseFloat(cs.paddingTop) || 0) + (parseFloat(cs.paddingBottom) || 0);
    if (fontBox > lh + pad + 0.5) {
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

// queueRowFingerprintJS returns how many distinct row-style fingerprints the six
// queue states have, and whether idle is visually muted. Colour alone is not enough:
// idle, running and cancelled share the base ink colour.
const queueRowFingerprintJS = `(() => {
  const states = ['idle', 'running', 'done', 'failed', 'partial', 'cancelled'];
  const prints = [];
  for (const st of states) {
    const row = document.querySelector('#tc-queue-' + st + ' .tc-queue__row');
    if (!row) continue;
    const label = row.querySelector('.tc-queue__label');
    const rowCS = getComputedStyle(row);
    const labelCS = getComputedStyle(label);
    prints.push([
      labelCS.color,
      labelCS.fontWeight,
      labelCS.textDecorationLine,
      rowCS.opacity
    ].join('|'));
  }
  const idleRow = document.querySelector('#tc-queue-idle .tc-queue__row');
  const idleMuted = idleRow && parseFloat(getComputedStyle(idleRow).opacity) < 0.9;
  return { distinct: new Set(prints).size, idleMuted };
})()`

type queueFingerprintProbe struct {
	Distinct  int  `json:"distinct"`
	IdleMuted bool `json:"idleMuted"`
}

func assertQueueStatesDistinct(t *testing.T, h *browser.Harness, srv *httptest.Server) {
	t.Helper()
	var probe queueFingerprintProbe
	if err := h.Run(srv.URL, 1280, 900, chromedp.Evaluate(queueRowFingerprintJS, &probe)); err != nil {
		t.Fatal(err)
	}
	if probe.Distinct < 6 {
		t.Fatalf("tc-queue states look alike: only %d distinct row fingerprints", probe.Distinct)
	}
	if !probe.IdleMuted {
		t.Fatal("tc-queue idle state is not visually muted")
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
	assertQueueStatesDistinct(t, h, srv)
}

func TestBite_catalogQueueStates(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("catalog queue bite needs Chrome")
	}
	css, err := typecase.CSS()
	if err != nil {
		t.Fatal(err)
	}
	css = strings.Replace(css, ".tc-queue__row--idle { opacity: 0.65; }\n", "", 1)
	srv := catalogServerCSS(t, true, css)
	h := browser.NewHarness(t)

	var probe queueFingerprintProbe
	if err := h.Run(srv.URL, 1280, 900, chromedp.Evaluate(queueRowFingerprintJS, &probe)); err != nil {
		t.Fatal(err)
	}
	if probe.Distinct >= 6 && probe.IdleMuted {
		t.Fatal("removing idle rule should fail queue state check")
	}
}

// assetsJS reports faces the page failed to load and images that did not
// decode. Both fail silently in a browser: a font that answers with HTML is
// dropped for the fallback, and a broken picture keeps its box (В-12).
const assetsJS = `(async () => {
  await document.fonts.ready;
  const faces = [];
  document.fonts.forEach(f => { if (f.status !== "loaded") faces.push(f.family + ":" + f.status); });
  const imgs = [];
  for (const i of document.images) { if (i.naturalWidth === 0) imgs.push(i.getAttribute("src")); }
  return { faces: faces, images: imgs, declared: document.fonts.size };
})()`

type assetProbe struct {
	Faces    []string `json:"faces"`
	Images   []string `json:"images"`
	Declared int      `json:"declared"`
}

// assetProbeAction evaluates assetsJS and waits for the promise it returns.
func assetProbeAction(dest *assetProbe) chromedp.Action {
	return chromedp.Evaluate(assetsJS, dest, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})
}

func TestCatalogAssetsLoad(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("catalog asset check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var probe assetProbe
	if err := h.Run(srv.URL, 1280, 900, assetProbeAction(&probe)); err != nil {
		t.Fatal(err)
	}
	if probe.Declared == 0 {
		t.Fatal("catalog declares no font faces")
	}
	if len(probe.Faces) > 0 {
		t.Errorf("font faces not loaded: %v", probe.Faces)
	}
	if len(probe.Images) > 0 {
		t.Errorf("images did not decode: %v", probe.Images)
	}
}

// TestBite_catalogAssets: a catch-all that answers every path with the page
// is exactly what the catalog did before В-12, and the check must bite it.
func TestBite_catalogAssets(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("catalog asset bite needs Chrome")
	}
	mux, err := catalog.Mux("")
	if err != nil {
		t.Fatal(err)
	}
	page := catalog.PageHTML("Typecase catalog", catalog.Entries(), true)
	catchAll := http.NewServeMux()
	catchAll.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/typecase.css", "/catalog.css", "/theme.js", "/tc.js":
			mux.ServeHTTP(w, r)
		default: // fonts and pictures get the page, as they used to
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(page))
		}
	})
	srv := httptest.NewServer(catchAll)
	t.Cleanup(srv.Close)
	h := browser.NewHarness(t)

	var probe assetProbe
	if err := h.Run(srv.URL, 1280, 900, assetProbeAction(&probe)); err != nil {
		t.Fatal(err)
	}
	if len(probe.Faces) == 0 && len(probe.Images) == 0 {
		t.Fatal("catch-all server should fail the asset check")
	}
}

// typographyJS reports text nodes drawn with a family the library never
// declared, and the ratio between the head title and its subtitle. A control
// that does not inherit the font falls back to the system face, and a heading
// without a size falls back to the browser's scale (В-15, В-16).
const typographyJS = `(() => {
  const declared = new Set(["Literata", "Public Sans", "Source Code Pro"]);
  const bad = new Set();
  for (const el of document.querySelectorAll(".catalog-demo *")) {
    let text = false;
    for (const n of el.childNodes) { if (n.nodeType === 3 && n.textContent.trim()) text = true; }
    if (!text) continue;
    const fam = getComputedStyle(el).fontFamily.split(",")[0].replace(/["']/g, "").trim();
    if (!declared.has(fam)) bad.add((el.className || el.tagName) + " → " + fam);
  }
  const title = document.querySelector("#tc-head-default .tc-head__title");
  const sub = document.querySelector("#tc-head-default .tc-head__sub");
  const size = el => parseFloat(getComputedStyle(el).fontSize);
  return {
    families: [...bad],
    ratio: title && sub ? size(title) / size(sub) : 0
  };
})()`

type typographyProbe struct {
	Families []string `json:"families"`
	Ratio    float64  `json:"ratio"`
}

func TestCatalogTypography(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("catalog typography check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var probe typographyProbe
	if err := h.Run(srv.URL, 1280, 900, chromedp.Evaluate(typographyJS, &probe)); err != nil {
		t.Fatal(err)
	}
	if len(probe.Families) > 0 {
		t.Errorf("text drawn with undeclared families: %v", probe.Families)
	}
	// 22px заголовка против 0.9em подзаголовка (≈11.7px).
	if probe.Ratio < 1.85 || probe.Ratio > 1.92 {
		t.Errorf("tc-head title/sub size ratio %.2f, want ≈1.88", probe.Ratio)
	}
}

// overlayGeometryJS reports sheets and dialogs that are not centred inside
// their own layer, and sheet heads whose target caption and title overlap.
const overlayGeometryJS = `(() => {
  const off = [], collide = [];
  for (const layer of document.querySelectorAll(".tc-overlay--sheet, .tc-overlay--dialog")) {
    const panel = layer.querySelector(".tc-sheet, .tc-dialog");
    if (!panel) continue;
    const l = layer.getBoundingClientRect(), p = panel.getBoundingClientRect();
    const delta = (p.x + p.width / 2) - (l.x + l.width / 2);
    if (Math.abs(delta) > 1) off.push(layer.id || layer.className + " off by " + delta.toFixed(1));
    if (p.y < l.y) off.push((layer.id || layer.className) + " above its layer");
  }
  for (const head of document.querySelectorAll(".tc-sheet__head")) {
    const target = head.querySelector(".tc-sheet__target");
    const title = head.querySelector(".tc-sheet__title");
    if (!target || !title) continue;
    const a = target.getBoundingClientRect(), b = title.getBoundingClientRect();
    if (a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom) {
      collide.push("target and title overlap");
    }
  }
  return { off: off, collide: collide };
})()`

type overlayGeometryProbe struct {
	Off     []string `json:"off"`
	Collide []string `json:"collide"`
}

func TestCatalogOverlayGeometry(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("overlay geometry check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var probe overlayGeometryProbe
	if err := h.Run(srv.URL, 1280, 900, chromedp.Evaluate(overlayGeometryJS, &probe)); err != nil {
		t.Fatal(err)
	}
	if len(probe.Off) > 0 {
		t.Errorf("overlay not centred in its layer: %v", probe.Off)
	}
	if len(probe.Collide) > 0 {
		t.Errorf("sheet head: %v", probe.Collide)
	}
}

// shellJS measures the shell: the work surface must fill its column, a closed
// side panel must not hold a column open, and neither an unset search nor an
// empty chip strip may be drawn (В-20…В-23).
const shellJS = `(() => {
  const shell = document.querySelector("#tc-shell-wide .tc-shell");
  if (!shell) return { missing: true };
  const h = el => el ? el.getBoundingClientRect().height : -1;
  const w = el => el ? el.getBoundingClientRect().width : -1;
  return {
    mainHeight: h(shell.querySelector(".tc-main")),
    columnHeight: h(shell.querySelector(".tc-shell__main")),
    sideWidth: w(shell.querySelector(".tc-shell__side")),
    emptyChips: [...shell.querySelectorAll(".tc-rail__chips")].filter(c => c.children.length === 0).length
  };
})()`

type shellProbe struct {
	Missing      bool    `json:"missing"`
	MainHeight   float64 `json:"mainHeight"`
	ColumnHeight float64 `json:"columnHeight"`
	SideWidth    float64 `json:"sideWidth"`
	EmptyChips   int     `json:"emptyChips"`
}

func TestCatalogShellGeometry(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("shell geometry check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var probe shellProbe
	if err := h.Run(srv.URL, 1280, 900, chromedp.Evaluate(shellJS, &probe)); err != nil {
		t.Fatal(err)
	}
	if probe.Missing {
		t.Fatal("no tc-shell entry in the catalog")
	}
	if probe.MainHeight+1 < probe.ColumnHeight {
		t.Errorf("tc-main %.1f does not fill its column %.1f", probe.MainHeight, probe.ColumnHeight)
	}
	if probe.SideWidth > 1 {
		t.Errorf("closed side panel still holds %.1fpx of column", probe.SideWidth)
	}
	if probe.EmptyChips != 0 {
		t.Errorf("rail draws %d empty chip strips", probe.EmptyChips)
	}
}

// noScriptJS reads the base typography the library promises without any
// behaviour script running. While the base hung on a class the script added,
// this page fell back to the browser defaults (В-24).
const noScriptJS = `(() => {
  const cs = getComputedStyle(document.body);
  const title = document.querySelector("#tc-head-default .tc-head__title");
  return {
    size: cs.fontSize,
    family: cs.fontFamily.split(",")[0].replace(/["']/g, "").trim(),
    titleSize: title ? getComputedStyle(title).fontSize : "",
    scripted: document.querySelectorAll("script[src$='/tc.js']").length
  };
})()`

type noScriptProbe struct {
	Size      string `json:"size"`
	Family    string `json:"family"`
	TitleSize string `json:"titleSize"`
	Scripted  int    `json:"scripted"`
}

func TestCatalogBaseWithoutScript(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("no-script base check needs Chrome")
	}
	srv := catalogServer(t, false)
	h := browser.NewHarness(t)

	var probe noScriptProbe
	if err := h.Run(catalogURL(srv, false), 1280, 900, chromedp.Evaluate(noScriptJS, &probe)); err != nil {
		t.Fatal(err)
	}
	if probe.Scripted != 0 {
		t.Fatal("static page must not load tc.js")
	}
	if probe.Size != "13px" {
		t.Errorf("base size without script is %s, want 13px", probe.Size)
	}
	if probe.Family != "Public Sans" {
		t.Errorf("base family without script is %q, want Public Sans", probe.Family)
	}
	if probe.TitleSize != "22px" {
		t.Errorf("tc-head title without script is %s, want 22px", probe.TitleSize)
	}
}

// segAfterClickJS clicks the second part of a segmented control and reports
// where the visible mark and the announced state ended up.
const segAfterClickJS = `(() => {
  const seg = document.querySelector("#tc-seg-default .tc-seg");
  if (!seg) return { missing: true };
  const opts = [...seg.querySelectorAll("button")];
  opts[1].click();
  const chosen = opts.filter(b => b.classList.contains("is-chosen"));
  const pressed = opts.filter(b => b.getAttribute("aria-pressed") === "true");
  return {
    chosen: chosen.length,
    pressed: pressed.length,
    same: chosen.length === 1 && pressed.length === 1 && chosen[0] === pressed[0]
  };
})()`

type segProbe struct {
	Missing bool `json:"missing"`
	Chosen  int  `json:"chosen"`
	Pressed int  `json:"pressed"`
	Same    bool `json:"same"`
}

func TestCatalogSegSelectionAgrees(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("seg selection check needs Chrome")
	}
	srv := catalogServer(t, true)
	h := browser.NewHarness(t)

	var probe segProbe
	if err := h.Run(srv.URL, 1280, 900, chromedp.Evaluate(segAfterClickJS, &probe)); err != nil {
		t.Fatal(err)
	}
	if probe.Missing {
		t.Fatal("no tc-seg entry in the catalog")
	}
	if !probe.Same {
		t.Errorf("after click: %d chosen, %d pressed, on the same part: %v",
			probe.Chosen, probe.Pressed, probe.Same)
	}
}
