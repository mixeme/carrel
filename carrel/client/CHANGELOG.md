# Changelog

All notable changes to the Carrel client are documented here. Numbers like
`v0.0.N` name steps in this file only — they are not git tags unless an
entry lists **Tagged:**. The plan is [docs/plan/client.md](../../docs/plan/client.md).

## v0.11.34 — 2026-11-16

Typecase `e92dd5c`, note 206; server `v0.2.417`. The whole client suite
passes.

- The tick of a task's row says in its hint what pressing does when the
  server says it (`TaskRow.Tick.Hint` → `Tick.Title`): «Repeating task:
  Done completes this occurrence and moves the task to the next one, 24
  Aug.» (s-tasks-21). A plain tick has none.
- Objects of a storage while its tree is walked is the number in the
  badge of work under way, «Counting: 4 so far.» in its hint; one waiting
  for the walk says «0» there; a walk refused mid-way is a dash in the
  failure's colour with the server's why (s-profile-37). Was «1240…», «…»
  and a plain dash.

## v0.11.33 — 2026-11-15

Typecase `2acba65`, note 205; server `v0.2.414`. The whole client suite
passes.

- U-5.5 #26 and #46 are closed: the event form with a loss report
  (states-17, -36) and the tasks and notes states (states-21, -23, -30)
  match their frames. The only row left is #22.

## v0.11.32 — 2026-11-15

Typecase `f1ebe7f`, note 204; server `v0.2.413`. The whole client suite
passes.

- Built with Typecase `f1ebe7f`: a press on a tag field's empty place, or on
  a tag that only shows, puts the cursor in its typing line (`tc.js`); a
  table cell's block keeps its button's 44 px zone on a phone. Nothing in
  the client changes; the client never read `FileRow.Upload`, which the
  server dropped.

## v0.11.31 — 2026-11-14

Typecase `eb7cfbe`, note 203; server `v0.2.411`. The whole client suite
passes.

- Built with Typecase `eb7cfbe`: the title, cell, excerpt or file name of a
  row that holds a link wraps on a phone. No Carrel screen puts a link
  there; nothing in the client changes.

## v0.11.30 — 2026-11-13

Typecase `ce8fa9d`, note 202; server `v0.2.410`. The whole client suite
passes.

- Built with Typecase `ce8fa9d`: one text edge for every block — the
  results stack, a list bled out of a card and a table's cards in a sheet
  take the page's gutter. Nothing in the client changes.

## v0.11.29 — 2026-11-13

Typecase `47e6f66`; server `v0.2.409`. The whole client suite passes.

- «Objects» of a storage while its tree is walked says how many files so
  far, «1240…» (`CollectionRow.Counting`), one waiting for the walk «…»;
  a walk refused mid-way leaves a
  dash with why in its hint (`CollectionRow.ObjectsNote`). The frame of
  the counting state is Typecase's task 262.

## v0.11.28 — 2026-11-12

Typecase `47e6f66`, notes 194–201; server `v0.2.408`. The whole client
suite passes.

- Inner paddings are steps, as the library names them (note 194):
  `HeadSnug` for the head's bottom of 6, 8 and 10, `FormColumnWide`,
  `FormBottom`, and a field's ticks `Gapped` in place of `Gap: 7` and `8`.
  The timeline's line under its rows is `EmptySmall`: the library prints
  the small empty state one way now.
- A list item's line is its own slot, `DocItemPart` (note 195): the words
  and the list nested in an item go in it part by part.
- Notes 196–201 change the library's stylesheet and script only: bars,
  tables, menus and the top bar fold to their room; nothing to change here.

## v0.11.27 — 2026-11-10

Typecase `2406c87`; server `v0.2.407`.

- An event's «Related to» is the records as tags with the line the next is
  typed in, as a task's and a note's (`bind.Related`): s-agenda-3, -15 and
  -32 match their frames, U-5.5 #44 is closed.
- U-5.5 #26 is narrowed to the two forms with a loss report: states-17 and
  -36 draw no «Other» and no «Source» (Typecase task 260).
- An empty list of tasks or notes draws its `Empty` without the bar
  (states-21, -28); a read-only one says «read-only» by its mark
  (`TasksPage.ReadOnly`, `NotesPage.ReadOnly`; states-23, -30).
- What the server did not keep of a note's save stands flush under the
  bar, as an event's and a contact's (states-31).
- The task, note and notes screens name states-19…-23, -26…-31.

## v0.11.26 — 2026-11-10

Typecase `2406c87`, notes 189–193; server `v0.2.406`. The whole client
suite passes.

- Gaps are steps, as the library names them (notes 190–192): lines
  `LineGapTight`, `LineGapNormal`, `LineGapWide`, `LineGapFields` (a
  `CardRow`'s `Gap` too); a section's list `StackGapTight`; the ticks in a
  column `GroupColumn`; `RubricBelow`, `DialogTitleGapTight`, `…Normal`,
  `MsgFlushAboveTight`, `MsgFlushAboveWide`, `MsgFlushBelowWide`,
  `MsgAroundNormal` in place of the numbered ones.
- The geometry fixes of note 189 are the library's stylesheet; the note's
  edit bar already gave H2, list, task, link and Auto-save the third rank
  in Focus too (s-notes-4).

## v0.11.25 — 2026-11-09

Typecase `911c960`, notes 180–188; server `v0.2.405`. The whole client
suite passes.

- An agenda row of a day an event goes on into says it — «day 1 of 3» —,
  and on paper a row prints its badges between the place and the source
  (print-1; notes 182, 183).
- A filter that is a form narrows the page's main as one types: the filter
  names `list-main`, the main of its screen has that ID (agenda, contacts,
  timeline, tasks, notes, folder, users, public pages; note 185).
- The rail's form asks the rail's zone; its answer is drawn as the rail, the
  form, and the poll and the list in their templates. Under a sheet and on
  paper the page's forms ask no zone (note 186).
- A file a new form no longer has is a row with «not kept» and a Remove that
  takes it off the page; the alert of a refused new form stands under the
  head (s-agenda-32, s-contacts-38; note 181).
- A published link that left records out says «1 record not in the link»
  by its name, why in the hint (s-publish-40).
- A tree goes four levels down (`TreeLevel76`, s-files-16; note 180).
- U-5.5 #45 is closed: s-publish-24 and -39 draw single rows (Typecase task
  258). s-agenda-32 waits with s-agenda-15 under U-5.5 #44 («Related to»).

## v0.11.24 — 2026-11-05

Typecase `664db01`, notes 178, 179 (sample data agree with the set); server
`v0.2.404`. The whole client suite passes.

- A public page draws its events and cards without a source column
  (`Single`, s-publish-6, -7), as its tasks and notes did. s-publish-24 and
  -39 still draw an empty source cell: U-5.5 #45 waits for Typecase's task
  258.
- The timeline of Анна Ковалёва has the note and the file the frame draws
  (s-contacts-7).

## v0.11.23 — 2026-11-05

Typecase `0510359`, notes 175–177 (Р-77, shorter hints; the pass done);
server `v0.2.403`. The whole client suite passes.

- The line under «Into» on an import's preview (s-io-6, -7): «Calendars
  only: a list of tasks is a different collection.» and «Lists of tasks
  only: a calendar is a different collection.»
- The restore's «Where the archive is» draws no hint under its switch
  (s-backup-7, -15, -16, -17): `RestoreBlock.fromHint` is gone from the
  server.

## v0.11.22 — 2026-11-05

Typecase `5fe384c`, notes 169–174 (Р-77, shorter hints; the feed panel
without an alert); server `v0.2.401`. The whole client suite passes.

- `Msg` is no longer a `SidePart` (Typecase, breaking): the panel
  «Subscribe to a feed» draws no message under its form, and
  `ConnectionsPage.addNote` is gone from the server.
- The lines under a folder's list (s-files-1): «Drop files onto the window
  to upload them here. A name already taken is refused, not overwritten.»;
  without scripts «Without scripts there is no drop and no queue:
  **Upload** sends the files. A name already taken is refused, not
  overwritten.», Upload in bold.

## v0.11.21 — 2026-11-04

Typecase `09827c3`, note 168 (Р-77, shorter hints); server `v0.2.399`.
The whole client suite passes.

- What the server did not keep of a save is its title in bold, then the
  properties (`bind.Loss`): `LossReport.Note` is gone from the contract
  (states-12, -17, -24, -31). The other words of the note are the
  server's.

## v0.11.20 — 2026-11-04

Typecase `99f680a`, notes 166 and 167 (Р-77, shorter hints); server
`v0.2.398`. The whole client suite passes.

- The hints are the server's words: nothing of the client changed. On
  s-plan-1 Generate profile has no line inside the unlock window — the
  server sends none, and the card draws none (the frame was red until the
  fixture was reshot).

## v0.11.19 — 2026-11-04

Typecase `8d83d14`, notes 162–165; server `v0.2.397`. The whole client
suite passes.

- A note's column has «Kept as the server sent them» above Source: folded
  under «Show 4», open a row a property in Source's monospace, a long text
  by its first line with «…»; then what they are (s-notes-2, -3, -27). On
  paper (print-5) the rubric and a field a property, the text whole.
- More tables keep a main column on a phone (`TableCol.Main`): Server of
  the selected storages (s-files-6), Objects of a subscription's «Lands in»
  (s-profile-1, -4, -10, -26, -27).
- The archive pick (s-backup-7) keeps its date on a narrow screen: the
  count of collections goes first (`Rank3`), then the size (`Rank4`).
- The queue row's words (task 252) and a heading inside a note in the
  text's font (Р-83) are Typecase's: nothing of the client changed.

## v0.11.18 — 2026-11-03

Typecase `3c73244`, note 161 (Р-66, the fourth group); server `v0.2.395`.
The whole client suite passes.

- More tables keep a main column on a phone (`TableCol.Main`): What
  happened of the setup check (s-admin-4) and of the DAV check (s-admin-29),
  Action of the audit log (s-admin-7), What of the password history
  (s-auth-15). A row that begins with a tick or a status mark is named by
  its next cell — Typecase's CSS, the markup the same.

## v0.11.17 — 2026-11-03

Typecase `08687ad`, note 160 (Р-66, the third group; one selection bar);
server `v0.2.395`. The whole client suite passes.

