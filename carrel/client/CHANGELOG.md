# Changelog

All notable changes to the Carrel client are documented here. Numbers like
`v0.0.N` name steps in this file only — they are not git tags unless an
entry lists **Tagged:**. The plan is [docs/plan/client.md](../../docs/plan/client.md).

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
