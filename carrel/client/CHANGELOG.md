# Changelog

All notable changes to the Carrel client are documented here. Numbers like
`v0.0.N` name steps in this file only — they are not git tags unless an
entry lists **Tagged:**. The plan is [docs/plan/client.md](../../docs/plan/client.md).

## v0.0.7 — 2026-10-01

The client is wired into the server (`v0.2.90`): the binary answers the
pages that have a screen with them. Typecase `2b50f5e`.

### Added

- **`renderer`** — the server's `render.Renderer`. A view model whose type
  has a screen in `screens.All` (a value or a pointer) is a page:
  `document.Of` under the instance's base path, `text/html`, the server's
  status. A fragment is the screen's root and its forms for `main` of a page
  outside the shell and `sheet` of a sheet, when nothing goes out of band.
  Everything else — a model without a screen, the zones of the shell, the
  out-of-band updates, the events, `JSON` — is `render.JSONRenderer`'s
  answer, byte for byte, until the shell and Typecase's swapping of zones.
- **`document.Version`** — 12 hex digits of the SHA-256 of the library's
  files (stylesheet, both scripts, fonts); **`document.Immutable`** — their
  caching.
- **Checks of the renderer on every fixture** (`guard/renderer_test.go`): a
  built screen's page is its document, whose root as served passes the frame
  comparison (`shape`, the classes of Typecase's CSS) under both base paths,
  and so does the fragment of `main`; a model without a screen answers the
  JSON renderer's page, `main` and `sheet`; a built screen answers JSON for
  `rail`, `sheet`, `main` with an update out of band, `JSON` and an event.

### Changed

- **The library's files are at `document.Static/{version}/…`**
  (`/static/tc/{version}/css/typecase.css` and so on): `document.Assets`
  names them there, `document.Files` serves only them — `GET`, `HEAD`,
  `Cache-Control: public, max-age=31536000, immutable` on a file found;
  another version, a directory, an unknown name — 404, not kept. Fonts carry
  their type (`font/ttf`) whatever the system's tables say. Mount with
  `mux.Handle("GET "+document.Static+"/", document.Files())`.

## v0.0.6 — 2026-09-30

Every page outside the shell is on `main`. The types of the contract become
widgets in one package, `bind` (the owner's decision), and the screens call
it beside the widgets. Before the shell, the contacts section was scanned
in one pass for what its fixtures lack against its frames: the list of gaps
is in docs/plan/client.md, for the server's next round.

### Added

- **`bind`**: `Runs` into running text (`Inlines`; `Words` for a tagline),
  `Action` into a button that means what it does (`Button`: GET — `Href`,
  POST — the `Form` it submits; primary by `Primary`, destructive by
  `Danger`; switched off with its `Reason` and no address), a link (`Link`),
  words that lead (`Span`), an icon without its prefix (`Icon`, «i-down» →
  «down», U-4); `Form` into a form that sends (`Form`, `Hidden`, `Rows`) or
  a `PostForm`; `FormRow` into a field (`Field`, `Control`, `Input`, `Pick`,
  `Options`, `Area`, `Hint`: name, kind, value, pattern, monospaced, cursor,
  switched off when `ReadOnly`, required, the muted part of the label, the
  reason or the hint under it); `Notice` into a message (`Msg`, `Tone`).
  No markup, no words of its own, no variant the frame decides; a test for
  each translation.
- **Guards of `bind`**: the screens may import it (`AllowedImports`); it
  imports only what a screen may, not itself nor another package of the
  client (`BindImports`); no string constant in it holds a word, but the
  methods of HTTP and the icon's prefix (`Words`). Bites on
  `guard/testdata/bind`.
- **The page's forms** (`Screen.Forms`): the PostForms a page prints at its
  end, from the same view model; `document.Of` puts them in
  `Document.Forms`, and the meaning guard holds the buttons against them and
  each form (bite: a form with no address, two forms of one name).
- **Registration** (`/register`, `s-auth-2`, sent `s-auth-3`) from
  `RegisterPage`; **the first run** (`/setup`, `s-auth-4`) from `SetupPage`
  — Login monospaced with its pattern, «(optional)» after Email; **the
  forgotten password** (`/forgot`, `s-auth-5` … `s-auth-9`) from
  `ForgotPage` — the options as cards whose buttons submit the page's
  PostForms; **the link of the forgotten password** (`/forgot/{token}`,
  `s-auth-10`, `s-auth-11`) from `ForgotWipePage` — the destructive
  confirmation through a PostForm, then the new password. Each is its
  drafts unchanged, then the screen.
- **The meaning table knows `Span`**: words, with an address or without.
- **The batch scan of the contacts section** — docs/plan/client.md,
  «Раздел контактов: пробелы контракта»: 54 gaps of the contract (the shell
  10, the list 16, the card 11, the timeline 8, the edit 9), the frame's
  words without a field (one decision), 6 for Typecase and the guards.

### Changed

- **The five screens on `main` use `bind`** and lose their own helpers
  (`lede`, `lead`, `tone`); the invitation and the sign-in draw the form's
  button primary by `Action.Primary`, not every button.
- **Spec**: U-5.7 and U-15.2, 2 — the owner's decision about `bind`; U-5.1
  and U-5.5 — the batch scan before each section. The plan says how a
  section goes.
- **Typecase `ef80fb4`** (only `docs/roadmap.md` edited in the neighbour's
  tree; the same code, CSS and mockup as `d13f50c`). By the end of the
  work the neighbour moved to `7f726e6`; its mockup came to Carrel with
  `eb234e8`, and the gates are green on it (Typecase `1664ee1`, clean).

## v0.0.5 — 2026-09-30

A screen is the screen of a view model: the renderer will pick it by the
type of the body the server hands it. The page about the program, the
invitation and the confirmed address join the sign-in page on `main`. The
dead link is built and waits on the branch `client/rest-1` for one line of
`STATE_FRAME`; the registration, the first run and the forgotten password
wait on the contract — a piece of a sentence the frame marks, a field drawn
monospaced (docs/plan/client.md, «Что блокирует»).

### Changed

- **The registry is by view model** (`screens.All`): an entry is a screen of
  a view model (`Model`) with the frames it draws (`Frames`), no route. A
  screen's fixtures are those of its view model — the map of fixture
  directories to view models moved from the zones' test to the package
  `fixtures`, which both read —, so the dead link's fixture on the route of
  the invitation is the dead link's. Each fixture is held against the frame
  `check.py`'s check 8 gives it (`STATE_FRAME`, else its route's frame),
  which the screen must name, and each frame named is drawn by a fixture.
  Bites: another frame than the route's, a state's frame left out, a view
  model without fixtures.
- **An address is built by its view models** (`guard.CheckAddresses`, U-15.2,
  5): an address of C-5 is built when every view model its row answers with
  has a screen — the page's own and, on a secret link, `DeadLinkPage`; «те
  же, что строкой ниже» is the row below. Red always: a view model no page
  or sheet of C-5 answers with, one built twice, an address built and
  waiting, a page or sheet row with no view model. Bites for each, and for
  what a cell quotes that is no view model (`Issued`).
- **Typecase `f86a3f0`**, as in v0.0.4; the mockup copy of Carrel is now the
  same as Typecase's. The neighbour's tree became dirty during the work
  (Typecase's task 45); the drafts below are the files the clean `f86a3f0`
  prints.

### Added

- **The page about the program** (`/about`, `rest-3`) from `AboutPage`, the
  screen of the branch `client/rest-3` over its draft; the branch is gone.
- **The invitation** (`/invite/{token}`, `s-auth-12`) from `InvitePage`:
  draft, then screen — the title and who invited, the warning over the
  fields, the form's rows with the cursor where `FormRow.Focus` says, the
  address of an invitation by letter switched off (`ReadOnly`), a login taken
  marked and said under its field, «Create account» submitting the form by
  name.
- **The confirmed address** (`/confirm-email/{token}`, `rest-2`) from
  `ConfirmEmailPage`: draft, then screen — the mark, the title, what
  happened, «Sign in».
- **The draft of the dead link** (`rest-1`); its screen from `DeadLinkPage`
  is on the branch `client/rest-1`, red only in the frame of its fixture.

## v0.0.4 — 2026-09-30

The sign-in page is the first screen on `main`, with its refusal. The other
pages outside the shell wait: their frames put the cursor in a field the
server does not name yet, their states other than the normal one have no
fixture, and the dead link has no address of its own (docs/plan/client.md,
«Что блокирует»). The page about the program is built and waits on the
branch `client/rest-3` for the copy of the mockup.

### Changed

- **Typecase `f86a3f0`** (`784599e` and the roadmap after it; the
  neighbour's tree clean). `shape` compares an element with classes by its
  classes, so a link with an address is no difference from the frame's
  `span`; the draft ends its file with one newline, as gofmt wants.
- **A state is held against its own frame** (`guard/screens_test.go`):
  `STATE_FRAME` of `docs/spec/check.py` — the map its check 8 uses — gives
  the frame of a fixture's state (`login/refused` → `s-auth-13`). The frame
  comparison holds that fixture against the state's frame, which must name
  the route in its `data-screen` and be the same in both copies of the
  mockup; kinship holds it against the draft of the state's frame. Both
  maps of `check.py` are read by one reader. Bites: the refusal drawn as
  the empty sign-in (red on `login/refused` only), a state's frame of
  another route, copies parted on `s-auth-13`, the reason in another hint.

### Added

- **The draft of the sign-in page again** (`screens/login`, `s-auth-1`),
  printed by the current Typecase, a commit of its own.
- **The sign-in page** (`/login`, `s-auth-1`; the refusal `s-auth-13`) from
  `LoginPage`: the mark, the tagline (`Lede`, `Tagline`), the form with its
  `Action`, `Method`, hidden fields (CSRF) and fields by `Name`; «Sign in»
  submits the form by name; «Create an account» while `Register`, and
  «Forgotten your password?», with addresses. The cursor is where
  `FormRow.Focus` says; a field with an error is marked and says the reason
  under itself. In `screens.All`, every guard green on both fixtures.
- **The draft of the page about the program** (`screens/about`, `rest-3`),
  a commit of its own. The screen from `AboutPage` is on the branch
  `client/rest-3`: every guard green but one — the frame differs between
  the two copies of the mockup, as the prose after it changed in Typecase.

## v0.0.3 — 2026-09-29

Typecase 0.10.0 gives the sign-in page its meaning, and the client checks it
on every screen. The sign-in screen is built from its view model with that
meaning, but the frame comparison does not take its link yet (Typecase's
`shape` lags 0.10.0): the screen waits on the branch `client/s-auth-1`, its
draft is here (docs/plan/client.md, «Что блокирует»).

### Changed

- **Typecase `ab79062`** (0.10.0 and Carrel's tasks 40–44 after it; the
  neighbour's tree clean). `AuthStack.Links` are `[]widget.Link`: the
  guards' sample of the sign-in page prints its links without an address,
  as the frame draws them.
- **A screen draws a root of the library**: `screens.Root` is
  `widget.Root`, Typecase's closed interface — a shell, a page, an overlay
  (U-15.2, 1). The bite of a class of the screen's own is held on the page
  printed.
- **The registry imports the screens**: a file right in `screens/` may
  import a screen's package one level below; a screen importing another
  screen stays red (`guard.ScreensPath`).
- **`screens.Screen.Title`** — the document's title from the view model.

### Added

- **Meaning** (U-8, U-15.2; `guard.Meaning`, Typecase's
  `TestEveryControlHasMeaning` turned to the client's screens): each
  screen's tree of widget values, drawn on every fixture of its route, is
  held against a table of the contract by widget type — a link has its
  `Href`; a button leads (`Href`), submits a form by name (`Form`), submits
  its form (`Submit` in a `Form` with `Action`), adds a row (`Adds`), opens
  a sheet (`Opens`) or is disabled; a field has its `Name` and stands in a
  form that sends; a tick box sends (`Send`); a search and a filter are forms
  (`Action`); the form a button names is on the page. A widget with a field
  of meaning (`Href`, `Action`, `To`, `Form` as text, `Send`) the table does
  not know is a finding: the table grows with the screens. Bites: the
  sample of the sign-in page without meaning (the button, the link and the
  fields named), the rules on trees.
- **The draft of the sign-in page** (`screens/login`, frame `s-auth-1`):
  `go tool frame2go -self` as printed, a commit of its own (U-5.1).
- **`document`** — the page a screen stands in: Typecase's `page.Document`
  around the screen's root, the title from the view model, `lang="en"`,
  the library's files under the base path at `/static/tc` (`Assets`), and
  `Files` serving them (`page.Static`). No markup of its own (the guard of
  the module's strings); `TestDocument` — the page's head and root, nothing
  inline a strict content policy refuses; `TestFiles`. Not wired into
  `cmd/carrel` yet.

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