- More tables keep a main column on a phone (`TableCol.Main`): Type of the
  device setup (s-plan-4, -5), Verified of the copies' history (s-backup-1),
  Restore into of the matching (s-backup-8, -20), What happened of the
  restore report (s-backup-18).
- The selection bars of a folder and of the restore end with Select all
  and Clear selection, icon and label; the label is the title where the bar
  is narrow. Select all shows with a script only, as the frames draw it.

## v0.11.16 — 2026-11-02

Typecase `930ff57`, notes 157–159; server `v0.2.392`. The whole client
suite passes, the contacts list's kinship included.

- The import preview (s-io-5…8, -19): Result is the main column of a card
  on a phone; the columns are read from the end, so the table of one file —
  without Source now — and the notes' with File first are drawn alike.
- The notes list on paper (`?print=1`, print-8): the heading, the notebooks
  that answered, «Date | Note», the rubrics of the order, each note's date,
  title and under it the excerpt and tags as Columns shows them and where
  it is — one notebook its name, several «in 2 notebooks» —, the foot.
- The combined contact lists with the owner's card (note 158) need nothing
  of the client: the fixtures hold the card.

## v0.11.15 — 2026-11-02

Typecase `5c3a40c`, note 156 (Р-66, the second group); server `v0.2.392`.

- More tables keep a main column on a phone (`TableCol.Main`): Last seen
  of the sessions (`/settings/account`), Expires of the published links and
  of the invitations, Deposited of the escrow table.
- An invitation by link shows the server's «by link» muted; the ID of an
  invitation's forms is the action and its address, without the row's
  address (an ID holds no space).
- The button before «⋯» — Copy link of an invitation and of a link, a
  file's Download — already stands last in its row, as the cards need; the
  CSS of the note (a field in a column line, the selection bar on two
  lines) is Typecase's alone.

## v0.11.14 — 2026-11-01

Typecase `5b0f920`, note 155 (Р-66, the first group); the server is the
same, `v0.2.391`.

- On a phone a table becomes cards that keep their main column
  (`TableCol.Main`): Role of the users (`/admin/`), Changed of a folder,
  a public folder and the files of a letter's Attach, Changed on the server
  of a connection's collections, What it can see and do of davloom's
  devices. A feed's table keeps none, as its frames.

## v0.11.13 — 2026-11-01

Server `v0.2.391`, Typecase `f62d129`, notes 149–154. The whole client
suite passes, its names check included.

- A note's Attach — in the edit bar and in the column — picks files that
  go with Save: the document sends multipart (`Doc.Multipart`), the picked
  file joins the column's attachments, the separate upload is gone. From
  Files of a new note sends the document it stands outside of
  (`MenuItem.To` with `Form`). U-5.5 #36 and #43 are removed.
- The Upload sheet's file field stands in the sheet's own form and names
  none, so the sheet sends multipart for it.
- A person's timeline has «Filter by word» first in its bar; the range
  carries the kind and the word.
- The tree of a move or a copy marks the folder the items are in, «the
  items are here», whichever node is chosen.

## v0.11.12 — 2026-10-29

- The contact photo controls keep the frame's line wrapper with one
  button too. The contact sample now carries its street, city, postcode
  and country in separate ADR components, matching the edit frames.
- Screens provide the upload size field name to the binding layer.

## v0.11.11 — 2026-10-29

- Contact photos and event/task attachments wait for Save, with a local
  photo preview and pending attachment rows. They work without scripts
  through the same multipart form; the separate Upload is removed.
- Contact frame exception U-5.5 #21 is removed. Notes still use their
  separate upload until Typecase 249 supports multipart Doc forms whose
  file fields stand outside the document; U-5.5 #43 remains for notes.

## v0.11.10 — 2026-10-29

- A full-width note uses the space freed by hiding its side column.
- The note under its publication sheet carries the same working controls.

## v0.11.9 — 2026-10-29

- Notes show literal Markdown with Source and return with Rendered. Width
  and side-column controls work as links without scripts; U-5.5 #41 is removed.

## v0.11.8 — 2026-10-29

Server `v0.2.385`, Typecase `f89d10d`, notes 141–148. Frame comparisons
and draft kinship pass. The full client suite still fails its names check:
Typecase task 247 records the three library issues; this step is not tagged.

### Changed

- Settings and administration follow the 580 px column in every state,
  including dialogs and side panels. Hints and messages inside cards and
  sheets keep their own width. The archive password's warning is a hint.
- Every letter's Send, including the link letter, says «Sending…» while
  sending. Import preview dates use the frame's ordinary type.
- Draft kinship generates all drafts in one `frame2go -self -out` call.
- The rewritten person's card and the task's loss report match their
  updated frames; U-5.5 rows 20 and 35 are closed, row 21 awaits only 43.

## v0.11.7 — 2026-10-28

Server `v0.2.381`, Typecase `2c6b5c8`. A person's card and timeline match
their frames s-contacts-6 and -7; the reports of a loss have their line of
Save.

### Changed

- **The edit of a contact and of an event, on the report of a loss**: the
  line of Save stands under the head, as on every edit — states-12, -17
  and -36 draw it now.
- **A person's card and timeline** no longer wait on `app-contact/normal`
  and `app-contact-timeline/normal`: the timeline has its Refresh, and only
  the fields another copy holds differently are «linked».
- **The task's report of a loss** waits for its frame (Typecase's task 244),
  not for the server.

## v0.11.6 — 2026-10-28

Server `v0.2.379`, Typecase `2c6b5c8`. The full form of a new note matches
its frame s-notes-30, the sign-in its frames s-auth-1 and -13: U-5.5 rows
42 and 17 closed.

### Changed

- **A new note's page**: Notebook in its bar and the caret in its title, as
  the server sends them now; the screen no longer waits on
  `app-note-edit/new`.
- **The sign-in** no longer waits on `login/normal` and `login/refused`:
  the fixtures come from an instance with registration open, as the frames
  draw it («Create an account»).

## v0.11.5 — 2026-10-27

Server `v0.2.376`, Typecase `2c6b5c8`. The list of one collection of tasks
matches its frame s-tasks-15: U-5.5 row 32 closed.

### Changed

- **A task just marked done in the list of one collection**: crossed under
  Open with its tick full (Р-42), as the server holds it now; the screen
  no longer waits on `app-tasks/normal`.

## v0.11.4 — 2026-10-27

Server `v0.2.374`, Typecase `2c6b5c8`. Typecase's notes 137–140: **a
letter's preview**, the sheets' keys (Р-81), the question before a
letter's text is lost, what Send ends with.

### Added

- **Preview** (s-mail-14): one card — From, Reply-To, To, Subject in their
  slots, a name and the address in muted mono; the text with the
  signature; the files as tags with their icons; the links a line each,
  the address in the accent. Send with Enter, Edit, Cancel with Esc.
- **«Close the letter?»** (s-mail-15): «Close and lose the text» without
  a key, «Keep writing» with Esc pushed.
- **The letter that went to some** (s-mail-18): the alarm, a line a
  recipient — the book's mark, the name, the address muted, «sent» strong
  or the relay's words in alarm —, «Send again to the refused» with Enter,
  Close with Esc.
- The relay's refusal on the letter's sheet (s-mail-17): the first
  sentence under To in alarm (`HintRuns`).

### Changed

