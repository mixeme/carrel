# Changelog

All notable changes to the Carrel client are documented here. Numbers like
`v0.0.N` name steps in this file only — they are not git tags unless an
entry lists **Tagged:**. The plan is [docs/plan/client.md](../../docs/plan/client.md).

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
