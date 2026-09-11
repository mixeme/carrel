# Changelog

All notable changes to Typecase are documented here. Versioning follows
[semantic versioning](https://semver.org/). This file tells consumers what to
do, not what the authors did.

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

- L-8 gate tests beyond token validation, L-9 browser checks (chromedp), and
  Opus API review.

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