- **The sheets' keys** (Р-81, note 138): the primary button has its key —
  Enter, Ctrl+Enter on a sheet with a text (a letter's Send, a quick
  note's Save); a button that deletes has none. Cancel stands at the end
  of the foot with Esc, in a pushed line: the foot has no `Push` any more.
- **The letter's Cancel** sends its form (`op=close`); the head's cross
  goes to `MailSheet.Back`. No Discard.
- `zone` reads a route's sheets as the list the row of C-5 opens with
  (`/app/mail/send` answers five).

### Not yet

- **«Sending…»** on Send while the letter goes (note 140): the library's
  button has no such state — Typecase task 242.

## v0.11.3 — 2026-10-27

Server `v0.2.373`, Typecase `fb5a005`. Tests only; nothing drawn changes.

### Added

- **The files' bind has tests of its own** (`bind/files_test.go`): the
  rail and its shortcuts and foot (s-files-1, -6), the tree a sheet
  chooses in (s-files-4, -16, s-notes-11), the upload queue (s-files-2),
  the crumbs, a row's menu, a switch option's icon, the root's note.
  Written with the files section on a branch that never reached main
  (`wip/client-files`); the section came in rebuilt in `v0.5.0` without
  them. Two expectations follow what main draws since: the queue has no
  Hide button, and the tree indents three levels.

## v0.11.2 — 2026-10-26

Server `v0.2.373`, Typecase `fb5a005`. Typecase's note 135: **a restore
asks about what is already here**, a folder's own delete sheet in Files,
the words of the passing screens.

### Added

- **The restore's question** (s-backup-8, -20): `RestoreMatch.Existing`
  as an own-line field over the bar — its label, Skip them · Replace them
  · Import as copies a segment of links, what the one on does under it.
- **How many are already here** in «What will happen» (s-backup-8): «· 2
  already here» after the row's words, plain — after an alarm only the
  words are alarmed (`RestoreRow.Here`).
- **«What is pointing at it now»** on the delete sheet of Files
  (s-files-27): `FileOpSheet.References` after the alarm; the guard knows
  s-files-27.

### Changed

- The restore's note under the table keeps the frame's spacing
  (`MsgFlushSides`); the folder panel's links note is the frame's words.

### Removed

- The report's note under its table (`RestoreReport.Note` is gone).

### Not yet

- The drop sheets of Typecase's note 136 (s-io-1, -20 … -23, and a
  folder's of note 60): the client draws no drop sheet yet.

## v0.11.1 — 2026-10-26

Server `v0.2.372`, Typecase `d7ba208`. **The keys' sheet** (U-7, account-6;
Typecase's note 134).

### Added

- **Keyboard shortcuts** in the account's menu after the theme
  (account-5): a link to `#keys`, pressed by «?», shown only with a
  script (`TopWords.Keys`).
- **The keys' sheet**, closed, on every page of the shell whose menu opens
  it (`document.Keys`, `Shell.Dialogs`): the table of U-7 over the screen,
  shut by its cross, Close and Esc.

### Changed

- The guard of meaning knows a page's own sheet without an address: its
  cross and a Close that `Closes` are the script's.

## v0.11.0 — 2026-10-26

Server `v0.2.372`, Typecase `ff7397d`. **All the screens** (U-6.16,
U-6.17): every address of C-5 is built (`CARREL_ACCEPTANCE=1` green).
**Tagged:** `client-v0.11.0`.

### Added

- **The last sheets**: the export of a combined view (s-agenda-30,
  `screens/export`), the quick sheet (s-notes-5, -6, -8, -9, -10 and
  s-notes-34 — nothing chosen yet, Save waiting; `screens/quick`), the
  photo's crop (s-contacts-5, `contactedit.Crop`).
- **The live zones** (stage 4; Typecase's notes 124, 127): details and
  sheet answer in place, the sheet's closed place over the shell; the
  poll's bar is the zone poll, its stream's events HTML; the queue of
  uploads is the zone queue, the folder after it out of band; the notices
  are one band, the zone notices (`bind.Band`, `document.withNotices`).
- **Paper for every kind** (U-10; print-1…-7, Typecase's note 132): the
  contacts list without photographs (Р-80), a card whole with what is
  kept as the server sent it, the tasks of the filter with their boxes, a
  note in a reader's width, a folder, one calendar; the tables name their
  columns. `bind.Paper`.
- **The page without a network** (U-11, offline-1, `screens/offline`):
  the document names it to the library's worker (`page.App.Offline`).
- **Keys** (U-7): «n» on New, «/» on the search, «Ctrl+S» on an edit's
  Save, «Ctrl+Enter» on the quick note's; **the theme's switch** in the
  account's menu (account-5). The keyboard sheet waits for Typecase's task
  239.
- The filter's cross (`Filter.Clear`), how late a task is in its panel,
  Related with its day for tasks and events.

### Changed

- Kinship reads the notices' band through, as Typecase's shape does
  (notes 127 p. 3, 133 p. 1); the quick sheet's «where» with nothing chosen
  is required (`MenuItem.Required`).

## v0.10.0 — 2026-10-25

Server `v0.2.371`, Typecase `5189dfa`. **The public pages** (U-6.15).
**Tagged:** `client-v0.10.0`.

### Added

- **The guest's pages** `/p/{token}` (s-publish-6…-39, `screens/guest`):
  the strip with the mark and what may be downloaded, the address's strip,
  the head, days, a book by letters, tasks, notes by month, one record, a
  text, a picture, a PDF, a folder as a table, the notices and the foot.
- **The owner's calendar filter**: the found by day, the past muted.

## v0.9.0 — 2026-10-24

Server `v0.2.370`, Typecase `34d1c1c`. **Duplicates** (U-6.7).
**Tagged:** `client-v0.9.0`.

### Added

- **The screens of duplicates and the merge** (s-dup-1, -2, -4, -5, -6,
  -8): cards that are forms with their questions, rows of their own kind,
  «Decided earlier», the threshold and the weights.
- **The linked record** of every kind (s-contacts-33, s-agenda-25,
  s-tasks-17, s-notes-32) and the panels of linked rows (Р-78): where each
  value came from, «Where … is kept».

## v0.8.0 — 2026-10-23

Server `v0.2.369`, Typecase `54efd28`. **The search** (U-6.6).
**Tagged:** `client-v0.8.0`.

### Added

- **The search's screen** (s-find-1, -4…-10) and the suggestions of its
  line (s-find-2, -3): an answer that is a menu, without a document.

## v0.7.3 — 2026-10-23

Server `v0.2.368`, Typecase `ce9da05`. Stage 5: without scripts (U-8).

### Fixed

- **Importing a file without scripts** answered 403: the step's dialog is
  the form that sends the file, but it had neither the action's method nor
  the session's token (C-11). Both are now the action's (`import.choose`).
  Found by the server's new walk without scripts.

## v0.7.2 — 2026-10-23

Server `v0.2.367`, Typecase `ce9da05`.

### Changed

- The guard holds the download's tick of the publish sheet off in
  s-publish-2, -4 and -19 (`ownerOverFrames`): the owner decided after the
  frames were drawn that it opens off for every kind; Typecase's task 229
  redraws them, and the rule goes.

## v0.7.1 — 2026-10-23

Server `v0.2.366`, Typecase `ce9da05`. Stage 5, the first pieces.

### Added

- **The agenda on paper** (U-10; print-1): `?print=1` draws the page on
  paper — the week, what answered, each day's events with the time, the
  title, where and in which calendar, the foot — and prints no forms. The
  other kinds wait for their frames (Typecase's task 228).
- **Installable** (U-11): every page links the manifest the server serves
  and names its colour (`theme-color`). The worker that keeps the shell for
  a lost network waits for the frame of its page (task 228).

## v0.7.0 — 2026-10-23

Server `v0.2.365`, Typecase `ce9da05`. Import and collections built whole:
the import wizard (s-io-3 … -19) and the sheets of collections (s-profile-5
… -35) hold against their frames. The milestones are renumbered (the
owner's decision of 23.10.2026): import and collections are `v0.7.0`,
search and duplicates — waiting for Typecase's tasks 227 and 190 —
`v0.8.0`. Nothing else changed since `v0.6.9`.

**Tagged:** `client-v0.7.0`.

## v0.6.9 — 2026-10-23

Server `v0.2.365`, Typecase `ce9da05`.

### Added

- **The search's screen** (`screens/search`, SearchPage; s-find-1, -4 …
  -10; Typecase's note 114): the query and what was found, the kinds as
  segments; the bar of the pass in every state — Stop while it runs (its
  form under it), the red «N failed» with its menu, how long it took; the
  kinds in their order with their counts, each record in the look of its
  kind (a contact's face, an event's day and «15:00 · Zoom», a task's sign
  and due, a note's excerpt, a file's mark, size, time and access), the
  match in bold; the words of an empty result; the page without a query —
  «What to look for», Search, the recent searches. Built and held against
  its frames; it is registered once Typecase's task 227 p. 1 says whether
  a stopped pass has its segments (s-find-10 draws none).
- `Screen.Poll`: a screen whose bar of the poll is not the lists' draws it
  itself, and the zone `poll` is drawn so too.

### Changed

- A rail section's count apart (`RailSection.Tally`, «Sources 14 / 14»)
  stands in its rubric, monospaced.
- A query standing in the top's field has the cross that empties it.
- The meaning guard knows a file's row found by the search (`SizeRow`: a
  link); its own rule's example of a widget it lacks is `CardPick`.

## v0.6.8 — 2026-10-23

Server `v0.2.364`, Typecase `ce9da05` (notes 114–116; the mockup copied).
What milestone `v0.8.0` names — the import wizard and the collection
sheets — is built; the tag waits for the owner's order of milestones
(`v0.7.0`, search and duplicates, is not).

### Added

- **The import wizard is registered** (`screens/importer`, ImportPage;
  s-io-3 … -9, -14 … -19): every import address of C-5 is built.

### Changed

- **«Into» opens its menu** (note 115; s-io-10 … -13): as «Saved in» of an
  edit — each collection a link to the same step with it chosen, the
  chosen one on, one that cannot be written to off with its reason and
  «ro», under the rule the new collection of the kind.
- The field another is derived from (the name of a new collection,
  s-profile-5) has no id of its own: the library finds it by its name in
  the form (note 116).
- The guard holds the notice over the cards of Connections (s-profile-31,
  -32) with its margin: the frames now draw the page's gutter (note 116).

## v0.6.7 — 2026-10-23

Server `v0.2.363`, Typecase `3779ac0`. Stage 4, step 1 seen in a browser.

### Fixed

- **A sheet and a panel go the plain way again.** Seen in a browser on the
  fixtures: a sheet's answer is its scrim and the forms its buttons submit
  after it — not one element, so the layer put it inside the empty sheet
  zone (a scrim in a scrim), and that zone stood in `main`, the scrim over
  the page only, not over the rail and the top as `s-contacts-2` draws it.
  A panel's answer is the same. `bind.Zones` is `main` and `rail` until
  Typecase's task 226; the empty sheet zone is gone, the empty panel stays
  (its place is right).

## v0.6.6 — 2026-10-23

Server `v0.2.363`, Typecase `3779ac0`. Stage 4, step 1.

### Added

- **The live layer is loaded** (stage 4; Typecase's tasks 84, 85): every
  page loads `tc-live.js` after `tc.js` and names, as data, the headers of
  a request for a part of the page (C-7.1) and the field and header of the
  token (C-11) — the server's names.
- **The zones of C-7.2 are named** (`document.Zoned`): the top bar
  `topbar`, the rail `rail`, the page `main`, the panel `details`, a sheet
  `sheet`; a panel that is not open and a sheet that is not are empty
  hidden zones, where an answer goes. `main` owns the address on a page of
  the shell; a page without it names none.

### Changed

- The answer for `main` is the one `Main` with the page's forms inside, so
  that the layer puts it in the zone's place; for a closed panel it is
  empty (the layer hides the zone); for a sheet, its scrim alone
  (`renderer.SheetZone`), without the screen it stands over.
- A button or a form names its target only when the pages carry that zone
  (`bind.Live`): `poll` and `queue` go the plain way until their step.
- The server's `cmd/carrel` test counts four files of the library on a
  page (the live layer is the fourth).

## v0.6.5 — 2026-10-23

Server `v0.2.363`, Typecase `3779ac0`.

### Added

- **The import wizard** (`screens/importer`, ImportPage; s-io-3 … -9, -14 …
  -19): the source with the file and the folders of notes, the preview with
  the question about records already here, the counts and the table, the
  report's cards. Built and held against its frames; it is registered once
  Typecase's task 225 draws «Into» as a menu and s-io-6 its row with an
  error.

### Changed

- The meaning guard reads a picked file's name (`Input.Picked`) as read
  only, which it is.

## v0.6.4 — 2026-10-23

Server `v0.2.360`, Typecase `0f51d45`.

### Added

- **The collections' sheets** (`/app/collections/{new,rename,move,delete}`;
  U-6.10): New collection — by kind and without one, nowhere to create, the
  server's refusal (s-profile-5, -6, -20, -21, -28, -29); Rename and colour —
  a folder, a feed, the refusal in the sheet (-7, -18, -30, -33); Move or
  Copy — a folder, the whole storage (-8, -34, -35); Delete on the server —
  what points at it, the archive, Download ‹.ics› first or Open in Files
  (-9, -19).

### Changed

- An input whose sample follows another field carries it (`From`: the
  address after the name).

## v0.6.3 — 2026-10-22

Server `v0.2.359`, Typecase `0f51d45` (note 112).

### Changed

- **Connections** holds the outcomes of the collections' sheets on its
  page: the delete the server refused (s-profile-31) and the move that kept
  the original (s-profile-32). The guard takes a fragment drawn without a
  main column by what its root holds, and a notice drawn alone without its
  margin (Typecase task 224).

## v0.6.2 — 2026-10-22

Server `v0.2.358`, Typecase `a78141d`.

### Added

- **Deleting a record** (`/app/{kind}/{account}/{col}/{uid}/delete`,
  RecordDeleteSheet; s-agenda-26, s-tasks-18): what is lost, what points
  at it and its attachments under their rubrics, the tick for a link's
  file, the copies with their marks and ticks; 560 wide with copies, else
  480 (Typecase task 223 p. 4).
- **Rewriting an address** (`/app/contacts/{account}/{col}/{uid}/rewrite`,
  RewriteSheet; s-contacts-15) over the card's edit page: a choice per old
  address, a tick per group with its records behind a counting button,
  what keeps the old address, the button that counts as the form changes.

## v0.6.1 — 2026-10-22

Server `v0.2.357`, Typecase `a78141d` (note 111).

### Changed

- **A contact's head** names its address book with the book's mark, in the
  list's panel and on the page (s-contacts-1, -6, -16, s-publish-3) — as
  the agenda and the notes already do.

## v0.6.0 — 2026-10-22

Server `v0.2.356`, Typecase `bc0b574`. Publishing and mail built whole:
every screen holds against its frames.

**Tagged:** `client-v0.6.0`.

### Added

- **Publish** (`/app/publish`, PublishSheet): a calendar — how much of it
  and its days, what the visitor sees, the download, how long the link
  lives —, a contact, an event, a note with its attachments, a folder; the
  link made with Copy, Send by letter, Open the link, All published links
  (s-publish-2, -3, -4, -5, -13, -19, -22).
- **The letter** (`/app/mail/compose`, MailSheet): one form — the
  recipients as tags with «Name or address» and its Enter, the subject, the
  text, what is attached (Attach's menu, a part's format and size, as a
  file or as a link, its terms and Change…), why some go as links; Send,
  Preview, how it is signed, Cancel (s-mail-1).
- **«Send this link»** (`/app/mail/link`, MailLinkSheet; s-mail-4).
- **The attach sheets** (`/app/mail/attach`, MailAttachSheet): a note, a
  contact (searched under «All …», by a form of its own), a calendar (its
  form recounts the line at Attach), a file, a folder (the tree of the
  storage and the crumbs); Attach off with why until something is chosen
  (s-mail-5, -6, -10, -12, -13).
- The pages under a sheet: Published links and a folder.

### Changed

- The guard of meaning excuses a control of the page under a sheet by
  that page's screen's waits; it knows a tree's heading row (a server), a
  filter whose line is a field of a form elsewhere (`Query.Of`), and
  `BriefRow`.

## v0.5.1 — 2026-10-22

Server `v0.2.353`, Typecase `bc0b574` (its notes 108–110).

### Changed

- **A folder's upload**: Files and Folder pick into one upload form, with
  the field of the sizes and `?op=taken`; one Upload sends it without a
  script (s-files-8). The queue the live layer fills waits hidden
  (s-files-1, states-32), and the server's queue carries the templates of
  a file's row and of a name taken (s-files-2).
- **Published links**: Revoke sends the list's one form with the row's
  `id`, in the row and in its «⋯».
- **Restore**: the archive's field asks which parts to send
  (`FileField.Ranges`), step 2 of an archive opened in parts sends the
  parts of the collections chosen (`Dialog.Ranges`, `RangesOf`), and the
  page's column is the zone its steps answer into (s-backup-15, -16, -20).
  All of it waits for the live layer (stage 4).
- The guard of meaning knows the live queue's templates and its Hide.

## v0.5.0 — 2026-10-22

Server `v0.2.352`, Typecase `8e4944b` (its notes 87–107). Three sections
built whole — the files, the settings, the administration — and the list of
published links; every screen holds against its frames.

**Tagged:** `client-v0.5.0`.

### Added

- **Files** (`/app/files`, `/app/files/{account}/{col}/{path…}`): the root
  with the storages, their access and the shortcuts; a folder — the
  selection form on the page and its bar, tiles and the table, crumbs, the
  empty folder, the queue of transfers folded and unfolded, a name taken
  answered by one form (Keep both and Replace bring the file again, Skip);
  Upload of files and of a folder.
- **The sheets of files**: an operation — the tree as the sheet's choice,
  the server's menu in its head, a new folder in its row; the picker — files
  for an upload or an attachment, a folder to choose.
- **Published links** (`/app/files/published`): a row a link with Copy, Send
  and Revoke, none out, no mail; a link whose thing is gone — a row switched
  off, «no longer there», Copy and Send off with their reason (s-publish-21).
- **Settings**: Appearance; Account — sessions, the password's change, the
  forced one without the rail, key escrow and its history; Connections —
  servers and feeds, the dialogs of a connection by their addresses, the
  collection list updated, Disconnect; Devices — a new device step by step,
  its addresses, the profile; davloom — the address, devices, scope and
  tree; Backups — jobs, a new job, the external trigger, restore from a job,
  a server or this computer, the match and the report; Attachments — the
  folder unset and unavailable; the password's sheet wherever a secret is
  given.
- **Administration** (`/admin/…`): Users, Add users, Settings, the
  installation check, the DAV validator, key escrow, the sheets of a reset
  and of a deletion over Users, the history of a password by its link.

### Changed

- **A field's hint in pieces** (`FormRow.HintRuns`) keeps its monospaced
  piece; a switch's option turns fields off while chosen
  (`Option.Disables` → `PickOption`, `SegOption`).
- **The guards** hold a fragment's form among the forms a screen prints
  after its root, and the trees, tables and sheets of the new sections by
  their meaning.

## v0.4.61 — 2026-10-19

Server `v0.2.351`, Typecase `324ee14` (its note 86). The mockup copied byte
for byte; no screen of the client changed (the password's sheet is still
the server's fixtures).

## v0.4.60 — 2026-10-19

Server `v0.2.350`, Typecase `631f76a` (its note 85). The mockup copied byte
for byte; no screen of the client changed (the settings' screens are still
the server's fixtures).

## v0.4.59 — 2026-10-19

Server `v0.2.349`, Typecase `8f2dbd6` (its note 84). The mockup copied byte
for byte; no screen of the client changed (the administration's screens
are still the server's fixtures).

## v0.4.58 — 2026-10-19

Server `v0.2.347`, Typecase `b804598` (its notes 81–83). The mockup copied
byte for byte.

### Changed

- **The account deletion sheet** is a sheet of its own (`AccountDeleteSheet`
  in the zone of sheets).

## v0.4.57 — 2026-10-18

Server `v0.2.344`, Typecase `844ab51`.

### Changed

- **A tick whose label comes in pieces** (`FormRow.LabelRuns`) keeps its
  monospaced piece: «Allow `.ics` download and subscription».

## v0.4.56 — 2026-10-18

Server `v0.2.342`, Typecase `844ab51` (its notes 79, 80). The mockup
copied byte for byte.

### Changed

- **A note is read at its measure** unless the browser chose the full
  width («Open notes in full width»); the width switch of the bar says
  which is in force, reading and editing alike.

## v0.4.55 — 2026-10-18

Server `v0.2.337`, Typecase `69f0f71` (its note 78). The mockup copied
byte for byte; no screen of the client changed (the admin screens are the
server's fixtures yet).

## v0.4.54 — 2026-10-17

Server `v0.2.336`, Typecase `8e09917` (its note 77). The mockup copied
byte for byte. The conflict screen draws its states (`s-conflict-6`…`-11`):
a second line under the title, the notices as the dialog's messages with
their technical block apart, a notebook's mark in a message, «Your form»,
a deleted record's one card; a card with nothing chosen is required, and
the button waiting for it is pale (`ButtonWaits`).

## v0.4.53 — 2026-10-17

Server `v0.2.335`, Typecase `96b9973` (its notes 72–76); needs
`golang.org/x/text`. The mockup copied byte for byte. A collection's mark
stands by its name; the edits choose where they save in the head (a
note's notebook in its bar), each radio naming the edit's form
(`MenuItem.Of`); the theme the server chose is the page's
(`page.Document.Theme`); the address of a new collection follows its
name as it is typed (`SlugOf`).

## v0.4.52 — 2026-10-16

Server `v0.2.311`. A contact's edit draws its «Dates» as the frame does
(`s-contacts-34`): each row's words as wide as Organisation, its day as
the birthday; the field «Anniversary» of the head is gone.

## v0.4.51 — 2026-10-16

Server `v0.2.309`, Typecase `49412c3` (its note 71). The mockup copied
byte for byte. The task's edit draws what waited for its frames: the List
beside Summary with the lists' marks, the time zone of When as an event's,
Attendees after Reminders, Repeats after the attachments; the event's edit
its Calendar after Tags; a note's Link as wide as its notebook. The
notebook's list has no column «Attendees».

## v0.4.50 — 2026-10-16

Server `v0.2.304`. A note's status and an event's priority are gone from
the screens (note 69 of Typecase, tasks 176–178): the reading column names
no status, the note's edit carries none hidden, the event's edit draws no
Priority, the agenda and the notes have no such columns.

## v0.4.49 — 2026-10-16

Typecase `ad18c91` (its note 70). The mockup copied byte for byte; the
client is green on it without a change: the screens the note redraws —
the publish sheet, the letter, the connection's dialog — are not the
client's yet, and a radio check keeps its markup (only its look is a
circle now). The dates' fields take the width of their value in the
library itself.

## v0.4.48 — 2026-10-16

Server `v0.2.300`, Typecase `22f0506` (its note 69). The mockup copied
byte for byte. A card's Print is a button of the third rank — on a phone
it goes under «⋯», where its twin stands first (`bind.Twins`); the note's
width switch is of the third rank and Source its icon with the word as a
hint; Attach in a rubric keeps its paperclip; a task list has «Filter by
word» first in its bar, as the notes'; the notes' orders are of the second
rank. `bind.Button` sets the button's title from `Action.Hint`.

## v0.4.47 — 2026-10-16

Server `v0.2.296`. A message keeps the faces of its body when the server
says it in pieces (`Notice.Runs`): the addresses of the Email message are
monospaced (`account-9`, `-10`). `bind.Msg` draws the pieces after the
title, as it draws `Body`.

## v0.4.46 — 2026-10-16

Server `v0.2.278`. A conflict card is on only at the side chosen
beforehand: where the server chose nothing (Carrel cannot tell whose the
change is), neither radio is on — before, «Your edit» was. «Write this
combination» comes off with its reason, as the server sends it; how it
comes on without scripts once every side is chosen is Typecase's task 198.

## v0.4.45 — 2026-10-15

Server `v0.2.273`. **The conflict screen** (`ConflictPage`, frames
`s-conflict-1`, `-3`, `-4`, `-5`; U-6.8): «Conflict» and what happened
under it, the page's notices, a line for each property that agrees, then a
card a property — its name as the record has it, monospaced (`TEL;work`,
`X-EVOLUTION-FILE-AS`), how it differs, and its two versions as radios of
the card's row: «Your edit» and «Server», «Your edit · include» against
«Leave it out» where only the edit has it, «Server · include» where only
the server; nothing is a dash. The foot: Write this combination (the cards'
form), Use only my edit and Use only the server version (forms of their
own), Cancel. The meaning guard knows a comparison with `Action` as a form
(`Diff`, as `Form` and `Sheet`). One screen for every kind.

## v0.4.44 — 2026-10-15

Server `v0.2.263`. The sheet of copies says why it came back — Mark done
with no copy ticked — as an alarm over the copies (`Form.Errors`; no frame
draws it yet, Typecase's task 197).

## v0.4.43 — 2026-10-15

Server `v0.2.258`. **The question before a series is saved** (`AskSheet`,
fragments `s-agenda-22`, `-23`, `-24`): over the edit as typed, or over the
series' card once it is written in part. Each block a rubric and its lines,
the alarm, the line about the answers; the answers in their order — the
first the primary (with Enter when there are two), the last pushed right
with Esc — each sending the form to its own address or leading where it
says. With it the server asks again before such a save (`askOnPages`
gone).

## v0.4.42 — 2026-10-15

Server `v0.2.256`. **The sheet «which copy»** of a linked record
(`CopiesSheet`): «Which copy to edit?» (`s-contacts-32`) and «Which copy to
mark done?» (`s-tasks-16`), over the screen it was opened from, as the move
sheet. A line a copy: the mark of its collection, a radio (Edit) or a box
(Mark done) with its source, and how it differs, muted; a copy that cannot
be chosen — its source muted and why. The sheet is a form of its method
(GET for Edit); a button that sends the form it stands in goes by GET too
(`bind.Button`). No frame draws the sheet at its address yet (Typecase's
task 183): the guards hold it by the two fragments. With it the server's
pages lead to the sheet again: Edit of a linked record and the tick of its
row in a combined list.

## v0.4.41 — 2026-10-15

Server `v0.2.255`. **The first sheet: Move and Copy of a record**
(`MoveSheet`, frame `s-contacts-2`). Opened by its address, the sheet is
drawn over the screen it came from (`MoveSheet.Under`, the owner's decision
of 15.10.2026) — that page by its own screen under the scrim
(`bind.Over`, `screens.over`) —, else alone (`bind.Alone`). The sheet is a
form: the collections a tree of radios under `to`, each with the address of
the sheet with it chosen; the one it is in says «already there», a
read-only one its badge; what the move does under them; Move and Cancel.
The guards hold a sheet by its part of the frame and its draft's Sheet
(`guard/sheets_test.go`, brought from the files section's branch and
widened to a sheet over a screen); the meaning guard knows `Sheet` and a
tree's radio node.

## v0.4.40 — 2026-10-15

Server `v0.2.254`. The combined lists of contacts, the agenda, tasks and
notes draw no source column when the browser chose not to see it
(`Shell.Look.Source`, Appearance): their rows as the list of one collection
draws them (`bind.Sourceless`). The theme and the width of a note wait for
the library (Typecase's task 186).

## v0.4.39 — 2026-10-15

Server `v0.2.247`. `CopiesSheet` — «Which copy to edit?» and «Which copy
to mark done?» — is a sheet to the zones (`zone.KindOf`); no screen draws it
yet, so it answers JSON, as Move does.

## v0.4.38 — 2026-10-15

Server `v0.2.246`. A chosen Attendees is drawn in the tasks' and the notes'
rows as one plain badge, as the agenda's cell draws an event's people.

## v0.4.37 — 2026-10-15

Server `v0.2.245`. A task's edit carries its Time zone hidden, as it came:
no frame draws it yet (Typecase's task 180), and the times are shown and
read in it.

## v0.4.36 — 2026-10-15

Server `v0.2.244`. A task's card and panel and a note's column and panel
name their people, the organiser first, drawn as Related draws its records
(`bind.Person`); no frame draws them yet (Typecase's task 179). Their edits
do not draw the field of attendees, and a save leaves the people as they
were.

## v0.4.35 — 2026-10-15

Server `v0.2.243`. A chosen Status and Priority are drawn in the agenda's
rows, and a chosen Status in the notes', with the cells of the tasks'.

## v0.4.34 — 2026-10-15

Server `v0.2.242`. An event's edit draws Priority after Status, as wide.

## v0.4.33 — 2026-10-15

Server `v0.2.241`. A note's column names its status under «Status» after
Tags. Its edit draws no Status yet — no frame does (Typecase's task 176) —
and sends the value back as it came, so a save keeps it.

## v0.4.32 — 2026-10-15

Server `v0.2.240`. An event's edit offers its Calendar last in the head; a
save into another calendar moves the event. A task's List waits for its
frame (Typecase's task 175): the screen leaves it out, and the task stays
where it is.

## v0.4.31 — 2026-10-15

Server `v0.2.238`. A task's form now holds its Repeats; no frame draws
the rubric yet (Typecase's task 173), so the screen draws none and sends
its values back as they came — a save keeps the task's rule instead of
reading as «Does not repeat».

## v0.4.30 — 2026-10-15

Typecase `95453f6` (note 68), server `v0.2.237`. The mockup is Typecase's,
byte for byte (446 frames). The open card of the agenda, a task and a note
takes a file dropped on it into its Attach (`Side.Drop`, s-io-2); without
a script nothing is dropped and Attach stays. The guard reads every page
it prints for names that point at nothing (`page.Check`): none.

## v0.4.29 — 2026-10-15

Server `v0.2.237`. A note's column draws its link under «Link», as Related
draws its records; the task's and the note's Link field is as wide as Tags
beside it, and a note's empty Link has no dash.

## v0.4.28 — 2026-10-14

Server `v0.2.236`. A contact's form gives the anniversary the birthday's
width and a website an e-mail's.

## v0.4.27 — 2026-10-14

Server `v0.2.235`. A task's row draws the chosen Where in the agenda's
Where cell.

## v0.4.26 — 2026-10-14

Server `v0.2.234`. An event's edit ends with «Source» — the path, when it
was read, the comparison with the server — as a task's edit does; a new
event has none.

## v0.4.25 — 2026-10-14

Server `v0.2.233`. A task's edit draws «Kept as the server sent them» — the
properties Carrel does not know, as they came — as the event's edit does,
after the attachments and before the source.

## v0.4.24 — 2026-10-14

Server `v0.2.232`. Send — the record into a letter, a POST — stands on the
event's and the task's head and over the note's text as on the contact's
card: each such button submits a form of the page of its own («send»), so it
works without a script. New task on the note's bar and the card's head
(server `v0.2.231`) is a link and needed nothing.

## v0.4.23 — 2026-10-14

The agenda in the columns a person chose (Typecase's task 154, notes 55 and
63; `s-agenda-21`): a choice other than the default draws each event as a
row of those columns — the time first, the title, the place (the dash when
there is none), the badges of attendees and repetition (an empty cell when
there are none), the tags, when it changed, the source; the default set
still draws the time row of `s-agenda-1`. One calendar has no source
column. No fixture carries a choice, so the guard does not compare these
rows with their frame.

## v0.4.22 — 2026-10-14

Typecase `f47752a` (Typecase's note 66). The mockup is `5c13336`'s. The
library's script the client serves no longer sends a form twice: a form
other than GET sent by the live layer is not sent again the plain way when
an answer came — a redirect is followed, any other answer is shown whole.

## v0.4.21 — 2026-10-14

Typecase `5c13336` (Typecase's note 65). The copy of the mockup is `5c13336`
byte for byte; no screen changed: the sheet whose button counts on its own
(`s-contacts-29`) is not built.

## v0.4.20 — 2026-10-14

Typecase `8b407a5` (Typecase's note 64; task 161 closed). The guard compares
a screen with its frame as it is: Typecase's shape no longer counts an
element printed `hidden` as drawn, and `guard.Drawn` is gone. A field the
page does not show stays out of kinship, as out of shape. A reminder added
in the form opens its own «Custom…» numbers now: the library numbers its
`data-tc-shows` as it numbers its fields.

## v0.4.19 — 2026-10-14

Typecase `f136966` (Typecase's note 63), server `v0.2.220`. The copy of the
mockup is `f136966` byte for byte. A row of the agenda gives the library
every badge of the row, in the server's order (`TimeRow.Badges`, which took
the place of `Caption`); the library draws those that fit, side by side
(«4 attendees» «repeats weekly», `s-agenda-1`).

## v0.4.18 — 2026-10-14

Typecase `14fa0c2` (Typecase's note 62), server `v0.2.219`. The copy of the
mockup is `14fa0c2` byte for byte (443 frames).

- A loss report is drawn as its frames draw it: what was lost in bold, the
  properties after, a full stop; the days of a series in a message of their
  own (`bind.Loss` gives the messages, `states-36`). The edits of a contact
  and of an event draw no line of Save on it, as their frames (`states-12`,
  `-17`, `-36`); the task's and the note's frames draw one (Typecase's task
  161).
- An event's edit gives «Related to» the frame's width and puts the
  attachments after its set (`s-agenda-3`: Reminders, Attendees, Other,
  Attachments, Repeats).
- `app-event-edit/loss-days` waits with U-5.5 row 26, as the loss of an
  event does: the attendees of the set are not the frame's.

## v0.4.17 — 2026-10-14

Typecase `629c31e` (Typecase's notes 50–61), server `v0.2.218`. The copy of
the mockup is `629c31e` byte for byte (442 frames).

- A dialog with `Action` is a form for the guard of meaning (`s-backup-7`,
  note 54).
- A field the model hides (`FormRow.hidden`) is a hidden field, an option
  that shows fields names them (`Option.shows`): the numbers of a
  reminder's «Custom…» (`s-agenda-19`); the edits of an event and of a task
  give them the frame's widths.
- A note's «Tags» carries «linked» when another copy holds other tags
  (`NoteMeta.tagsBadge`, `s-notes-2`).
- The guard compares a screen with its frame by what it shows: a field
  printed `hidden` is left out of the frame comparison and of kinship
  (`guard.Drawn`) until Typecase's shape does it itself (task 161).
- U-5.5 row 40 is gone: the note's edit is its frame now.

## v0.4.16 — 2026-10-12

Typecase `22f594a` (Typecase's note 49), server `v0.2.214`. The copy of the
mockup is `22f594a` byte for byte (429 frames). No screen and no guard
changed: what the note brings — a tick that belongs to a form it does not
stand in (`Send.Of`), a `PostForm` in `Main`, the selection bar of a page
without scripts (`Bar.NoScript`) — is for the files section, which is not
built.

## v0.4.15 — 2026-10-12

Server `v0.2.213`.

### Changed

- **The event's form draws «Related to»** (`s-agenda-3`, `s-agenda-15`): the
  server sends the field for an event as it does for a task and a note, and
  the screen draws a form's row by what it is. The frames of the event do
  not draw it — U-5.5 row 44 (the owner, 12.10.2026), Typecase's task 160;
  `app-event-new/normal` waits under it.

## v0.4.14 — 2026-10-12

Server `v0.2.208`.

### Changed

- **The photo buttons of a card's edit form send the form** (`s-contacts-3`,
  `s-contacts-4`): the server marks them `Submits` now, as a new card's, and
  the screen draws them buttons of the form with their own address — what
  was typed goes with the photo and comes back. The screen's code did not
  change: it already drew a button by what its action says; the page prints
  no separate forms for them any more.

## v0.4.13 — 2026-10-12

Typecase `f30d992` (Typecase's note 48), server `v0.2.207`. The copy of the
mockup is `f30d992` byte for byte (429 frames). No screen changed.

### Changed

- **The guard of meaning knows a group of fields that is a form**
  (`FieldSet.Action`, Typecase's task 149): what stands in such a group
  stands in a form that sends, as in a card or a field with an address;
  without the address the group is a group. Red before the change: every
  edit screen (`TestScreensHaveMeaning` — «FieldSet carries meaning the
  guard's contract does not know») and the two new cases of
  `TestMeaningRules`.

### Not done

The screens the note changes are not built yet: a folder's tile
(`ThumbFolder`) and a thumbnail's icon under its picture belong to the files
section (`wip/client-files`), the forms of task 149 to the settings and the
sheets.

## v0.4.12 — 2026-10-12

Server `v0.2.195`. No screen changed.

### Changed

- **A combined list whose poll is still running matches its frames**
  (`states-8` — the contacts, `states-13` — the calendar): the words of the
  poll (`v0.2.194`) and the numbers of the rail (`v0.2.195`) are the
  server's now. U-5.5 rows 19 and 24 are gone, and the two fixtures no
  longer wait.

## v0.4.11 — 2026-10-12

Typecase `462ffb5` (Typecase's notes 45 and 46; Carrel's task 152 answered),
server `v0.2.192`. The copy of the mockup is `462ffb5` byte for byte (428
frames).

### Changed

- **The agenda's bar has Columns where the ticks of the kinds stood**
  (`s-agenda-1`, `-17`, `states-15`): the calendar shows events and nothing
  else. The menu is the form of `s-agenda-18`. **The line of the agenda is
  still the frame's own whatever is chosen**: the library has no line of
  chosen columns for an agenda (to-typecase task 154).
- **A relation's pill carries the mark of its record's collection**
  (`bind.Related`, `Tag.Mark`; `s-tasks-3`, `s-notes-20`); a record that is
  not among the loaded ones is its UID, without a mark.
- **The end of an event follows its start** (`bind.Input` hands
  `FormRow.Follows` and `Day` to the library's `Input`): `s-agenda-3`,
  `s-agenda-15`, `states-17`, and the quick sheet `s-notes-9`.
- U-5.5 row 40 (`app-note-edit/normal`) stays, for another reason: both
  relations of the note now match `s-notes-20`; what differs is the second
  attachment's Remove and the block of the same note in other notebooks.

## v0.4.10 — 2026-10-11

Typecase `43ca489` (Typecase's note 44: tasks 144 and 145), server
`v0.2.187`. The library took the tags out of the rows of tasks and notes
(`DueRow.Tags`, `ExcerptRow.Tags` are gone): the client did not build
until it stopped passing them.

### Changed

- **Rows of tasks and notes carry no tags.** They are in the panel of the
  record and in the rail; the column «Tags», when a person turns it on,
  shows the first tag and counts the rest (`ColumnCell.Tags`). The rows the
  screens draw as the frames' own are now Title, Due, Complete, Source and
  Title, Excerpt, Date, Source.
- **The list of one address book has no source column** (`states-4`): the
  book is named in the head.
- The lists of notes match their frames — the combined one, one notebook
  and a notebook that did not answer (`s-notes-1`, `s-notes-31`,
  `states-29`) —, and so do the contacts with a book that did not answer
  (`states-9`) and a note read (`s-notes-2`): U-5.5 rows 37 and 38 are gone,
  19 waits for `states-8` alone (the counts of the sections while a poll
  runs — the server's), 40 for the edit of a note (`s-notes-20`).

## v0.4.9 — 2026-10-10

Typecase `7b8f469` (Typecase's note 43: tasks 146 and 147), server
`v0.2.179`. No draft of a built frame prints otherwise and no screen
changed.

### Changed

- **The edit of a task matches its frames** (`s-tasks-3`, `s-tasks-14`): the
  comparison no longer takes the `datalist` of the line in Related to for
  something drawn. `app-task-edit/normal` and `/new` no longer wait; U-5.5
  row 35 stays for the report of what was lost (`states-24`), rows 40 and
  42 for what else they wait for — the `datalist` is gone from all three.
- The copy of the mockup says «Move» and «Copy» on the buttons of the six
  sheets, as the server has since `v0.2.167`.
- The guard of kinship with the draft takes the choices of a line of input
  (`TagAdd.Options`, a `datalist`) for the meaning of HTML, as it takes a
  hidden field and an option of a select: no frame draws them. Without
  this the two screens that stopped waiting were red against their drafts.

## v0.4.8 — 2026-10-09

Typecase `0fe0244` (Typecase's note 42: the files picked for an upload by
name, the table of published links breaks no word; the owner's decisions of
09.10.2026), server `v0.2.162`. No draft of a built frame prints otherwise
and no screen changed: the frames that did — the sheet of an upload
(`s-notes-11`, `s-files-19`), the list of published links (`s-publish-1`,
`-17`) — are of the files section, not built yet.

## v0.4.7 — 2026-10-09

Server `v0.2.161` (the owner's decision of 09.10.2026: several relations).

### Changed

- **Related to shows every relation and always has its line**
  (`bind.Related`): each record related is a tag its cross unticks, and
  after them stands the line one more link is typed into (`FormRow.Add`,
  `TagInput.Add` — «Title or UID», «Add a link»), with the records loaded
  and not yet related as its choices without a script. The edits of a task
  and of a note draw it.

### Waiting

- U-5.5 rows 35, 40, 42: the field matches its frames, but the frame
  comparison takes the `datalist` of the line's choices for something drawn
  (Typecase's task 146).

## v0.4.6 — 2026-10-09

Typecase `6833ef4` (Typecase's note 41: Carrel's task 119, the owner's
decisions of 09.10.2026), server `v0.2.159`. No draft of a built frame
prints otherwise and no screen changed: the frames that did — the list of
published links (`s-publish-1`, `-17`), the menu Upload (`s-files-8`), the
sheet of an upload (`s-notes-11`) — are of the files section, not built yet.

### Closed

- U-5.5 row 43 no longer waits for the library: a form with a file field
  of its own is multipart (`Form`, `Doc`, `Card`, `Sheet`). It waits for
  the server alone — Save of an edit takes no files (C26).

## v0.4.5 — 2026-10-09

Typecase `3adc445` (Typecase's note 40: the rest of Carrel's tasks 106 and
121, the owner's decision of 09.10.2026), server `v0.2.158`. The two drafts
that print otherwise went in first, unchanged (`fe77252`). No screen
changed: the panel and the card of an event draw the fields the server
sends, in its order.

### Closed

- The second reminder of «Синк-ап по релизу» is in the set (server
  `v0.2.158`), as the frames of the card, the panel and the edit draw it;
  U-5.5 row 26 keeps the organiser and the report of losses.

## v0.4.4 — 2026-10-09

Typecase `0bf5f71` (Typecase's notes 37, 38 and 39 in one step: Carrel's
tasks 105, 121, 127, 136–143 and the owner's decisions of 08–09.10.2026,
Р-42 … Р-48), server `v0.2.156`. The drafts that print otherwise — 38 of
the 55 frames built — went in first, unchanged (`557f35d`); the screens
over them here.

### Changed

- **A row in the list of one collection has no source and no column for
  it** (Р-48): one calendar, one list of tasks, one notebook
  (`TimeRow.Single`, `DueRow.Single`, `ExcerptRow.Single`; `bind.Single` —
  the rail's row that is on is a source). The sources of a linked task or
  note are the hint of its badge «linked ×2» (`bind.LinkedBadge`,
  `Badge.Title`).
- **The panel of a task has its card's order**: the attachments before the
  related records, then the block Source — the path and how the task
  compares with the server (`s-tasks-1`).
- **The name of a property Carrel does not know is monospaced** in the line
  under a note's text in its edit (`NoteDoc.Foreign` in pieces, `s-notes-20`).
- The line under a calendar's title has the zone monospaced and the count
  of the window's events after it; the server sends it, the screen draws
  its pieces as before.

### Closed

- Seven states match their frames and wait no more: nothing ticked
  (`app-contacts/empty`, `app-tasks-all/empty`, `app-notes-all/empty` —
  states-7, -18, -25), an empty book and a book that did not answer
  (`app-contacts-one/empty`, `-/source-error` — states-10, -11), one
  calendar (`app-agenda/normal`, `s-agenda-17`), the card of a task
  (`app-task/normal`, `s-tasks-2`). U-5.5 rows 25 and 33 are closed; rows
  19, 24, 31, 32, 35, 37, 38, 40, 42 say what the new frames leave — the
  set of fixtures against the one set of the frames (Р-47), and the
  server's.

## v0.4.3 — 2026-10-08

Server `v0.2.155` (the rail of one collection carries the section's tags).
No screen changed.

### Closed

- Three states match their frames and wait no more: a read-only book
  (`app-contacts-one/readonly`, `states-4`), one calendar's empty week
  (`app-agenda/empty`, `states-15`), a list of tasks whose server did not
  answer (`app-tasks/source-error`, `states-22`). U-5.5 rows 19, 24, 32, 38
  narrowed to what is Typecase's (its tasks 138–143) and the set's.

## v0.4.2 — 2026-10-08

Typecase `80d1319` (Typecase's notes 35 and 36: Carrel's tasks 86, 97, 104,
108, 114, 116, 118, 120, 124–126, 128, 129, 131–135, the rest of 106, and
the owner's decisions of 02–07.10.2026, Р-34 … Р-41), server `v0.2.154`. The
drafts that print otherwise and the drafts of the new frames that have a
fixture went in first, unchanged (`d7efae9`); the screens over them here.

### Changed

- **A rail's count and «ro» stand apart** (Р-34): a feed has its count
  where a count stands — the dash when it has none — and «ro» as a badge
  beside it, as a read-only collection has; a source that does not answer
  has the sign alone (`bind.TickRow`).
- **The refresh switched off says why** (Р-39): with nothing ticked or
  while the poll runs its words are its own, «Refresh», and the reason its
  hint (`bind.Top`).
- **The poll's bar is red only when the poll ended with a refusal** (Р-40);
  while it runs it is green (`bind.Runbar`, `Meter.Alert`).
- **What is shown and what is missing stands under the rows** of a list
  whose source did not answer (`PollBar.Note`): the calendar (`s-agenda-1`),
  contacts, tasks, notes.
- **In a list read only a task's tick is a sign** (Р-38): it looks as usual,
  is not pressed and sends nothing (`bind.Tick`, `Tick.Sign`).
- **A note's text in its panel has the parts its page has** (task 125): a
  heading, a bulleted list, a quotation, code, a table (`bind.SecText`).
- **A note's edit has the bar of an edit** (`Docbar.Edit`, task 104):
  without a script only Save and Cancel stand; its Attach opens its menu
  (task 126), each item by a form of the bar's own. A new note with nothing
  written says nothing of a draft (Р-41), and has no source in its column;
  its form is `note-new`.
- **A new task** (`s-tasks-14`): its form is `task-new`, «Add reminder»
  stands alone without a blank label, and there is no source.
- **The frames of one collection and of the states** are the screens' own:
  `s-agenda-17`, `s-tasks-15`, `s-notes-31`, `states-18`, `-22`, `-24`,
  `-25`, `-29`, `s-tasks-14`, `s-notes-30`.
- The meaning guard's table: a `Field` and a `Card` with `Action` are forms
  of their own (task 131); in an edit's bar (`Docbar.Edit`) what is neither
  a link nor a submit is the page's script's and needs no meaning.

### Closed

- U-5.5 row 39 (the parts of a text in a panel). Row 24 without
  `app-agenda-all/empty`, which matches `states-14`; row 41 without the
  edit's bar. Rows 19, 24, 25, 32, 35–38, 41, 42 rewritten for the new
  frames: what each still waits for is the server's or Typecase's
  (docs/plan/server.md, «Переход на Typecase `80d1319`»).

### Not yet

- **A note's Source, width and side column** (row 41): the frame draws
  them links the live layer swaps in place; the model carries no address
  for them yet.
- **A new note's notebook and a new record's Related to** (rows 35, 42).

## v0.4.1 — 2026-10-03

Typecase `ac658b0` (Carrel's tasks 112, 113, 115, 117, 122, 123 and the
owner's decisions of 02.10.2026 on edits), server `v0.2.152`. The drafts
that print otherwise went in first, unchanged (`0dce6a6`); the screens over
them here.

### Changed

- **The line of Save** (`Runbar.Progress`) stands under the head of every
  edit — a contact's that exists, an event's, a new event's, a task's, a
  note's —, hidden until the library's live layer moves it; the edits'
  forms take the frames' IDs (`contact-edit`, `event-edit`, `event-new`,
  `task-edit`, `note-edit`).
- **A new record's Attach and a new card's photo send the form they stand
  in** (task 122): From Files and «Upload» to their own address
  (`MenuItem.To`, `Button.To`), the file picked with the script
  (`FileField.To`) — what was typed goes with them (`bind.MenuItems`,
  `bind.Uploads`, `bind.FileButton`, `bind.Upload`).
- **The timeline's «⋯»** opens its menu (task 112): the twin of Columns.
- **A note's attachment in its edit** has its Remove at the right
  (`DocAttach.Remove`, `s-notes-20`).
- «Nothing attached» is the words of an empty list (`Dim.Empty`).
- The meaning guard's table: an item or a file field that sends the form it
  stands in (`To`) — only inside a form that sends.

### Closed

- U-5.5 row 36 for the new task and the new event; row 22 narrowed to the
  twin of Columns; row 19 without the «⋯» of `states-8`, `-9`.

### Not yet

- **A file of an edit with its Save** (row 43, the owner's decision of
  02.10.2026): the frames draw the file and the photo a field of the edit's
  form; the library's form prints no multipart without a script, and the
  server's Save takes no file yet (debt C26). The file goes by its own form,
  as before.
- **A new note's From Files and «Upload»** (row 36): the column stands
  outside the note's form.

## v0.4.0 — 2026-10-03

The notes section: the fourth section built whole. Typecase `d29ecf8`,
server `v0.2.150`. Each screen went in as the draft `frame2go -self`
printed, alone, then the screen over it.

**Tagged:** `client-v0.4.0`.

### Added

- **The notebook** (`/app/notes`, `/app/notes/{account}/{col}`, `s-notes-1`)
  from `NotesPage`: the head with what is open and shown, New note with its
  arrow in the combined view, Import, Export; the bar — the filter by word
  (a GET form), the order (links), Columns with its menu (a form), how many
  notes are loaded; the poll's bar; the months with their counts, a row each
  note — its title leading to the note, its excerpt, tags, date, «2 sources»
  of a linked one with the sources in its hint; with the columns a person
  chose, a row of them; the empty state; the note's panel (`?open=`) — Edit,
  the note's page, «⋯» with Delete note last, the notebook and when, the
  text with its tasks (a tick a sign, not pressed), the tags, the
  attachments with Attach and its arrow, the related records.
- **The note** (`/app/notes/{account}/{col}/{uid}`, `s-notes-2`) from
  `NotePage` read: the head — the notebook's mark, where and when —, Edit,
  Export Markdown, Print, «⋯» (Move, Copy, Publish a link); the text — its
  title, paragraphs, headings, tasks, lists with their nested lists,
  quotations, code, tables (`s-notes-21`, `-25`); the column — tags, related
  records, attachments with Attach, the same note in other notebooks, the
  source, Delete note; the rail of return with the notes beside it.
- **The edit and the full new form** (`…/{uid}/edit`, `…/new`,
  `s-notes-20`) from `NotePage` in its edit: the document is the form
  (`Doc.Action`), which Save of the bar submits without a script; the title,
  Notebook as a choice with the notebooks' marks (a read-only one shown and
  off, «ro»), Date and Time, Tags, Related to as the record chosen or the
  line of a UID with the records loaded as its choices, the text, the line
  of the properties Carrel does not know; the column without the tags and
  related records.
- **`bind` for them**: a note's text as a document's parts (`DocText`,
  `Spans`) and as a panel's (`SecText`), the edit's document (`DocForm`) and
  its Save (`SubmitForm`), a choice of collections with their marks
  (`Marks`), a neighbour of the rail (`TreeRow`). A test for each.
- **The meaning guard's table**: a note's row (`ExcerptRow`) and a
  neighbour (`TreeRow`) lead to the note; a task's tick in a text is a sign
  (`Tick.Sign`); a record and an attachment of a document's column
  (`DocRef`, `DocAttach`) go where the model says; a document in its edit
  is a form (`Doc` with `Action`). Bites in `TestMeaningRules`.
- U-5.5 rows 37–42 and row 36 wider (the owner's decision of 01.10.2026):
  what the library or the frames do not give yet, each with its Typecase
  task or the server's debt (C25 — plan, «Экраны раздела»).

### Changed

- U-5.5 row 31 narrowed: the set carries a task tagged «read» since the
  server `v0.2.149` (C24 closed); what still waits is a task done under
  Open (Typecase's task 105). Row 36 names Typecase's task 122.

### Not yet

- **Held against their frames only in part** (U-5.5 37, 38, 40, 42): the
  combined notebook's frame has three notebooks, a fifth tag, a «linked»
  badge among the tags, a shorter text and two attachments in the panel;
  the note's frames have a third neighbour, and the read one a text without
  the set's second heading; one notebook, a notebook that did not answer,
  nothing ticked and the new note's form have no frames of their own
  (Typecase's tasks 105, 97).
- **The panel's text** is paragraphs and tasks (row 39): its headings,
  lists, quotations, code and tables stand as words among the paragraphs.
- **The client's controls** of the note — Source, the width of the text,
  the side column, the markup's buttons, Auto-save, the draft in this
  window, Focus — do nothing without a script (row 41, task 104); the modes
  Reading width (`s-notes-3`) and Focus (`s-notes-4`) are the client's,
  stage 4.
- **The arrow of a new note's Attach** posts its own form, not the form it
  stands in (row 36).

## v0.3.1 — 2026-10-02

Typecase `d29ecf8` (Carrel's tasks 96, 98–103, 109, 111 and the owner's
acceptance of 02.10.2026), server `v0.2.150`. The drafts that print otherwise
went in first, unchanged (`52a47d9`); the screens over them here.

### Changed

- **A file without a script** (task 109): «From this computer» in every
  Attach menu is the label of its file input, which the browser opens
  without a script, and «Upload» after the menu's items, shown only without
  a script, sends the file picked (`bind.MenuItems`, `bind.Uploads`); the
  photo's Replace and Add likewise, «Upload» beside them
  (`bind.FileButton`, `bind.Upload`; the input takes a picture).
- **Related to without a script** (task 96): the records loaded are the
  choices of its line, each sent by its UID and shown with what it is
  (`bind.Related`, `TagAdd.Options`).
- **All day switches off both times** (task 111): the library's check takes
  the names the server sends.
- **The agenda's Day · Week · Month · Range** fold on a phone's bar only
  (`SegFold.Phone`).
- The meaning guard's table: a button or a menu's item that picks a file is
  judged by its file field — a form of the page and the field's name.

### Closed

- U-5.5 rows 28 (a file without a script), 30 (Related to without a
  script), 34 (All day's two fields).

## v0.3.0 — 2026-10-02

The tasks section: the third section built whole. Typecase `bc575a8`,
server `v0.2.146`. Each screen went in as the draft `frame2go -self`
printed, alone, then the screen over it.

**Tagged:** `client-v0.3.0`.

### Added

- **The list of tasks** (`/app/tasks`, `/app/tasks/{account}/{col}`,
  `s-tasks-1`) from `TasksPage`: the head with what is open and shown, New
  task with its arrow in the combined view; the bar — Open, Done, All with
  their counts and the order (links), Columns with its menu (a form); the
  poll's bar; the groups with their counts, Overdue's in alarm; a row each
  task — its tick posts Mark done without a script, its title leads to the
  task, «2 sources» of a linked one with the sources in its hint; with the
  columns a person chose, a row of them (`s-tasks-13`); the empty state;
  the task's panel (`?open=`) — its fields, description, related records,
  attachments with Attach and its arrow and the line of where a file goes,
  deletion.
- **The task** (`/app/tasks/{account}/{col}/{uid}`, `s-tasks-2`) from
  `TaskPage`: read, not edited — Edit, Mark done with its tick, Print, «⋯»;
  overdue in alarm in the head and after the due date; the related records
  with their kind and day; the same task in other lists; the source.
- **The edit and the full new form** (`…/{uid}/edit`, `…/new`,
  `s-tasks-3`) from `TaskEditPage`: the list's mark and overdue as a badge
  in the head; When with All day, the dates and times; Progress — Status,
  Priority, Percent complete, Done; Reminders with «Add reminder»; Other —
  Location, Tags, Related to as the record chosen (or the line of a UID);
  Attachments with Attach and Remove; the source; the deletion under the
  form.
- **`bind` for them**: the head's line whose alarm is a badge (`SubAlert`),
  a task's tick (`Tick`, `TickID`), the field Related to (`Related`); a
  card's value of one monospaced piece is the line's monospaced face
  (`FieldsItem`). A test for each.
- **The meaning guard's table**: a task's row (`DueRow`, Typecase's task
  95) and a row of chosen columns (`ColumnRow`) lead to the record; bites in
  `TestMeaningRules`.
- U-5.5 rows 30–36 and row 28 wider (the owner's decision of 01.10.2026):
  what the library or the frames do not give yet, each with its Typecase
  task or the server's debt (C24 — plan, «Экраны раздела»).

### Not yet

- **Held against their frames only in part** (U-5.5 31–33, 35): the
  combined list's frame has a tag the set lacks and a task done the set has
  open; the task's frame draws it overdue on a day it is not; the edit's
  frame has All day off for a due date without a time; one list, a list
  that did not answer, nothing ticked, the loss of a save and the new
  task's form have no frames of their own (Typecase's task 97).
- **Without a script** Related to offers no list of the loaded records
  (row 30, task 96); All day's two fields stay as the server sent them,
  not as it is ticked (row 34, task 111) — both done in Typecase
  `d29ecf8`, which the client takes with its next bump; the arrow of a new
  task's Attach posts its own form, not the form it stands in (row 36).
- **The suggestions of Related to** (`s-tasks-10`) come with the live
  layer (stage 4): `FormRow.Suggest` is handed to the field, and without a
  script a UID is typed in its line.

## v0.2.1 — 2026-10-02

Typecase `bc575a8` (Carrel's tasks 78–82, 93–95, 106, 107 and the owner's
decisions of 02.10.2026), server `v0.2.146`. The drafts that print otherwise
went in first, unchanged (`437bd97`); the screens over them here.

### Changed

- **A source that did not answer** is an error mark in the rail, its reason
  the mark's hint (`SourceRow.Detail`); no words by its name and no Retry
  there. The poll bar says «N failed» (`PollBar.Failed`) — a red button with
  the menu of refusals: each source's mark, name, what was asked and
  answered (`SourceProgress.Answer`), its Retry; Retry all under them
  (`RetryAll`). `bind.Refusals`, `bind.PollForms` (the retries' forms at the
  page's end); `bind.RetryForms` of the rail is gone.
- **The bar's «⋯» holds its twins** (task 82): the contacts list — the
  order as items and «Compact rows» as the same GET form; the agenda — the
  ticks «Events · Tasks due · Notes» and «Compact rows» as forms, the range.
- **The range sends the window** (task 107): `Range.Hidden` carries
  `view=range` and the list's other choices.
- **An attachment's name is a link** in the event, its panel and the agenda
  (task 93); **Remove** stands on the edit (`AttachRow.Actions`, red for
  data inside, plain for a link). `bind.Button`: an action that sends the
  form it stands in (`Action.Submits`, no file) submits it to its address.
- **The invitation by link** has its frame (`s-auth-14`), the event's loss
  its frame (`states-17`).
- `zone`: a view model C-5 answers only as a fragment of its own
  (`SuggestList`) is no page, and has no zones; `fixtures.Models` names
  `app-contacts-suggest`.

### Closed

- U-5.5 rows 18 (`s-auth-14`), 23 (`Range.Hidden`), 27 (Attach of the new
  form, server `v0.2.145`), 29 (Remove on the edit); row 22 narrowed to the
  timeline's «⋯» (its frame draws no menu), 19, 21 and 26 to what still
  differs.

## v0.2.0 — 2026-10-02

The calendar section: the second section built whole. Typecase `9e69804`
(Carrel's tasks 71–77: a menu in a rubric and in a line, days as one row,
the time zone as a menu of the form's radio buttons, a bar's ticks of
several checks, the attendees' tags), server `v0.2.142`. Each screen went in
as the draft `frame2go -self` printed, alone, then the screen over it.

**Tagged:** `client-v0.2.0`.

### Added

- **The agenda** (`/app/calendar`, `/app/calendar/{account}/{col}`,
  `s-agenda-1`; the poll running `states-13`, nothing ticked `states-14`,
  one calendar's empty week `states-15`) from `AgendaPage`: the head with
  what is open and shown, New event with its arrow in the combined view; the
  bar — the step back, Today and forward, the windows (links), the range of
  dates (a GET form), «Events · Tasks due · Notes» and «Compact rows» (GET
  forms of ticks), «⋯» with their twins; the poll's bar with its right; the
  days that have events, a row each record; the empty state; the event's
  panel (`?open=`) — its fields, description, attendees, attachments with
  Attach and its arrow, related records, source, deletion.
- **The event** (`/app/calendar/{account}/{col}/{uid}`, `s-agenda-2`) from
  `EventPage`: read, not edited — Edit, New task, Print, «⋯»; the same event
  in other calendars.
- **The edit and the full new form** (`…/{uid}/edit`, `…/new`,
  `s-agenda-3`, `-15`) from `EventEditPage`: When with All day and the time
  zone as a choice with its menu, Reminders with «Add reminder», Attendees
  as tags with the line the next is typed in, Attachments, Repeats with its
  days as one row, what is kept as the server sent it; the deletion and the
  line of a series under the form.
- **`bind` for them**: the head's line in its pieces (`Sub`), a person of an
  event read (`Attendee`) and the field of attendees (`Attendees`), a form's
  tick (`Check`), the days of a series (`Days`), a choice with its menu
  (`PickMenu`); a field's value in its pieces and its badge
  (`FieldsItem`: `Field.Runs`, `.Badge`); the right of the poll's bar
  (`PollBar.Aside`); an action's file (`ActionForm`: `Action.File`). A test
  for each.
- **The meaning guard's table**: a choice that opens its menu (the time
  zone) and a menu's line that finds its items are no findings; bites in
  `TestMeaningRules`.
- U-5.5 rows 23–29 and row 22 wider (the owner's decision of 01.10.2026):
  what the library or the frames do not give yet, each with its Typecase
  task or the server's debt (C22, C23 — plan, «Экраны раздела»).

### Changed

- **The rubric of subscriptions** carries the screen's badge «external» by
  `RailSection.Feeds` (Ж8, ЗЖ4; `TestSources` red before the fix).
- **The row of the combined view, ticked off,** stays as it is: nothing is
  ticked, no source is hidden (`states-14`; `TestTickRow` red before).

### Not yet

- **Held against their frames only in part** (U-5.5 22–29): the state
  frames of the agenda are drawn shorter than the model and `s-agenda-1`;
  one calendar has no frame of its own; the edit's frame has a second
  reminder and an organiser from the books; a new event has no Attach
  (C22); the range of dates sends no `view=range`; a file goes from no
  form without a script; no frame takes an attachment off.
- **The suggestions of attendees** (`s-agenda-12`, `-13`) come with the live
  layer (stage 4): without a script what is typed goes with the form, and
  the server finds the person.

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
