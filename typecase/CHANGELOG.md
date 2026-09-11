# Changelog

All notable changes to Typecase are documented here. Versioning follows
[semantic versioning](https://semver.org/). This file tells consumers what to
do, not what the authors did.

## typecase/v0.1.1 — 2026-09-05

Visual audit fixes from the catalog browser review (findings В-1…В-7) and
follow-up gate/browser checks (В-8…В-10).

### For consumers (v0.1.0 → v0.1.1)

- **No breaking API changes.** CSS, markup, and gate checks only. Run `go test ./...`
  from the module root after upgrading.

### Fixed

- `.tc-btn` and ellipsis labels: `line-height` raised so descenders are not clipped
  inside `overflow: hidden` (`В-1`).
- `gate.CheckCompleteness` now requires every `tc-*__*` class in catalog markup to
  have a stylesheet rule or an explicit hook (`В-6`). Bare variant modifiers
  (`tc-row--lead` and similar) stay outside gate scope; see `CatalogClassHooks`
  (`В-8`).
- `.tc-row__text`: column layout so title and subtitle stack (`В-2`).
- `tc-tree`: children moved out of the node flex row (`В-3`).
- `tc-row--avatar` fallback: no longer puts subtitle in `<img src>`; initials use
  the first letter of the title (`В-4`).
- `tc-queue__row--*`: distinct styles per state in the catalog (`В-5`).
- Catalog page: containing blocks for menus and search popovers; browser check
  covers `position: absolute` as well as `fixed` (`В-7`).

### Included

- Browser checks: ink overflow (`fontBoundingBox` vs line box), row text stacking,
  tree label width, queue state fingerprints (colour, weight, decoration,
  opacity), popover containment.
- `gate.CatalogClassHooks` for element classes intentionally without CSS.
- Stylesheet rules for catalog element classes that were markup-only.
- `TestBite_completeness_elementClass` runs through `gate.Run` (`В-9`).
- `TestBite_catalogQueueStates`: removing the idle rule fails the queue check (`В-10`).

## typecase/v0.1.0 — 2026-09-05

First tagged release after the pre-tag reviews. Upgrade in one step from
`v0.0.5` and run `go test ./...` from the module root before deploying.

### For consumers (v0.0.5 → v0.1.0)

- **Body fields are `Prose`, not HTML strings.** `Overlay.Body`, `Sheet.Body`,
  `Dialog.Body`, `Doc.Body`, `Main.Body`, `Card.Body`, and `Sec.Body` now take
  `widget.Prose`. Use `widget.Paragraph("text")` for a single paragraph, or build
  `Prose{Blocks: ...}` for richer content. This is a breaking change from
  v0.0.5, where those fields were unescaped `string` markup.
- **Disabled controls use `Reason`, not `Enabled`.** `Button`, `Action`, `Check`,
  and `Tick` no longer have an `Enabled` field. Set `Reason` to a non-empty
  string to disable; leave `Reason` empty for an enabled control.
- **IDs are generated inside widgets.** `Menu`, `Sheet`, `Dialog`, `Side`,
  `Field`, `Input`, and `Check` assign unique `id` values automatically. Pass
  `ID` only when you need a stable reference for tests or `aria-controls`.
- **`RowMeta` replaces `RowFile`.** The row variant constant is now `RowMeta`
  (`"meta"`); CSS class is `tc-row--meta`. Update any code that used
  `RowFile` / `tc-row--file`.
- **CSS-bound fields use named enum types.** `Brand.Size`, `Avatar.Size`,
  `Skel.Width`, `Rail.Layout`, `Nav.Layout`, `Thumb.Size`, `ProseBlock.Kind`,
  `Step.Status`, `RunItem.State`, and `RailSection.State` are no longer plain
  `string`. `Mark.Colour` and `Avatar.Colour` use `MarkColour`.
- **Localise UI strings with `widget.SetLabels`.** Call before rendering to
  replace defaults such as "Close", "Cancel", "Search", and "More". Use
  `widget.SetStepOfFunc` for step progress text.
- **Run the full check suite** with `go test ./...` from the module root. L-8
  gates live in `gate/`; L-9.2 browser checks in `browser/` need Chrome or
  Edge (`CHROME_PATH` overrides detection).

### Included

- `gate.CheckDocInput`: L-3.2 «Вход» field lists must match exported struct fields.
- `gate.CheckDomainNaming`: const names, enum string values, and L-3.2 «Состояния»
  slugs are checked without the catalog whitelist.
- `gate.CheckInvariantCoverage`: every invariant clause has a `gate:inv-*` markup
  check; `todo:` bindings are rejected; 72 bite fixtures under
  `gate/testdata/inv/`.
- `gate.CheckSpecL4`: implemented widgets must match the L-4 registry in
  `docs/spec/typecase.md`.
- `gate.CheckGofmt` and `gate.CheckVet` run from `gate.Run` via `go test ./...`.
- `gate.CheckOwnership`: scans git repository root, skips the typecase module by path.
- `tc.js`: focus trap lifecycle (`attachTrap` / `releaseOverlayTrap`); no
  autofocus on pre-open overlays at load.
- `gate.CheckUniqueIDs`: catalog HTML must not repeat `id` attributes.
- `gate.CheckA11y`: each button and field in catalog HTML is checked individually.
- `widget.Paragraph`, `widget.SetLabels`, `widget.SetStepOfFunc`, internal
  `uniqueID` for collision-free markup.
- `gate.CheckVerifiable` and bite tests that call `gate.Run` for every gate.
- `tc-table`, `tc-grid`, `tc-queue`, and `tc-runbar`: behaviour moved to
  «Чего нет» (consumer-owned); orphan `data-tc-*` hooks removed.
- `tc-diff`: arrow keys documented as native radio-group behaviour.
- `tc.js`: menu keyboard navigation (arrows, Home/End, first letter), seg
  arrows, tree toggle/selection, search clear, rail drawer, bar collapse,
  side-panel close, crop controls, list arrows, range validation, tickrow toggle.
- `tc-tree` disabled nodes: `disabled` button and `pointer-events: none`.
- `tc-row` avatar/tick variants read `RowCells.Avatar` and `RowCells.Done`.
- `tc-stat` and `tc-stats`: pre-formatted `Stat.Value` string; equal-width row
  with `flex-wrap: nowrap`.
- Catalog page styles served as `/catalog.css` (strict `style-src` on the demo).
- `Column.Kind` defaults to `ColText` when empty in table headers.

### Breaking changes from v0.0.5

- `Body string` → `Body Prose` on overlay, document, shell, and utility widgets.
- `Button.Enabled` / `Action.Enabled` removed; use `Reason`.
- `RowFile` / `tc-row--file` renamed to `RowMeta` / `tc-row--meta`.
- `RowCells` gained `Done` and `Tags`; avatar variant prefers `Avatar` field.

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
