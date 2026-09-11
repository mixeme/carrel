# Changelog

All notable changes to Typecase are documented here. Versioning follows
[semantic versioning](https://semver.org/). This file tells consumers what to
do, not what the authors did.

## Unreleased

### For consumers

- **Assemble screens through `widget.Shell`.** Pass `Nav`, `Rail`, `Main`, and `Side`
  as typed widgets. Empty `Rail` / `Side` no longer emit a column; `notes-edit`
  is a shell with `Top` + `Main` only.
- **Narrow layout.** At phone widths the rail column collapses to an off-screen
  drawer; an open side panel covers the viewport. `tc-bar` and `tc-crumbs` clamp
  to the viewport; middle crumbs collapse into a menu. The screen check measures
  those widgets at 375 px (it does not skip `.tc-main`).
- **Gate `screens-compose`** now scans all demo screen assemblers, not only
  `notes.go`.

### Included

- `widget.Shell`: optional `Nav` in the rail column; `IsZero()` on `Nav`,
  `Rail`, and `Side`; empty columns omitted from markup.
- Demo screens (`files`, `contacts`, `io`, `duplicates`, `states`, all notes
  frames) assembled via `shellFrame` and widgets — no `shellWrap`.
- `tc.js`: `updateCrumbs` (middle crumbs → menu, then clamp to the viewport),
  `clampBar`. Browser screen check measures `.tc-bar` / `.tc-crumbs` /
  `.tc-head` / `.tc-top` / `.tc-docbar` against the viewport at 375 px; other
  nodes use the clipped box so `overflow: auto` in `tc-main` does not flag
  every cell.
- CSS: narrow rail drawer, narrow side overlay, bar/crumbs/head/docbar overflow.

The typecase wordmark work documented below stays documentation-only until
released separately.

### For consumers (wordmark preview)

- **Nothing to do for the wordmark.** No module code, token, markup or behaviour
  changed for the wordmark; it is not part of this library release.
- **The wordmark belongs to the library's own pages** — the widget catalog, the
  README, a release page — and nowhere else. It marks the library, not an
  application built on it, so it does not go into a consumer's interface, and it
  never stands next to the marks of the application that depends on it.
- **Do not set the word in Literata and call it the mark.** The nests, the air
  inside them and the empty nest are measured, not typed; `typecase-logotype.svg`
  is the only file that carries them.
- **Below 164 px wide, do not use it.** That is where the hairline of “e” still
  holds 1.5 px. There is no hinted small version and no icon yet, so in square
  formats the library has no mark at all.

### Included

- `docs/visual/typecase-logotype.svg`: the master. Eight letters, each in its own
  nest, and one empty nest as the accent. Three fills, no strokes, no embedded
  text and no metadata, 8.1 KB.
- `docs/visual/typecase-logotype.html`: the construction reference — the single
  source of truth for the mark, in the genre of the Carrel references next to it.
  It records what was chosen and why, including the five rejected accent
  placements and the two rejected nest geometries.
- The instance is `opsz 12, wght 400`, and the reference shows the measurement
  behind it: across the optical size axis the hairline of “e” varies from 5.89 to
  1.34 units, so at `opsz 72` the word would need 564 px of width to keep that
  stroke at 1.5 px, against 164 px here. It is also the instance that sets the
  library's own text.
- The grid is derived, not designed: a nest is the letter's ink plus the
  instance's mean side bearing (3.24) on all four sides, which makes the nests
  sum to the font's own advances exactly (420.42 against 420.42). The partition
  between nests and the corner radius are one module — the stem of “p”, 8.77.
- The accent is an empty nest after the word, as wide as the average nest
  (52.55): in a case, an empty nest means the sort is already set in the line and
  its place is still there.
- `docs/visual/tools/`: `typecase_logotype.py` builds the mark from the font,
  `build_typecase_logotype.py` releases the master and the reference together,
  `checks.py` measures contours against the font, containment, gaps, the sums and
  the size limit. Every number printed in the reference comes from that
  measurement, so the document cannot drift away from the file it describes.

### Not yet included

- An icon. The 3×3 case sketches — `docs/visual/typecase-logo-v2.svg` and
  `typecase-logo-bg.svg`, which differ only by a background plate — are a
  separate piece of work with its own reference.
- A Cyrillic wordmark. There is none, and Literata set next to the mark stays
  text.

## typecase/v0.3.0 — 2026-09-07

Screens assembled from widgets. Geometry matches the mockups; notes frames no
longer bypass `tc-main`, `tc-bar`, or `tc-side`; `tc-seg` and `tc-top` collapse
when width runs short. Remaining demo pages still wrapped the shell by hand;
that leftover is closed in Unreleased. The narrow rail still has no open
control — [docs/plan/typecase-rail-drawer.md](../docs/plan/typecase-rail-drawer.md).

### For consumers (v0.2.0 → v0.3.0)

- **`Node` and `[]Node` for widget composition.** `Main.Body`, `Card.Body`,
  `Sec.Body`, `Bar.Left`, and `Bar.Right` now take `[]Node` (any value with
  `Render() string`), not `Prose` or `[]Action`. Use
  `widget.Paragraph("text")` as a `Node` inside the slice; for bar actions
  use `widget.ActionsAsNodes([]widget.Action{...})` or
  `[]widget.Node{widget.Action{Label: "Save"}}`.
- **`Sheet.Body` is `[]Node`.** Pass `[]widget.Node{widget.Paragraph("…")}` or
  fields and other widgets; a bare `Prose` value no longer compiles.
- **`RailSection.Body` for non-tickrow rail content.** When `Rows` is empty,
  `Body` renders instead (for example a neighbour `tc-tree`); do not set both.
- **`Top.Narrow` and `tc-top--narrow` are gone.** Narrow layout is computed in
  `tc.js` from width: search shrinks first, then the right group goes under
  `labels.More`. Do not pass window width as widget input.
- **Redefine tokens if you override the bundle.** `--tc-row-height` is now
  `38px`, `--tc-font-size` is `13px`, and three new required tokens appear:
  `--tc-page-gutter` (`20px`), `--tc-page-gutter-narrow` (`14px`), and
  `--tc-radius-sm` (`2px`). `token.Validate` fails without them. Defaults ship
  in the bundle; nothing to do if you use `typecase.CSS()` as is.
- **Default button height is bar (26 px), not form (28 px).** Bare `Button{}`
  and `Action{}` now render `tc-btn--bar`. Form footers still get
  `tc-btn--form` automatically; elsewhere pass `Size: ButtonSizeForm` when the
  control sits in a form.
- **Page gutter replaces ad-hoc horizontal padding.** Panels, rows and headers
  read `var(--tc-page-gutter)`; narrow shells swap it for
  `var(--tc-page-gutter-narrow)` via `.tc-shell--narrow`.
- **Run the screens demo beside the catalog.** `scripts/typecase-screens.bat`
  serves `http://localhost:8081/` — assembled mockup pages, not widget
  cells. Routes: `/notes`, `/notes-read`, `/notes-edit`, `/notes-quick`,
  `/files`, `/contacts`, `/io`, `/duplicates`, `/states`. Append `?theme=dark`
  for a dark rerender before first paint. `screens.css` holds page chrome
  only; it must not contain `.tc-` selectors.

### Included

- `widget.Node` and `[]Node` on `Main`, `Card`, `Sec`, and `Bar` shelves; gate
  `screens-compose` on `/notes`.
- `SideBlock` union (`*Sec`, `*Fields`, `*Field`); `/notes` side panel through
  `Side.Render()`.
- Empty `RailSection.Title` omits `tc-railsec__title`.
- `RowCells.Date` and `Source` on `tc-row--excerpt`; excerpt is one line with
  ellipsis.
- Narrow layout (L-4.4): `tc-seg` collapses to `tc-menu` when width runs short;
  `tc-top` shrinks search first and stashes the right group under `labels.More`.
- Notes demo frames: routes `notes-read`, `notes-edit`, and `notes-quick`; list
  frame drops inline `tc-doc` preview; read frame carries neighbour `tc-tree`
  in the rail; `RailSection.Body` and `Sheet.Body []Node`.
- Mockup-aligned geometry: row height 38, row gap 12, rail tickrow padding 16,
  top bar 52, default button 26, second radius 2 px on check/badge/mark.
- Gates `geometry`, `screens-css`, `screens-coverage`, and `screens-compose`.
- Package `typecase/screens`, binary `cmd/screens`, routes for s-notes,
  s-files, s-contacts, s-io, s-dup and m-states mockup frames.
- Browser check `screens_test.go`: each screen at 1280×900 and 375×812, light
  and dark (`?theme=dark`), including page-level horizontal overflow at 375 px.
- Demo chrome follows the palette tokens, so dark theme does not leave the
  screens nav in light ink. Theme links: `?theme=light` / `?theme=dark`.

### Fixed

- Screens browser probe: SVG `className` is not a string; `color-mix`
  backgrounds come back as `color(srgb …)` and now parse for contrast.
- Completeness: `tc-btn__icon` had no stylesheet rule.
- `tc.js` treated every `data-tc-seg` node as a group, including the option
  buttons; layout and keyboard now bind to `.tc-seg`.
- `tc-bar` restore after collapse: left-shelf items were inserted before the
  overflow menu, which lives on the right shelf, so a second layout pass threw.

### Not yet included

- Narrow rail has no open control; see
  [docs/plan/typecase-rail-drawer.md](../docs/plan/typecase-rail-drawer.md).
  Hand-wrapped shells and the 375 px `OutOfBounds` skip of `.tc-main` are
  closed in Unreleased.

## typecase/v0.2.0 — 2026-09-05

The second visual audit pass, closed (`В-12`…`В-27`). The palette gains an
eighth slot, small controls stop drawing their tap area, headings stop
inheriting the browser's scale, and sheets and dialogs are finally positioned
by the library.

### For consumers (v0.1.1 → v0.2.0)

- **Define `--tc-card` if you override the palette.** The eighth palette slot: a
  quiet surface tone one step off the page ground. `token.Validate` and
  `token.ValidateWidget` now require it, so a consumer stylesheet that redefines
  the palette without it fails validation. Defaults ship in the bundle:
  `#e8dfcb` light, `#242e27` dark. Nothing to do if you use the bundle as is.
- **Mark the root with `data-tc-root`, not the `tc-root` class.** Base
  typography, colour and `box-sizing` now hang on the attribute the markup
  already carries, so the library no longer needs a script to look like itself.
  `tc.js` stops adding the class; if your own CSS targeted `.tc-root`, retarget
  it at `[data-tc-root]`.
- **Small controls are drawn at their real size again.** `tc-tick` and the
  tick in `tc-tickrow` are 13 px, `tc-close` is 24 px. The 44 px tap area comes
  from a transparent pseudo-element below 720 px, so layouts that reserved
  44 px of space for these marks will now see them take 13 or 24. `tc-check`
  no longer forces the touch row height at every width.
- **Headings inherit the base size unless a widget names one.** If you styled
  around `h1` being 2em inside `tc-head`, drop those overrides: the title is
  `1.6em`, the sheet and dialog titles are body size in bold.
- **`tc-usermenu` markup changed.** The popover is a `div` wrapping the header
  and a `ul[role=menu]`; `aria-controls` points at the list. CSS or scripts
  that assumed `ul.tc-usermenu__popover` need updating.
- **`Badge` with an empty label renders nothing** instead of an empty chip.
- **`tc-seg` changed appearance.** The chosen part is `--tc-card` with ink text
  and `font-weight: 600`; the others are muted with no fill. A solid accent
  marks the primary action and the "done" state, and a view switch does not
  take that rank.
- **The side column's width moved to `.tc-side`.** `.tc-shell__side` is
  `width: auto`, so a closed panel collapses its column instead of holding
  400 px open.
- **Run `go test ./...`** from the module root after upgrading. The browser
  gates need Chrome or Edge (`CHROME_PATH` overrides detection).

### Fixed

- `tc-tick`, `tc-tickrow__tick`, `tc-close`, `tc-check`: the 44 px tap area was
  written as `min-width`/`min-height` on the mark itself, so it was painted at
  every width instead of being reserved for the narrow layout (`В-14`).
- `tc-tickrow` in the narrow layout spaces the tick and the label by `1.2em`, so
  the tick's tap area does not swallow taps meant for the label.
- Heading sizes: no `h1`–`h4` inside the library inherits the browser scale any
  more. `tc-head__title` is `1.6em/1.5` serif, `tc-sheet__title` and
  `tc-dialog__title` are body size in bold, `tc-empty__title` is `1.1em`, and
  the document heading scale is stated in `09-document.css` (`В-15`).
- `button`, `input`, `select`, `textarea` take `font: inherit`; `code`, `kbd`,
  `samp` and `pre` take the declared monospace. Nineteen controls were drawn
  with the system face (`В-16`).
- `tc-sheet` and `tc-dialog` are centred by their overlay layer, 44 px from the
  top, at `560px`/`92%` and `24em`/`90vw`. The layer is transparent to pointer
  events so the scrim still closes on a click outside (`В-17`).
- `tc-sheet__head`: the title leads and the target caption is pushed right;
  `width: 100%` on the caption used to squeeze it against the title (`В-18`).
- `Badge.Render` with no label renders nothing; `tc-usermenu` no longer emits
  the empty badge that its own `inv-badge-has-text` fixture describes, and the
  popover header no longer sits inside the `ul` (`В-19`).
- `.tc-main` fills its column, like `.tc-rail` and `.tc-side` (`В-20`).
- A closed side panel collapses its column (`В-21`).
- `tc-rail` draws no chip strip when there are no chips (`В-23`).
- Base styles hang on `[data-tc-root]`, so the library looks the same without
  `tc.js`; the static catalog page used to fall back to 16 px Times (`В-24`).
- `.tc-mark` has no palette default: without a supplied colour it is not
  painted, rather than painted with the theme accent (`В-25`).
- `tc-seg`: chosen part follows the mockups, carries a non-colour cue, and takes
  `flex: none`/`white-space: nowrap` so a part is never squeezed below its
  label (`В-26`).
- `tc.js` moves `is-chosen` together with `aria-pressed` when a segment is
  clicked; the visible and the announced selection used to diverge (`В-27`).

### Included

- `catalog.Mux`: the catalog site — pages, styles, scripts, fonts, icons and a
  sample image — built once and used by both `cmd/catalog` and the browser
  checks. Unknown paths answer 404 instead of the page, which is what made font
  faces fail to parse and images fail to decode (`В-12`).
- `--tc-card` in `css/00-tokens.css`, in `token.Palette`, and in the L-5.1
  palette list in `docs/spec/typecase.md`.
- `browser.TapTarget` measures the hit area with `elementFromPoint` at four
  points 21 px from the centre, across five controls, and separately checks the
  drawn size at 1280 px.
- `gate.compositeInvariants`: per-occurrence markup invariants read every
  catalog entry the widget is drawn in, not only the entry named after it.
- Browser checks for catalog assets, typography, overlay geometry, shell
  geometry, the no-script base, and segment selection agreement.
- Catalog fixtures carry a build caption; `tc-railfoot — missing` shows the
  unassembled-input state on purpose.

### Not yet included

- `tc-seg` does not collapse into a menu when width runs short (L-4.4).

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
