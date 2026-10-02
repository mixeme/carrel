# Changelog

All notable changes to the Carrel client are documented here. Numbers like
`v0.0.N` name steps in this file only — they are not git tags unless an
entry lists **Tagged:**. The plan is [docs/plan/client.md](../../docs/plan/client.md).

## v0.1.4 — 2026-10-02

Typecase `9e69804` (Carrel's task 77, the calendar's frames; the live layer
`tc-live.js`, L-1), server `v0.2.135`. The drafts of every frame the
screens draw print the same as on `29413d6`; no row of U-5.5 is resolved
by it, and none is added — the guards and the screens are green as they
were.

### Fixed

- **The version of the library's files** (`document.Version`) is taken in
  the order of the files' names: it was taken in a map's order, and the
  same files could give another version on the next start, so a browser
  fetched them again. It covers the live layer too: `page.Static` serves
  `js/tc-live.js` beside `tc.js` whether a page names it or not, and a new
  layer now gets a new address. `TestVersionCoversEveryScript` red before
  the fix (both).

### Not yet

- **No page loads the live layer** (`page.Document.Live` stays false; the
  library prints it only when asked): the client takes it with its stage 4,
  after Typecase's tasks 84 and 85 (`TestNoPageLoadsTheLiveLayer`).

## v0.1.3 — 2026-10-02

Server `v0.2.121`.

### Added

- **Fixture type `app-event-new`** in `fixtures.Models` (`EventEditPage`):
  the full form of a new event, the fixture the server added (Д9).

## v0.1.2 — 2026-10-01

Typecase `29413d6` (Carrel's tasks 60–76; the mockup's copy byte for byte
the same), server `v0.2.115`. The drafts whose print changed and those of the
new state frames went in first, as printed, under `screens/*/_draft/`; the
screens over them took the drafts out.

### Added

- **Closed menus that open without a script** (Typecase task 60): the «⋯» of
  a card's head and of the card in a panel, Columns with its menu — a form of
  the columns, their handles, Apply and «Reset to defaults»
  (`bind.Columns`) — and the account's menu: who is signed in and where, the
  items of `TopBar.User`, Sign out an item that posts its form (task 68,
  `bind.MenuItems`, `bind.MenuForms`, `bind.TopForms`).
- **The session's token in a form of an action** (`Action.CSRF`, C-11):
  Delete, Send, a source's retry, Sign out post with `_csrf` without a script.
- **The bar's tick is a form** (task 67, `bind.CheckForm`, `bind.Checked`):
  «Compact rows» goes with the list's choices, Apply without a script. **The
  filter sends the list's other choices** (task 65, `bind.Filter`). **Tags
  go to the list filtered by them** (task 63, `bind.Tag`). **The photo in a
  card's head** (task 64). **The search's and the filter's words are their
  placeholder** (task 69).
- **The states of contacts are held against their frames** (task 66,
  `states-7` … `-12`; `STATE_FRAME` in `check.py`), the read-only book and
  the edits against frames drawn whole (task 62).
- `bind.Notices` (a notice and its technical block, `bind.NoticeDiag`; a
  message's buttons small), `bind.EmptyParts` (an empty state: its title
  bold, its words, its buttons). Tests for each.
- **The meaning guard's table** (Ж9): `TimeRow`, `TableCell` with its pick,
  `TickBox` of a menu's item, `CheckForm`, `Tag`, the button of a closed
  menu, a menu's item that submits a form.
- U-5.5 rows 17–22 (the owner's decision of 01.10.2026): what the strict
  frame comparison (task 76) found that is not the client's — the server's
  debts C18–C21 and Typecase's candidates (plan, «Подъём на Typecase
  `29413d6`»).

### Changed

- **A feed's row in the rail** is the feed's (`is-ext`), its mark only its
  colour, as every frame draws it (Ж8).
- **A loss on save** is a notice in the plain tone (`states-12`).
- **The list**: its notices under the head, its bar while there are cards or
  a filter to clear, its empty state as the frames draw it. **The edit**: an
  empty field shows the dash, but the one with the cursor; the line under
  the form only for a card that exists.
- `check.py`: check 9 does not hold the Apply of a bar's tick (the library's
  button) against an action, and keeps `SERVER_TODO` («Tick every book»).

### Removed

- U-5.5 rows 1–16: each wait is lifted by Typecase's tasks 60–69 or by the
  server (C17), or stands now on a row of its own.

## v0.1.1 — 2026-10-01

Server `v0.2.100`.

### Changed

- **The timeline's panel closes.** The cross of the card in the panel
  (`s-contacts-7`) leads to `Card.Close` — the same timeline with `?card=0`;
  a closed panel (`CardClosed`) is not drawn. U-5.5 row #14 is gone.

## v0.1.0 — 2026-10-01

The shell and the contacts section: the first screens inside the sign-in.
Typecase `8c99103` (`4af7ad7` and a test of its ladder; widgets, CSS and the
mockup the same), server `v0.2.94`.

### Added

- **The list of contacts** (`/app/contacts`, `/app/contacts/{account}/{col}`,
  `s-contacts-1`, read-only book `states-4`) from `ContactsListPage`; **the
  card** (`/app/contacts/{account}/{col}/{uid}`, `s-contacts-6`, after the
  rewrite `s-contacts-16`) from `ContactPage`; **the timeline**
  (`…/{uid}/timeline`, `s-contacts-7`) from `ContactTimelinePage`; **the
  edit and the full new form** (`…/{uid}/edit`, `…/new`, `s-contacts-3`,
  `-4`, `-27`) from `ContactEditPage`. Each a draft of `frame2go -self`
  committed as printed, then the screen over it.
- **The shell in `bind`** (`bind/shell.go`): the top bar (`Top`: the search
  as a GET form under `SearchBox.Name`, New with its arrow as a closed menu,
  the refresh, the account's button), the menu items (`MenuItems`), the rail
  (`NavItem`, `TickRow`, `Sources`, `Tags`, `RailForm`, `RetryForms`,
  `Foot`), the poll's bar (`Running`, `Runbar`, C-6.4), the notices
  (`Notice`, `NoticeForms`), `Badge`, `Avatar`, a card's line (`FieldsItem`,
  `Book`), `SegOption`, `Submit`, `ActionForm`, `Diag`, and of a form's
  repeating field `Remove`, `Add` (with its `Template`), the loss of a save
  (`Loss`). The screen's words come in with the screen (`TopWords`,
  `RailWords`). A test for each.
- **Zones of the shell in the renderer**: a fragment of `main`, `details`,
  `rail`, `notices` and `poll` of a page in the shell is the part its screen
  draws (`renderer.ShellZone`), with the page's forms; `queue`, `topbar`,
  out-of-band updates and events stay JSON.
- **Waiting for the library beyond an address** (the owner's decision,
  U-5.5): a row names a state (fixture) or a control; `Screen.Waits`; a
  waiting state is drawn and not held against a frame; a waiting control's
  meaning finding is excused; a wait with nothing to excuse is red. Rows
  1–16 of U-5.5 for the contacts section (Typecase tasks 60–66, a new one for
  the bar's tick, and the server's debts C16, C17).
- **The meaning guard's table** grows: `TickRow` (the rail of return sends
  nothing), `FaceRow`, `FindRow`, `LinkedRow`, `About`, `Range` (a GET form),
  `FieldsItem`, `SegOption`, `Seg` of links.

### Changed

- **Kinship takes shape's rule** (the lead's decision): a button with its
  menu (`Drop`, `TopDrop`) is its button.
- **A state's frame is STATE_FRAME's**, whether or not it names the route.

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
