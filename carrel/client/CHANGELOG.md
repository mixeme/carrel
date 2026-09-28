# Changelog

All notable changes to the Carrel client are documented here. Numbers like
`v0.0.N` name steps in this file only — they are not git tags unless an
entry lists **Tagged:**. The plan is [docs/plan/client.md](../../docs/plan/client.md).

## v0.0.2 — 2026-09-28

Typecase is connected, and the guards of stage 2 that need it are built. The
first screen, the sign-in page, is not: its frame is built by the widgets,
but not with the meaning of HTML (docs/plan/client.md, «Что блокирует») —
stopped for the owner.

### Added

- **Typecase** in `go.mod`: `require carrel.dev/typecase`, `replace
  carrel.dev/typecase => ../../../typecase` (owner's decision 3 of
  28.09.2026), checked on Typecase `6f5a896` (0.8.0 and Carrel's tasks
  36–38 after it; the neighbour's tree had uncommitted work), and `tool
  carrel.dev/typecase/cmd/frame2go` — the draft is `go tool frame2go -self`.
  `chromedp` enters the module graph; no package of the client imports it.
- **`screens.All`** — the registry of the screens built: route of C-5, frame,
  view model, drawing. Empty.
- **Frame comparison** (U-15.2, 4; `guard/screens_test.go`): each screen,
  drawn on every fixture of its route (`FIXTURE_SCREEN` of
  `docs/spec/check.py`), has its frame's make-up (`shape.Check`), and every
  class it prints is a class of Typecase's CSS or of its list
  `names-without-rule`; its frame is the one whose `data-screen` names the
  route, and is the same in Carrel's and Typecase's copy of the mockup. Bites:
  a wrapper, another variant, a widget less, a class of the screen's own, a
  frame that differs between the copies, a screen naming another frame.
- **Kinship with the draft** (U-15.2, 3; `guard.Kin`, `guard.Skeleton`,
  `guard.Drafts`): the draft of each screen's frame is printed again, built
  as printed in a directory of `guard/testdata` made for the run, and its
  tree of widget values is held against the screen's, drawn on the normal
  fixture: widget types, variants (fields of the widget package's enums),
  parts; leaf data and the meaning of HTML are not compared; neighbours of
  one type are a run of any length. Bites: a wrapper, another variant, a
  widget less; the rules on trees alone.
- **Addresses** (U-15.2, 5; `guard.CheckAddresses`): each screen's route is
  a page or sheet of C-5 and built once, each row of «ждёт библиотеку» names
  one, none is both. The addresses neither built nor waiting are counted,
  and red with `CARREL_ACCEPTANCE=1`. Bite: `guard/testdata/addresses`.

## v0.0.1 — 2026-09-28

The module, without Typecase (owner's decision 3 of 28.09.2026): stage 1 of
the plan in part and the guards of stage 2 that need no library. It draws
nothing yet, and the server does not import it: it answers JSON until the
client draws HTML.

### Added

- **Module `carrel.dev/carrel/client`** in `carrel/client/`: requires the
  server module (`replace ../server`), imports only its `view` and `render`.
- **`zone`** — the part of a page's view model that a target of C-7.2 shows:
  a page in the shell has `main` (the page without its shell), `rail`,
  `details`, `notices`, `topbar`, and `poll` or `queue` where it carries
  them; a page outside the shell has `main` only, the whole page; a sheet has
  `sheet` only. A target the page lacks is `ErrNoZone`, not an empty zone.
  Held on every fixture of `docs/spec/fixtures` against C-5 and the table
  «Где зона во вью-модели» of C-7.2.
- **`screens`** — empty; its doc names the rules U-5.1, U-5.2, U-5.5, U-5.7.
- **`guard`** — the source guards of U-15.2, each red on a broken sample in
  `guard/testdata` first: the imports of `screens/` (only `view`, Typecase's
  `widget`, `strconv`, `slices`); no markup, `tc-`, `class=`, `style=` or
  `data-tc` in a string or rune constant; no stylesheet, page, template,
  script or picture file; the marks `специфика экрана: U-5.7 #N` and
  `ждёт библиотеку: U-5.5 #N` against the two registries of
  carrel-client.md, both ways, and no row without the owner's decision.
