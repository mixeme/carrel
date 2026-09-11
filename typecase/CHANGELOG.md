# Changelog

All notable changes to Typecase are documented here. Versioning follows
[semantic versioning](https://semver.org/). This file tells consumers what to
do, not what the authors did.

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
