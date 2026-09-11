# Changelog

All notable changes to Typecase are documented here. Versioning follows
[semantic versioning](https://semver.org/). This file tells consumers what to
do, not what the authors did.

## Unreleased — v0.1.0 is on hold

The pre-tag API review (L-11.2) found ten blocking discrepancies, so no tag was
cut. Findings Б-1…Б-10 are in `docs/plan/reviews.md`. Nothing in the module
changed in this entry; it exists so that anyone already building against
`v0.0.5` knows which parts of the public API are going to move.

### For consumers

- **Do not pass markup to a `Body` field.** `Overlay.Body`, `Sheet.Body`,
  `Dialog.Body`, `Doc.Body`, `Main.Body`, `Card.Body`, and `Sec.Body` are
  `string` and are written to the output **unescaped** today. That is a defect,
  not a feature: it contradicts L-3.1 mechanism 1, and consumer data reaching
  one of those fields is an injection hole. These fields will become typed
  block structures (as `Prose.Blocks` already is) before v0.1.0, which is a
  breaking change. Until then, escape anything you put there yourself, and do
  not route user-supplied text through them.
- **Stop setting `Button.Enabled` / `Action.Enabled`.** The field is never read.
  Disabled state is derived from `Reason != ""`, so `Button{Enabled: false}`
  renders an enabled button. The field will be removed and disabled state will
  become one enumerated field shared by `Button`, `Check`, and `Tick`.
- **Give every `Menu`, `Sheet`, `Dialog`, and `Side` an explicit unique `ID`.**
  The defaults collide: a page with two menus emits `id="tc-menu"` twice, and
  `aria-controls` then resolves to the wrong popover. `Row` menus are worse —
  every row hardcodes `id="tc-row-menu"`. Uniqueness will move inside the
  widget; until it does, pass your own.
- **Do not rely on the behavior documented under «Поведение» for**
  `tc-seg`, `tc-tree`, `tc-search`, `tc-rail`, `tc-shell`, `tc-bar`, `tc-diag`,
  `tc-crop`, `tc-range`, `tc-list`, or `tc-side`. Those widgets emit
  `data-tc-*` hooks that `tc.js` has no handler for: the search clear button
  does not clear, tree nodes do not expand, the rail does not slide out, and the
  bar does not recompute on resize. Menus support click and `Esc` only — no
  arrow keys, `Home`/`End`, or first-letter jump.
- **`tc-row` variants `--avatar` and `--tick` read the wrong cells.**
  `--avatar` uses `Cells.Title` as initials and `Cells.Subtitle` as the image
  `src`; `Cells.Avatar` is ignored. `--tick` derives its checkmark from
  `Cells.Progress`. Both will change shape.
- **`tc-field` with `Check` set produces nested `<label>` elements** and a
  dangling `for`. Render `tc-check` directly instead until this is fixed.
- **A disabled `tc-tree` node is still selectable.** `Tree.Disabled` currently
  only dims the node; the label stays a live, focusable button.
- **Widget text is hard-coded English** and cannot be supplied by the consumer:
  "Close", "Cancel", "Search", "Clear", "All results", "Loading", "Step N of M",
  "Main", "Breadcrumb", "More", "unavailable", "Unsaved changes", "Toggle",
  "Recent", "Build information missing". Label fields will be added to the input
  of the affected widgets before v0.1.0.

### Gates

Five L-8 gates and the L-9.3 bite requirement were marked done in the plan and
are not: typed input, enumerations, role naming, verifiability, and
accessibility. The verifiability gate has no branch in `gate.Run` at all, and
three bite tests exercise helper functions that the real gates never call. The
plan's checkboxes have been reverted to match.

`gofmt`, `go vet`, and `go test ./...` remain green, and the L-9.2 browser
checks do run a real browser and do go red on their broken fixtures.

## typecase/v0.0.5 — 2026-09-04

Browser checks that actually run, and a readable catalog.

### For consumers

- The L-9.2 checks now run on Microsoft Edge as well as Chrome. The harness
  starts the browser itself and connects over the DevTools endpoint published
  in the profile directory, instead of relying on the "DevTools listening" line
  on stderr — a browser built as a Windows GUI subsystem binary writes nothing
  to an inherited pipe, so every check previously failed with `chrome failed to
  start` on a machine that has only Edge. `CHROME_PATH` still overrides
  detection.
- One browser process now serves every check in a run; viewports come from
  device metrics rather than a window-size flag.
- The catalog page loads `/theme.js` in `<head>`. The route was served but
  never linked, so the catalog rendered without a resolved theme. Both pages
  carry it: the theme boot script must precede first paint (L-5.3) and is not
  widget behavior. `/static` still withholds `tc.js`.
- The catalog gives each cell holding a scrim, overlay, or rail drawer its own
  containing block. Those widgets are fixed to the viewport by design, so on a
  page that shows every state at once a single open scrim covered the whole
  catalog and hid all 139 entries.

### Included

- `browser`: own launcher (`--remote-debugging-port=0` plus `DevToolsActivePort`)
  behind `chromedp.NewRemoteAllocator`; `chromedp.EmulateViewport` per check.
- `browser/catalog_test.go`: measures that no viewport-fixed widget escapes its
  catalog cell and that the theme attribute is set on load.

### Fixed

- Settle waits in the browser checks were untyped constants passed to
  `chromedp.Sleep`, so `Sleep(80)` waited 80 nanoseconds instead of 80
  milliseconds; menu-open and theme-switch checks measured before the page had
  settled.
- `browser/fixtures.go` was not `gofmt`-clean, failing the first-commit gate.

### Not yet included

- Opus API review.

## typecase/v0.0.4 — 2026-09-04

L-9.2 browser checks (chromedp) and menu popover positioning.

### For consumers

- Run `go test ./...` from the module root to execute L-8 gate checks and L-9.2
  browser checks in the `browser` package. Set `CHROME_PATH` if Chrome is not on
  `PATH` (Microsoft Edge is used automatically on Windows when Chrome is absent).
- `tc-menu` popovers flip upward when there is not enough space below the
  control; long labels no longer overlap the arrow.

### Included

- `browser` package: chromedp v0.16.0 checks for menu geometry, ellipsis,
  single-row bar, 44 px tap targets, focus return, stacked `Esc`, WCAG AA
  contrast, and stable geometry across themes.
- L-9.3 bite fixtures under `browser/testdata/broken/`.
- `tc.js` `positionPopover()` and `is-flip-up` CSS for menus.

### Not yet included

- Opus API review.

## typecase/v0.0.3 — 2026-09-04

L-8 gates, full catalog coverage, and markup without inline styles.

### For consumers

- Run `go test ./...` from the module root to execute L-8 gate checks in the
  `gate` package. Each gate has a bite test under `gate/testdata/broken/` (L-9.3).
- The live catalog now includes every widget in each declared state. Start it
  with `go run ./cmd/catalog` as before; `/static` still serves the same set
  without `tc.js`.
- Widget markup no longer emits `style=` attributes. Dynamic colours, meter
  width, skeleton width, and crop transforms are carried in `data-tc-*`
  attributes; `tc.js` applies them on load.
- `Input` accepts an optional `Label` field used for `aria-label` when the
  control is not wrapped in `tc-field`.

### Included

- `gate` package: completeness, ownership, node names, typed input,
  enumerations, domain naming, doc format, catalog, no-inline, and a11y checks.
- Catalog entries for all missing widget states; `Shell.Narrow`, `Rail.Layout`.
- `tc.js` `initDynamic()` for data-driven presentation.

### Not yet included

- Opus API review.

## typecase/v0.0.2 — 2026-09-04

Widget families, catalog server, and expanded behavior script.

### For consumers

- Import `carrel.dev/typecase/widget` and call `Render()` on typed widget
  structs (for example `widget.Button{Label: "Save"}.Render()`). Every input
  field is typed; there are no HTML slots.
- Run the live catalog with `go run ./cmd/catalog` from the module root. Open
  `http://localhost:8080/` for the page with `tc.js`, or `/static` for the same
  widgets without behavior scripts.
- Styles now include family sheets (`02-primitives.css` through
  `10-utility.css`) concatenated in `css.Order`. Continue to serve via
  `typecase.CSS()`.
- `js/tc.js` handles menu open/close, overlay `Esc`, scrim dismiss, close
  buttons, and basic focus trapping inside `[data-tc-trap]`. Load after markup
  as before.
- Widget CSS is validated against the public `--tc-*` token surface; do not
  reference undeclared custom properties in overrides.

### Included

- All widget families from L-4: primitives, forms, overlays, panels, lists,
  tables/grids/trees, shell, document, and utility widgets.
- `catalog` package with every widget in each declared state.
- `widget` package tests: render smoke, HTML escaping, catalog completeness,
  per-family CSS token validation.

### Not yet included

- L-9.2 browser checks (chromedp), Opus API review.

## typecase/v0.0.1 — 2026-09-04

First publishable skeleton: module layout, token surface, asset delivery.

### For consumers

- Add `carrel.dev/typecase` as a module dependency (`go 1.27`).
- Call `typecase.CSS()` for the concatenated stylesheet; include it in HTML
  before widget markup.
- Load `typecase.ThemeJS()` in `<head>` **before** any stylesheet or body
  content so the theme is applied before first paint (L-5.3). Load
  `typecase.JS()` at the end of `<body>` for behavior.
- Serve font files from `typecase.Fonts()` at the same relative paths the CSS
  expects (`../fonts/…` from the stylesheet URL).
- Serve `typecase.Sprite()` for the neutral icon sprite; substitute your own
  drawings via the same delivery mechanism when integrating (L-5.1).
- Override appearance only through the documented `--tc-*` custom properties in
  `:root` and `[data-tc-theme="dark"]`. Do not add inline styles or handlers.
  Missing required tokens fail at bundle time, not at runtime with silent
  defaults.

### Included

- MIT-licensed module `carrel.dev/typecase` with `gofmt`, `go test`, and
  `go vet` gates.
- Palette tokens (ink, surface, background, rule, muted, accent, danger) for
  light and dark themes; typography, two densities, radius, and stroke.
- OFL fonts: Literata, Public Sans, Source Code Pro.
- Neutral geometric icon sprite (not application artwork).
- CSS concatenation in declared order (`css.Order`).
- `js/tc.js` (`data-tc-*`) and separate `js/theme.js`.

### Not yet included

- L-8 gate tests beyond token validation, L-9 browser checks (chromedp), and
  full widget catalog acceptance.
