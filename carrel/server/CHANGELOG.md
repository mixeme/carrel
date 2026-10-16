# Changelog

All notable changes to the Carrel server are documented here. Numbers like
`v0.0.N` name releases in this file only — they are not git tags unless an
entry lists **Tagged:**. This file tells consumers what to do, not what the
authors did.

## v0.0.79 — 2026-09-21

Copies the Typecase mockup through plan 1 steps 6–8: slips closed (Р-7–Р-12),
narrow shell is the source (Р-3, Р-4, Р-21), lock lint on, screenshot diffs
only the rows in Typecase `allowed-diffs.md`. Leftover `--rN` stay.

### For consumers

Nothing in the running server. Open
[carrel-ui-mockups.html](../../docs/visual/carrel-ui-mockups.html) for the
current mockup; fonts are in `docs/visual/fonts/`.

## v0.2.314 — 2026-10-16

### Changed

- **The administration's fixtures hold what its frames draw** (Ад4, Ад8,
  Ад11, Ад15, Ад23, Ад36 of the scan of 15.10.2026): beside mix, anna, whose
  key is in escrow, and viktor, disabled (`s-admin-2`, `-6`); an invitation
  by link and one by letter the relay refused, and the registration open
  (`s-admin-1`); no relay and people's letters off (`s-admin-3`). The sheet
  that resets a password has its fixtures (`admin-reset`): anna chosen,
  the password issued, a password too short, a login that is not hers
  (`s-admin-13`, `-14`, `-16`, `-20`).

### Fixed

- The reset sheet answered a refusal with the confirmation field emptied;
  it keeps the login typed, with its reason (`s-admin-20`).

### For consumers

New fixture directory `admin-reset` (states `normal`, `issued`, `refused`,
`mismatch`; C-14.3 has `issued` and `mismatch`). Fixtures of the
administration re-shot.

## v0.2.313 — 2026-10-16

### Changed

- **The journal, the invitations and key escrow by their frames** (Ад14,
  Ад24, Ад25, Ад34, Ад35 of the scan of 15.10.2026; `s-admin-1`, `-6`,
  `-7`): the journal is read from a day — a date in the bar's form,
  `?since=`, kept by the segments and by Export CSV — and a line that says
  something did not happen is marked (`AuditRow.Failure`); an invitation
  whose letter did not go says so in its own field (`InviteRow.SentFailed`);
  a recovery answers with the temporary password as a reset issues it
  (`IssuedPassword`: the message, Copy with its icon, whether the person was
  told by letter, Done), and the page no longer carries an empty `History`;
  a covered row says whether its last recovery is a moment
  (`EscrowCoverRow.Recovered`), so «never» is dimmed.

### For consumers

C-5: `/admin/audit?since=`. C-6: `AuditRow.Failure`, `InviteRow.SentFailed`,
`EscrowCoverRow.Recovered`; `AdminEscrowPage.Result` is an `IssuedPassword`,
`History` and `EscrowRecoveryResult` are gone. Fixtures `admin-audit`,
`admin-escrow` re-shot.

## v0.2.312 — 2026-10-16

### Changed

- **The instance's settings are one form** (Ад17–Ад22, Ад37 and a part of
  Ад38 of the scan of 15.10.2026; `s-admin-3`): «Global settings and mail»
  under the title, Discard and Save in the head; the rubrics Global, Mail
  («not configured» while there is no relay), Letters from users («off»
  while people do not send; the sender of their letters stands here now)
  and Test. Save stores all of it or nothing, every refusal at its row at
  once (`user.SaveSettings`), and the journal has one entry naming the rows
  changed — «inviteTTL, idle, session, name»; a value the default shows is
  kept as the default. The relay's fields have the frame's samples; the
  ranges and the relay's rules are said when a value is refused, not beside
  the fields. Send test is a button of the same form: it sends through the
  relay as typed — saved or not — and stores nothing; off, with «Mail is not
  configured: there is nothing to send through yet.», while there is no
  relay saved or typed.

### For consumers

`AdminSettingsPage` in C-6: `Instance`, `SMTP`, `Mail` and `Limits` give way
to `Subtitle`, `Discard` and one `Form`; `FieldSet.Badges` is new. The
settings are posted whole. Fixtures `admin-settings`, `admin-audit`
re-shot.

## v0.2.311 — 2026-10-16

### Changed

- **A person's dates are a rubric of rows** (Typecase's note 71, task 171;
  `s-contacts-34`, the owner's decision of 16.10.2026): «Dates» after
  «Websites», each row what it marks — free words — and the day, typed as
  the birthday is, with Remove and «Add date»; folded away with the other
  empty fields when the card has none. A row is written as Apple writes it,
  `itemN.X-ABDATE` with `itemN.X-ABLabel` in one group, a new one in the
  card's next group; Apple's own words are its marks (`_$!<Anniversary>!$_`).
  A card's `ANNIVERSARY` is a row marked «Anniversary» and stays that line
  while it is so marked. The card shows each date with its words, in mono;
  a date's label is no longer a foreign property, a phone's still is.
  The field «Anniversary» of the form's head is gone.

### For consumers

The contact's form: rows `date.N.mark` and `date.N` in the set «Dates»
instead of `anniversary`. A patch has `model.SetGroup`. Fixtures of the
contact's edit re-shot.

## v0.2.310 — 2026-10-16

### Changed

- **A person added with a temporary password, and the installation check,
  by their frames** (Ад12, Ад13, Ад29, Ад30 of the scan of 15.10.2026;
  `s-admin-4`, `-17`, `-19`): the form refuses every field at once — login,
  email, password each at its row (`user.CheckNewUser`), not one refusal a
  sending; the answer that issues the password is its card alone, without
  the form, the badge and the warning around it; the installation check has
  its form in its head — the address it asks (`url`, monospace) and «Run the
  check» — and says how long it ran, «4.2 s».

### For consumers

`AdminInstallPage` in C-6: `PublicURL` and `Actions` give way to `Form`;
`Took` is new. Fixture `admin-install/normal` re-shot.

## v0.2.309 — 2026-10-16

### Changed

- **The edit forms of a task, an event and a note by Typecase `49412c3`**
  (its note 71; tasks 172, 173, 175, 179–181): a task's List stands beside
  its Summary, an event's Calendar after Tags — in a new record's form too,
  which is made in the collection chosen (`s-tasks-14`, `s-agenda-15`); a
  Link left empty says «optional», as a task's Location does; a task's
  Repeats says its rule under Advanced RRULE — «A task repeats from its
  start date, or from the due date if it has none.» —, and Every reads 1
  until another is typed, a record that does not repeat too.
- **A note's people are not edited** (Typecase's Р-64: the standard has no
  invitation to a note, only a copy): its edit has no field of them and a
  save keeps them; the notebook's list has no column «Attendees». A task's
  stands last and off in its menu (`s-tasks-12`).

### For consumers

`NoteRow.Attendees` and `NoteDoc.Attendees` are gone from C-6. Fixtures of
the edits of a task, an event and a note, of the notes' lists re-shot.

## v0.2.308 — 2026-10-16

### Changed

- **Administration by its frames** (Ад1, Ад2, Ад3, Ад5, Ад6, Ад9, Ад10,
  Ад16, Ад26 of the scan of 15.10.2026; `s-admin-2`, `-3`, `-7`, `-13`,
  `-14`, `-17`): the rail's rows have their icons and the rail no line of
  davloom; the list of people says what it holds above it
  (`AdminUsersPage.Subtitle`: «6 accounts · 2 administrators · 4 covered
  by escrow») and sifts by login (`Bar.FilterForm`) and by All ·
  Administrators · Disabled (`Bar.Segments`, `?show=`); the journal writes
  its moments in numbers and a dash for the target of an action on oneself;
  the field that confirms a reset shows the login chosen; Copy of a
  password issued has its icon; the instance's settings are «Settings».
- A moment in numbers (`numStamp`) keeps the person's clock: «2026-08-10
  8:41 AM» on twelve hours.

### For consumers

`AdminUsersPage.Subtitle` in C-6; fixtures of administration re-shot.

## v0.2.307 — 2026-10-16

### Added

- **A found contact and a found file are rows of their kind** (Пк12 of
  the scan of 15.10.2026, in part; `s-find-1`): `FindRow.Avatar`
  (initials and colour), `Subtitle` (the phone, mono), `Detail` (the
  address), `Caption` (the organisation) of a contact; `Icon`, `Size`,
  `Changed` (in numbers), `Access` («rw», «ro») of a file. An event's and
  a task's rows wait for the frames' answers (В1, В2 of the scan).
- Fixtures of Connections' states (Нс30, in part): `add` (`s-profile-2`),
  `label-taken` (`s-profile-11`), `feed` (`s-profile-10`).

### For consumers

`FindRow.Avatar`, `.Subtitle`, `.Caption`, `.Icon`, `.Size`, `.Changed`,
`.Access` in C-6; a contact's `Detail` is its address only now (it was
the phone when there was none). Three state names in C-14.3.

## v0.2.306 — 2026-10-16

### Changed

- **The panel of a subscription is the frame's** (Нс27 of the scan of
  15.10.2026; `s-profile-10`): the address and the name with the frame's
  hints; «Label colour» — the palette, the one derived from the address
  chosen and said so («derived from the address»), kept with the
  subscription; «This feed asks for a password» — a tick, the login and the
  password hidden until it is ticked (`FormRow.Changed`: `asks`) and read
  only with it; «Sections» after a check — what the feed carries, not a
  choice; `ConnectionsPage.AddNote` — the warning under the form that the
  server, not the browser, reads the feed; Cancel beside Subscribe.
- **The source of a restore is an address of its step** (Нс74;
  `s-backup-7`): `RestoreBlock.From` — From a job · From a server · From
  this computer, links to `?restore=1&from=…`, `FromHint` under them — and
  the step's form holds the fields of the one chosen alone: a job's copies
  one row a job (its newest verified copy; its day, size and collections in
  `Meta`), a server's storage and path, this computer's file; the archive's
  password with the frame's words; Cancel.

### For consumers

`ConnectionsPage.AddNote`, `RestoreBlock.From`, `.FromHint` in C-6;
fixtures of Connections, Backup and the sheets over Connections re-shot.

## v0.2.305 — 2026-10-16

### Added

- **The sheet of deleting a record** (note 69 of Typecase, task 166;
  `s-agenda-26`, `s-tasks-18`; the owner's decisions А8, А3, А12): Delete
  posts as before, and a record that has something to lose answers with
  `RecordDeleteSheet` instead of going — where it is deleted and that
  Carrel keeps no copy; «What is pointing at it now» (its published links,
  the records related to it among the loaded ones); that the server, which
  sends invitations itself, may tell its attendees; its attachments —
  written into it, going with it, or a link that stays, the file of one in
  the attachment folder deletable with it by a tick (`attachment`); its
  copies, the one opened ticked (`copy`). The sheet's Delete (`sheet=1`)
  deletes the ticked files, the ticked copies and the record, unless its
  own copy was unticked. A record with nothing to lose is deleted at once.
  Delete is aimed at the sheet zone now (C-5: «redirect or sheet»).

### Changed

- The three fixture tests that wrote one file each compare through
  `compare`: one run with `CARREL_FIXTURE_WRITE` writes every fixture.

### For consumers

`RecordDeleteSheet` in C-6, its row in C-5; fixtures `app-record-delete`
(`normal`, `single`); `target: sheet` on every Delete of a record.

## v0.2.304 — 2026-10-16

### Removed

- **A note's status and an event's priority** (note 69 of Typecase, tasks
  176–178; the owner's Р-63: kinds are equal in what is common, not in
  their content): no Status field in a note's edit nor `NoteMeta.Status`, no
  Priority field in an event's edit nor line on its card, no Status and
  Priority columns of the agenda (`AgendaRow.Status`, `.Priority`) nor
  Status of the notes (`NoteRow.Status`). What came from the server stays:
  a note's STATUS and an event's PRIORITY are kept as sent — shown under
  «Kept as the server sent them», written back untouched.

### For consumers

C-6 without those fields; fixtures of the agenda, the notes and the edits
re-shot.

## v0.2.303 — 2026-10-16

### Changed

- **The publish sheet of a calendar kind is the frame's** (note 70 of
  Typecase, task 163; `s-publish-2`, `-19`): how much of it (a calendar),
  then «What the visitor sees» — Time with «always shown» beside it
  (`LabelAside`, not a hint), the title on, the rest off, and under the set
  `FieldSet.Hint`: «Without the title an event shows as “Busy”.» («the
  event» of one) —, then «Allow .ics download and subscription», then «Link
  lives for». One event or task asks of its .ics too, ticked at first, and
  the sheet's post says of the .ics of any calendar kind.

### For consumers

`FieldSet.Hint` in C-6.

## v0.2.302 — 2026-10-16

### Added

- **A link a letter makes says its terms, and they can be changed** (note
  70 of Typecase, task 165; `s-mail-1`; the owner's decision Б38):
  `MailAttachRow.Terms` — how long it lives, as the list of links says it
  («30 days», «No expiry») — and `MailAttachRow.Change` — «Change…», the
  publish sheet over the letter (`/app/publish?…&mail=<part>`) with the
  part's terms, its main button «Apply» (`publish.apply`, POST
  `/app/mail/compose?op=terms`): the terms stay in the letter, nothing is
  made, and the link is made when the letter goes — living as long and
  showing what the terms say. A link published before the letter has
  neither.
- `MailSheet.Links` — the line under the attachments when some go as links
  (`s-mail-1`): how many and why — a folder always, a file over the
  attachment's ceiling, one the letter has no room for, a link published
  before — and that each can be revoked in **Published links**, «including
  for the people in this letter». The sentence of the limits that counted
  them is gone.
- What the publish sheet chooses is one function for the sheet and the
  letter (`publishChoices`).

### For consumers

`MailAttachRow.Terms`, `.Change`, `MailSheet.Links` in C-6.

## v0.2.301 — 2026-10-16

### Changed

- **The question about a connection's links waits in its dialog** (note
  70 of Typecase, task 164; `s-profile-22`, `-23`): the edit of a
  connection that published links use carries «Published links» at once,
  hidden, with `FormRow.Changed` — `server`, `login`: with a script it
  shows while one of them differs from what came; without one it comes
  after the first Save, as before (Б40). A label, a password, a
  fingerprint move the links along without a word.

### Added

- Fixtures of the edit dialog (Нс76): `app-settings-connections/edit`
  (`s-profile-16`) and `edit-links` (`s-profile-23`).

### For consumers

`FormRow.Changed` in C-6; two state names in C-14.3.

## v0.2.300 — 2026-10-16

Typecase `22f0506` (its note 69).

### Changed

- **Icons of the frames**: Attach `i-clip`, «From this computer» and
  Upload `i-upload`, Export `i-export`, Import `i-import`, Download and
  «Download … first» `i-download`, Restore `i-restore`; «Export CSV» of the
  journal has none, as its frame.
- **No «New task» on the cards** of an event, a note and a contact (the
  owner, task 168): a task is made in the tasks; a person's timeline keeps
  its own.
- **A note's export is «.md»** with the export icon, «Export Markdown» its
  hint (`Action.Hint`).
- **A task list has no order «Title»** (task 167: a task's date matters)
  and has **«Filter by word»** — `ListBar.FilterForm`, a GET to the list
  that keeps the tasks whose title or text holds the word —, combined and
  of one collection.

### For consumers

Fixtures re-shot (icons, the cards' actions, the task lists' bar, the set:
the task «Отправить документы в МФЦ» carries three properties Carrel does
not know — `s-tasks-3`, «Kept as the server sent them»).

## v0.2.299 — 2026-10-16

### Added

- **The card of a subscription is the frame's** (Нс25, Нс26 of the scan of
  15.10.2026; `s-profile-1`). `SubscriptionCard.ID`; `Access` — «no
  password» or «password saved»; `Password` — Show password of a link with
  a saved one, behind the unlock window a connection's is (the sheet names
  the feed; the journal says `connection.password.reveal`); `Lands` — the
  table «Lands in», a row a kind of the feed (`FeedKindRow`: the kind, its
  word, its tick, how many the last read held, its tick as POST
  `?op=tick&kind=…`) and `Changed`, one for the rows; `State`
  unavailable with `Note` and «Check again» when the last fetch came back
  refused — «The feed asks for a password and the saved one no longer
  fits. A feed that stopped answering must not look like a feed with
  nothing in it.» for a saved password that stopped fitting —, while
  `Checked` keeps when it was last read.
- `op=tick` of Connections takes `kind`: one kind of a collection or of a
  feed. A feed answering other than 200 is `publish.StatusError` (its words
  are as before), so a refused password is told from the rest.

### For consumers

`SubscriptionCard.ID`, `.Access`, `.Password`, `.Lands`, `.Changed`,
`.State`, `.Note` and `FeedKindRow` in C-6; fixtures of Connections and of
the sheets over it re-shot.

## v0.2.298 — 2026-10-16

### Added

- **A connection whose server refused the last read says so** (Нс23 of
  the scan of 15.10.2026; `s-profile-1`, the card of baikal): a poll that
  comes back refused is kept in the session as an answer too, and while it
  is newer than every read of the connection's collections its card is an
  alarm — `ConnectionCard.Refused` («last answer refused» instead of a
  time; `Answered` empty), `Test again`, and `ConnectionCard.Note`, the
  line under it: «The server refused the password. It may have been
  changed on the server: the one Carrel saved no longer fits.» for a 401,
  the refusal itself for any other. A read that answers after it puts the
  card back.

### For consumers

`ConnectionCard.Refused`, `.Note` in C-6. Fixture
`app-settings-connections/normal` is taken after the lists were read, as
`s-profile-1` draws it (times, counts, baikal refused); the fixtures of
the collection sheets over it re-shot.

## v0.2.297 — 2026-10-16

### Added

- **A collection's row carries its tick** (Нс21 of the scan of
  15.10.2026; `s-profile-1`, `-10`: «a collection unticked shows an
  unticked box: it is not polled»). `CollectionRow.Tick` — on when it is
  polled in every kind it holds, part in some, off in none;
  `CollectionRow.Toggle` — POST `/app/settings/connections?op=tick`: a
  collection ticked everywhere is unticked everywhere, any other ticked
  everywhere, and the page comes back. Its label is the collection's name.
  How the frame sends it without scripts is Typecase's task 193, item 7.

### For consumers

`CollectionRow.Tick`, `.Toggle` in C-6; fixtures of Connections and of
the collection sheets over it re-shot.

## v0.2.296 — 2026-10-16

### Changed

- **The Email message sets its addresses in mono** (Нс10 of the scan of
  15.10.2026; `account-9`, `-10`): `Notice.Runs` — the body in pieces when
  the frame marks a part of it, `Body` then empty. «…the link we sent to
  ‹mix@example.org› on …» and «No link has gone to ‹new@example.org›: until
  one is opened, ‹mix@example.org› stays in use.»; the message without an
  address (`s-mail-9`) stays `Body`. Every other notice is as it was: the
  field is left out of the JSON when empty.

### For consumers

`Notice.Runs` in C-6; fixtures `app-settings-account/email-sent`,
`email-unsent` re-shot. The client draws it from `v0.4.47`.

## v0.2.295 — 2026-10-16

### Added

- **Fixtures of the account's states** (Нс9, Нс11, Нс13, Нс16, Нс17 of
  the scan of 15.10.2026), each against the frame that draws it
  (`STATE_FRAME` of `check.py`): Email — `email-confirmed` (`account-8`),
  `email-sent` (`account-9`), `email-unsent` (`account-10`),
  `email-unconfirmed` (`s-mail-9`); Key escrow on the account —
  `escrow-deposited` (`account-7`), `escrow-off` (`account-13`); the
  escrow page — `normal` is now the copy deposited (`account-4`, the
  route's frame), `no-copy` (`account-12`), `escrow-off` (`account-14`);
  the deposit and withdraw sheets — their own `app-settings-escrow-deposit`
  (`account-2`) and `app-settings-escrow-withdraw` (`account-11`); the
  password change after a temporary password — `app-password/forced`
  (`account-3`). Nothing in the running server changed. Check:
  `TestFixtureAccountStates`.

### For consumers

`app-settings-escrow/normal` is the deposited copy now; what it was is
`no-copy`. Eight new state names in C-14.3.

## v0.2.294 — 2026-10-16

### Changed

- **The escrow page carries the deposit sheet only when a deposit came
  back refused** (Нс14 of the scan of 15.10.2026; `account-12`): opened,
  the sheet is its own answer (`…/escrow/deposit`, `account-2`), and the
  page without a copy draws no sheet. `EscrowPage.Confirm` is set only in
  the answer to a refused deposit without scripts.

### For consumers

Fixture `app-settings-escrow` re-shot: no `confirm`.

## v0.2.293 — 2026-10-16

### Changed

- **A collection's row counts its objects** (Нс22 of the scan of
  15.10.2026; `s-profile-1`: «Objects» — «128»): `CollectionRow.Objects`
  is how many records it held when this session last read it, every kind
  it holds together; before any read, and for a folder of files, a dash.

### For consumers

The collection sheets' fixtures (Connections under them) re-shot.


## v0.2.292 — 2026-10-16

### Changed

- **The password sheet says whose password it asks for** (Нс18–Нс20 of the
  scan of 15.10.2026; `s-profile-3`). Show password of a connection opens
  `/app/settings/unlock?for=<connection>`: the sheet's head is «Show
  password» with the connection beside it (`UnlockSheet.Subject`), so two
  cards in a row cannot be confused. Under the field one line, what the
  window opens and for how long — «Unlocks credentials everywhere for five
  minutes: passwords, device profiles, key escrow — anything handed out or
  handed over.» (the time from `CARREL_UNLOCK_TTL`); the line about the
  audit log is gone. The form has Show and Cancel, Cancel back where it was
  opened.

### For consumers

The connection's `Reveal.Href` carries `for=`. Fixtures
`app-settings-unlock`, `app-settings-connections`, `app-settings-escrow`
and the collection sheets (their Connections under) re-shot.


## v0.2.291 — 2026-10-16

### Changed

- **The tables of the settings and the administration write a moment in
  numbers** (Нс2 of the scan of 15.10.2026; `account-1`, `s-davloom-1`,
  `s-backup-1`, `s-admin-2`): a session's start, a davloom device's last
  request, a backup run's start and a person's last sign-in are
  «2026-08-16 09:02»; a device's and a person's creation «2026-06-02».
  When a session was last seen stays in words («just now»).

### For consumers

Fixtures `app-settings-account` and `admin-users` re-shot; a fixture's
moment of the run in numbers is the day of the frames, «2026-08-10 17:04».

## v0.2.290 — 2026-10-16

### Changed

- **The unlock mark says what is left** (Нс7 of the scan of 15.10.2026;
  `s-profile-4`: «credentials unlocked · 4:12»): `TopBar.UnlockLeft` —
  minutes and seconds left of the window when the page was made, beside
  `Unlocked`, the moment it ends.


## v0.2.289 — 2026-10-16

### Changed

- **A piece of a phrase can carry a hint** (Нс5 of the scan of 15.10.2026;
  `account-*`): `Run.Hint`. «verified» in the rail's backup line says
  «The archive was read back from the destination and the checksum
  matched». A piece with a hint is not run together with its plain
  neighbours.


## v0.2.288 — 2026-10-16

### Changed

- **Key escrow stands under Account in the rail** (Нс4 of the scan of
  15.10.2026; `account-4`): the rail has no row of its own for it, and
  Account is the one on — before, none was.
- **A shown password's button is «Hide»** (Нс29; `s-profile-4`), not
  «Hide password».

### For consumers

Fixture `app-settings-escrow` re-shot.

## v0.2.287 — 2026-10-16

### Changed

- **What matched the query is marked in a result's title** (Пк13 of the
  scan of 15.10.2026; `s-find-1`: «**Ковал**ёва Анна»): `FindRow.Marked`
  is the title in pieces, the first match of the query strong, whatever
  the case; empty when the result was found by another field.

### For consumers

Draw `Marked` where it is set, else `Title`. Fixture `app-search`
re-shot.

## v0.2.286 — 2026-10-16

### Changed

- **The search says what it asked** (Пк5 of the scan of 15.10.2026;
  `s-find-1`): `SearchPage.Subtitle` — «14 of 14 collections · 7 records ·
  1 source timed out»: the collections asked of those ticked, everything
  found, and what did not answer by its kind (refused, unavailable, timed
  out).
- **The line about files speaks as the frame** (Пк14): `SearchPage.Note`
  is `Runs` — «Files are searched by name: ‹PROPFIND› returns names anyway,
  and walking a folder costs one request per folder. Carrel does not look
  inside files — and says so here, so that “nothing found” is not read as
  “not in the files”.», PROPFIND monospaced.
- **A feed in the search's rail is said as one** (Пк3): `SourceRow.Feed`.
- The groups of a kind with their count are the screen's (Пк11, C-6.6);
  U-6.6 and S-15.2 no longer say the search has no Retry of one source
  (Пк21).

### For consumers

`SearchPage.Note` is `Runs` now. Fixture `app-search` re-shot.

## v0.2.285 — 2026-10-16

### Changed

- **A feed is a source of the duplicates** (Дб3 of the scan of 15.10.2026;
  `s-dup-1`: «Праздники РФ» in the rail, `is-ext`). The records of a feed
  read in this session are compared with the rest, and its row stands in
  the rail with `Feed`. A group with a feed's record can be linked, never
  merged: a feed is read, not written (Merge is off with the read-only
  reason).
- **A group's records read in the catalog's order** — its connections as
  the rail lists them, then collection and UID — not by the account's ID,
  which is random: the card's rows and the merge's «Where the merged
  record stays» read the same way every time (`s-dup-4`: «nextcloud ·
  Работа», then «radicale · vps · Дежурства»).

### For consumers

The duplicates fixtures' rails gain the set's feeds.

## v0.2.284 — 2026-10-16

### Changed

- **The duplicates with nothing read are the empty screen of `s-dup-6`**
  (Дб10 of the scan of 15.10.2026): no segments and no cards — the head
  with the threshold and the two notices. A linked group is no longer
  shown with «Not loaded in this session» before its lists are read: the
  decision is kept and comes back with them.

### For consumers

New fixtures: `app-duplicates/empty` (`s-dup-6`), `app-duplicates-merge-event/normal`
(the merge of an event, `s-dup-4`), `app-duplicates-merge/in-part` (a
merge whose delete the server refused, `s-dup-5`); `app-duplicates/normal`
is now All, in a session that read the four sections (Дб12). The rail of
the event's merge waits for Дб3 (a feed as a source) in `SERVER_TODO`.

## v0.2.283 — 2026-10-16

### Changed

- **A record without a value is a choice too** (Дб7 of the scan of
  15.10.2026; `s-dup-1`: Вера's «Organisation» — «—» or «Библиотека №4»,
  her «Photo» — «nextcloud · Архив» or «nextcloud · Контакты»). Where one
  member of a group holds a single value another does not, the group asks:
  the option of the member without it is an empty label (the screen's
  dash; a photo's is its source), and the value is chosen beforehand, so
  nothing changes unless the person says so. «—» chosen leaves the
  property out of the linked record and of the merge: the merge removes
  it from the kept record (`dupes.Leave`) and says so under «Not carried
  over» («ORG: … — left out, as you chose»). The written rule S-16 — «a
  field only one record has is taken» — gains this choice.

### For consumers

`MergeRow.Options` may hold an empty label: draw it as «—». Fixture
`app-duplicates` re-shot: Вера's group asks Organisation and Photo, the
linked Артём's Photo.

## v0.2.282 — 2026-10-16

### Changed

- **A group says what such a pair means** (Дб6 of the scan of 15.10.2026;
  `s-dup-1`): `DuplicateGroup.Hint` under its rows — for events under
  different UIDs not yet decided, «The same event in two calendars,
  different UIDs. Agenda shows it twice; here you decide whether to keep
  both or write one and delete the other.»; for notes, «Same title and the
  same date: a note copied into a second notebook. Linked, the two are
  shown as one row; merging writes one and deletes the other.». A group
  marked «not duplicates», of contacts or of tasks says nothing.

## v0.2.281 — 2026-10-16

### Changed

- **The sources' rubric counts apart** (Дб2, Пк2 of the scan of
  15.10.2026; `s-dup-1`, `s-find-1`): `RailSection.Account` is «Sources»
  and `RailSection.Tally` the count, monospaced — «3 / 3», «14 / 14» —
  on the duplicates and on a search.
- **The merge stands on every source compared** (Дб20; `s-dup-2`, `-4`,
  `-5`): its rail, the outcome's and a lost group's, carries every
  collection the duplicates compare, not only those of the group's kind.

### For consumers

Read the rubric's count from `Tally`. Fixtures `app-duplicates`,
`app-duplicates-merge` and `app-search` re-shot.

## v0.2.280 — 2026-10-16

### Changed

- **The plan of a merge comes as fields, each under its rubric** (Дб14,
  Дб15, Дб19 of the scan of 15.10.2026; `s-dup-2`, `-4`, `-5`).
  `DuplicatesMergePage.Stays` is the segment «Where the merged record
  stays» — every record's source, the kept one on, another the same page
  with it kept, a read-only one off with its reason — and `StaysNote` the
  line under it («Вера Лебедева there is rewritten with the values you
  chose.»); the actions «Keep … instead» are gone from `Actions`.
  `Deleted` names the records deleted afterwards (title and source),
  `Gains` what the kept one gains («EMAIL;TYPE=work: …», «PHOTO (from
  nextcloud · Контакты)», «2 reminders from the others», the time taken),
  `NotCarried` the rewritten occurrences that go. «(from …)» stands only
  where a value does not show itself — the photo. The warning keeps its
  title and body; its `Details` no longer carries the plan.
- **A merge that did not end in one record says what happened as a list**
  (`Happened`): «Kept and rewritten: …», «Deleted: …», «Still there: …»;
  what the server answered stays in `Details`. The refusal names the record
  by its title («Лебедева Вера refused»), as `s-dup-5`.

### For consumers

Read the plan from `Stays`, `StaysNote`, `Deleted`, `Gains`, `NotCarried`
and the outcome from `Happened`; choose where the record stays from
`Stays`. Fixture `app-duplicates-merge` re-shot.

## v0.2.279 — 2026-10-16

### Changed

- **The duplicates say what they compared** (Дб1, Дб13 of the scan of
  15.10.2026): `DuplicatesPage.Subtitle` — «7 records compared · 1 to look
  at» (`s-dup-1`; with a kind chosen, its groups), «0 records compared»
  when nothing waits; `DuplicatesMergePage.Subtitle` — «Вера Лебедева · 2
  records» (`s-dup-2`).
- **The plan of a merge is said, not alarmed** (Дб16): its notice is
  `info`; the button that deletes stays the alarm.
- **A photo is told by its source whole** (Дб8): «nextcloud · Архив», not
  the server alone.
- **The verb agrees with the count** (Дб18): «1 rewritten occurrence of …,
  which belongs to its series and goes with it»; so the line «Not kept».

### For consumers

Fixtures `app-duplicates` and `app-duplicates-merge` gain their subtitles;
the merge's warning is `info`.

## v0.2.278 — 2026-10-16

### Changed

- **A conflict Carrel cannot read chooses nothing beforehand** (the owner's
  decision of 16.10.2026; Т1 of the scan of 15.10.2026). When the version
  the form was opened with is no longer held, every card has no side chosen
  (`ConflictRow.Choice` empty) — before, «Your edit» everywhere, and writing
  the combination without a look put back what only the other client
  changed. «Write this combination» is off with «Choose a side for every
  property.» until every card has a side; a combination that comes with a
  card unanswered writes nothing and the screen comes back with «Nothing
  was written». «Use only my edit» stays available, with the notice «Some of
  these may not be your changes» above the cards. A card not sent with the
  form takes what was chosen beforehand, not the person's edit.

### For consumers

`ConflictChoice` can be empty (`ChoiceUnset`): neither radio on. Typecase's
task 198 asks how the combination is drawn without scripts until every side
is chosen.

## v0.2.277 — 2026-10-15

### Changed

- **The sheets of settings and administration carry the screen under
  them** (Лс29 of the scan of 15.10.2026; the owner's decision of
  15.10.2026): opened by their address, the password sheet stands over the
  page it was opened from (`back`, else `Referer`, else Connections —
  «Set up a new device» in `s-plan-3`, Connections in `s-profile-3`), the
  deposit and the withdrawal of the key over Account (`account-2`, `-11`),
  the reset of a password over Users (`s-admin-13`), Disconnect over
  Connections, the export's choice of a collection over the list it was
  opened from. `Under` knows `Account`, `Devices` and `AdminUsers`.

### For consumers

`UnlockSheet`, `EscrowWithdrawSheet`, `PasswordResetSheet`,
`ConnectionRemoveSheet`, `ExportSheet` gain `Under` (C-6). Every fixture
that carries an `under` gains its three new keys, empty; `app-settings-unlock`
carries Connections under it.

## v0.2.276 — 2026-10-15

### For consumers

Nothing in the running server. Fixtures `app-collections-rename` and
`app-collections-delete` re-shot over the calendar «Дни рождения», as
`s-profile-7` and `-9` draw them (Кл10, Кл20): its colour `#a05a2c`
chosen, «Rename and colour»; the delete says the calendar and its one
event, and that nextcloud may tell the attendees of its meetings they are
cancelled. The set holds no published link, backup job or davloom device
of it, so the delete's references are empty (the frame draws three).

## v0.2.275 — 2026-10-15

### Changed

- **What points at a collection is said in pairs** (Кл16; `s-profile-9`,
  `-19`): `CollectionDeleteSheet.References` is `[]Reference` — `What`
  («1 published link», «Backup job «Личное на домашний диск»», «3 devices
  through davloom», «Attachments») and `Then` («answers “not found” from
  that moment»); the dash between them is the screen's.
- **Deleting a calendar says its attendees may hear of a cancellation**
  (Кл18; the owner's decision А12): `CollectionDeleteSheet.Cancels`, as the
  move of a calendar says it, when the calendar holds anything and its
  server schedules.

### For consumers

Read `References[i].What` and `.Then` instead of one line. Fixture
`app-collections-delete` gains `cancels` (empty: an address book).

## v0.2.274 — 2026-10-15

### Changed

- **The move sheet of a collection is a form, as `s-profile-8` draws it**
  (Кл11, Кл12, Кл13 of the scan of 15.10.2026). `CollectionMoveSheet.Form`
  posts to the move's address with `_csrf` (and `copy` for a copy); the
  server chosen goes under `Target.Field` «to», each option's `Value` its
  connection; Move (Copy) submits it. Opened with nothing chosen, the first
  server that suits is chosen and the sheet says what will happen — Move is
  no longer off with «Choose a server.». A server the session knows does not
  answer is closed: «password · 401» when the last polls were refused the
  saved password, else «not answering»; chosen by its address, Move is off
  with the reason.

### For consumers

Fixture `app-collections-move` re-shot as `s-profile-8`: «Дни рождения»
over Connections, the calendar read first; radicale chosen, baikal closed
«password · 401». The count stays «all its events» (the set's calendar
holds one event; the frame draws 51).

## v0.2.273 — 2026-10-15

### Changed

- **The conflict screen is a card per label and per property outside the
  form** (Рв6 of the scan of 15.10.2026; `s-conflict-1`). A field whose rows
  have labels — phones, e-mails, addresses, websites — is a card for the
  rows of each label that differ, named as the record names them:
  `TEL;work`, `EMAIL;work`; a value no longer carries « · work» after it. A
  relabelled row is two cards, the label it came to and the one it left.
  Writing a card replaces the rows of that label in the server version and
  leaves the rest as the server holds them.
- **A property the form does not show that only the server changed is a
  card of its own** — `X-EVOLUTION-FILE-AS`, «only on the server», the
  server's chosen. «Your edit» puts it back exactly as the version opened
  held it (its line, parameters and group; none — removed); «Use only my
  edit» leaves it. The record's identity and stamps, its photo and its files
  are not compared. Without the version opened (the «Some of these may not
  be your changes» notice) there are no such cards.

### For consumers

`ConflictRow.Property` can be `TEL;work` and a property Carrel does not
model (C-6.5). Fixture `app-conflict` re-shot with the data of
`s-conflict-1`: FN agrees; TEL;work and NOTE only in the edit; EMAIL;work
both changed; X-EVOLUTION-FILE-AS only on the server.

## v0.2.272 — 2026-10-15

### Changed

- **The sheets of a collection speak as their frames** (Кл2, Кл3, Кл9, Кл14
  of the scan of 15.10.2026): the address is the path from the host's root,
  its start an ellipsis — «…/remote.php/dav/calendars/mix/birthdays/»
  (`s-profile-7`), and so is the example under a new collection's address
  (`-5`); the colour of a new calendar says «Written to the server as
  calendar-color — DAVx5 and Thunderbird will show the same one.», of a
  renamed one nothing; the move of an address book says «an address book».

### For consumers

Fixtures `app-collections-new`, `app-collections-rename` re-shot. A feed's
reading time in a fixture is set to «0 ms» (the machine's otherwise; the
Connections page under a collection's sheet carries it since `v0.2.270`).

## v0.2.271 — 2026-10-15

### Changed

- **A field only the server changed is a card of the conflict screen**
  (Рв2, Рв4 of the scan of 15.10.2026; `s-conflict-4`: «DUE · only on the
  server»): `Diff` `theirs`, the server's chosen; «Your edit» is what was
  opened, and choosing it writes that back. The refused edit keeps the whole
  version the form was opened with; a time zone goes with its time, not as
  a card of its own. Before, such a change was kept without a word. «Use only
  my edit» writes the person's changes and leaves these as the server has
  them.
- **The fixtures of the conflict of an event, a task and a note are their
  frames' data** (Рв11): `s-conflict-3` — SUMMARY both, LOCATION only in the
  edit, DTSTART agrees; `-4` — SUMMARY both, DUE only on the server, STATUS
  agrees; `-5` — SUMMARY and DESCRIPTION both, CATEGORIES agree. The
  contact's waits for the rows by parameter (Рв6).

## v0.2.270 — 2026-10-15

### Changed

- **The sheets of a collection carry the screen under them** (Кл1 of the
  scan of 15.10.2026; the owner's decision of 15.10.2026): New, Rename,
  Move and Delete of a collection answered as a page stand over the page
  they were opened from — `back`, else the Referer —, else over Connections
  (`s-profile-7`…`-9`); `Under.Connections` is new.

### For consumers

C-6: `Under` of `CollectionNewSheet`, `CollectionRenameSheet`,
`CollectionMoveSheet`, `CollectionDeleteSheet`; `Under.Connections`.
Fixtures of the four sheets re-shot, and every sheet's with `under` (a new
empty key).

## v0.2.269 — 2026-10-15

### Changed

- **Registration and the first setup past the ceiling of attempts answer
  their form**, the words at the login, as the other forms that take a
  password do since `v0.2.266`; status 429 as before.
- **The rail of the search lists its sources by kind** (Пк4 of the scan of
  15.10.2026; `s-find-1`): address books, calendars, lists of tasks,
  notebooks, storages — within a kind as the section's rail orders them,
  subscriptions last; before, in the order they were ticked.
- **The history of the search says when beyond a day as `s-find-2`** (Пк20):
  «yesterday», the day of the week within the week, then the day.

## v0.2.268 — 2026-10-15

### Fixed

- **Create password of davloom keeps what was typed** (Нс73 of the scan of
  15.10.2026): it sends the dialog; outside the unlock window the dialog
  comes back with the name, the access, the kinds and the collections as
  typed, and asks for the person's own password under them («Your Carrel
  password»); with it, the window opens and the device's password is made
  in one step. Before, Create password was a link to the password's sheet,
  and the dialog came back empty after it. How the step looks is the
  frames' (Typecase's task 193, item 3).

### For consumers

Fixture `app-settings-davloom/normal` re-shot: Create password is a POST of
the dialog.

## v0.2.267 — 2026-10-15

### Changed

- **Each card of the conflict screen says how the property differs**
  (`ConflictRow.Diff`, C-8 `ConflictDiff`; Рв2 of the scan of 15.10.2026;
  `s-conflict-1`…`-5`): `both` — the server changed it too; `mine` — the
  server holds what the form was opened with, the change is the person's
  alone. The refused edit keeps what that version held of the fields
  changed (`Base`); without that version (`Unsure`) the kind is not known
  and is empty. «only on the server» comes with the properties outside the
  form (Рв6). The words — «both changed», «only in your edit» — are the
  screen's.

### For consumers

C-6 `ConflictRow.Diff`, C-8 `ConflictDiff`. Fixtures `app-conflict`, `-event`,
`-task`, `-note` re-shot.

## v0.2.266 — 2026-10-15

### Fixed

- **A guest sees no marks of the owner's** (Пс23 of the scan of 15.10.2026;
  `s-publish-6` draws the caption empty): «linked ×2», «2 sources» and the
  like are taken off the rows of a published calendar, book, list and
  notebook.
- **A published folder cut at its ceiling says so** (Пс22): «Showing N of
  M», as the owner's own folder does.
- **Past the ceiling of attempts a form that takes a password answers the
  form again, the words at its field** (Ад39): sign-in, the password's
  sheet, the change of password, the deposit of the key and Recover in Key
  escrow — «Too many attempts just now. Wait a few minutes and try again.»,
  status 429 as before; before, a bare «too many requests». Registration and
  the first setup still answer as before.

## v0.2.265 — 2026-10-15

### Changed

- **The conflict screen speaks as `s-conflict-1`…`-5`** (Рв1, Рв3, Рв5, Рв7,
  Рв8 of the scan of 15.10.2026): what happened is the line under the title
  (`ConflictPage.Subtitle`: «Another client changed this contact while you
  were editing it. Nothing has been overwritten.»), not an alarm «Conflict»
  over the cards; a property the person changed to what the server already
  holds is named above the cards (`Agree`, «‹DTSTART› agrees — kept as it
  is.»); an empty value is empty — the screen draws its dash, not «Empty»;
  Cancel is a link to the record; the head's refresh reads the page again.

### For consumers

C-6 `ConflictPage`: `Subtitle`, `Agree`. Fixtures `app-conflict`,
`app-conflict-event`, `-task`, `-note` re-shot.

## v0.2.264 — 2026-10-15

### Changed

- **The import report names what it left out and what failed as the
  frames do** (Им15, Им16 of the scan of 15.10.2026; `s-io-9`, `-14`): a line
  for each record skipped, «Купить билеты · VTODO: a calendar takes events —
  import it into a list of tasks» (`ImportReport.Skips`); a failure begins
  with the record's title, or «— (no FN)», «— (no SUMMARY)» without one, not
  with «VCARD».
- **The answer about records already here is `existing=copy`**, the frames'
  word (Им7); `copies` is still understood. C-5 names `existing=`.

## v0.2.263 — 2026-10-15

### Fixed

- **A sheet stands over the page it was opened from, and Cancel goes back
  there** (Лс30, Лс31 of the scan of 15.10.2026): the move sheet and the
  sheets of copies read `back`, else the page the link was followed from
  (its Referer), and Cancel leads there, not to the record's page; the move
  sheet's collections carry `back` on. A sheet that answers a form (the
  series question) does not read the Referer — that is the form.
- **The quick sheet carries the screen under it** (`NewSheet.Under`, Лс28;
  `s-notes-5`, `-8`, `-9`, `-10`): where New was pressed, else the list it
  writes to; New of a section carries `back` to the sheet it leads to.
- **Rewrite N records sends the sheet's form** (Лс32; `s-contacts-19`,
  `-20`, `-29`): the choices `to.N` and the ticks `group` went nowhere — the
  button posted on its own, and what was offered was rewritten, not what was
  chosen.
- **Mark done with no copy ticked says so** (Лс34): the sheet again,
  nothing ticked, an error at `copy` («Tick the copy that is done: nothing
  was marked.»), rather than the sheet again with the opened copy ticked, as
  if the tick had been pressed.

### For consumers

C-6: `NewSheet.Under`; `CopiesSheet` errors in `Form.Errors`; the done
sheet's form carries `sheet=1`. C-5: `back` and `Referer` of the move and
copies sheets. Fixtures `app-quick`, `app-contact-move`,
`app-contact-rewrite`, `app-task-done` re-shot.

## v0.2.262 — 2026-10-15

### Changed

- **The bar of a search done speaks as `s-find-1`** (Пк7–Пк10 of the scan
  of 15.10.2026): «Done · 13 of 14 sources answered»; what did not answer is
  the red «1 failed» whose menu names each source with its answer and
  Retry, and «Retry all» when there are more (`PollBar.Failed`,
  `RetryAll` — the search had neither); a source that did not answer in
  time — «… → no answer in 10 s; the search went on without it.»; the right
  of the bar — how long the pass took, «1.8 s», and when it answered, today
  the clock alone, «11:42». The count of what was found is the page's, not
  the bar's.

### For consumers

C-6 `PollBar`: `Failed`, `RetryAll`, `Aside`, `AnsweredAt` of the search.
Fixture `app-search/normal` re-shot.

## v0.2.261 — 2026-10-15

### Fixed

- **Merge on the server takes what the card chose** (Дб9 of the scan of
  15.10.2026; `s-dup-1`: the button is one of the card's form): on a card
  with something to choose, Merge sends the card's answers (`do=merge`) and
  the merge page opens with them chosen (`pick.<PROP>=` the member's place
  in the group, over a linked group's own answers). Before, Merge was a link
  without them, and a value chosen on a candidate's card was lost.

### For consumers

C-5: `/app/duplicates/decide` takes `do=merge` (a redirect to the merge),
`/app/duplicates/merge` takes `pick.<PROP>=`. Fixture `app-duplicates/normal`
re-shot: Merge is a POST button of the card's form.

## v0.2.260 — 2026-10-15

### Fixed

- **Notes take every file of a drop** (Им5 of the scan of 15.10.2026;
  `s-io-1`): several Markdown files sent at once are one set, as a zip is —
  each a row of the preview, a file that is not Markdown or is empty named
  with why. Before, the first file alone was read and the others were lost
  without a word. Contacts, events and tasks refuse several files whole:
  «Only notes are imported several files at once.»
- **An empty Markdown file is no note by every way in** (Им14): a folder of
  notes skips it as a zip does («Empty file: nothing to store and nothing to
  name it by»); an empty file alone is refused with those words, not «No
  file was chosen».

### For consumers

C-5: the import form may carry several `file` fields for notes.

## v0.2.259 — 2026-10-15

### Fixed

- **Send is off, with the reason, for a letter that cannot go** (Пб39 of the
  scan of 15.10.2026; Б36, Б37): a link in it revoked or run out since it was
  put in, or days of a calendar that would have to go as a link — a link
  opens the whole calendar. Before, Send stood on and refused after the
  press; the refusal is still there, and the words are the same.
- **Delete of an account in Administration is off, with the reason**
  (Ад7): the item sent no login, and the account is deleted only once its
  login is typed, so it was refused every time. It waits for its dialogue
  (Typecase's task 195); the last administrator's item keeps its own
  reason.

## v0.2.258 — 2026-10-15

### Changed

- **Save of a series that leaves meetings changed by hand without their day
  asks first again** (`AskSheet`, А1): the client draws the question since
  `v0.4.43`. The switches that held the pages off the sheets
  (`copiesOnPages`, `askOnPages`, `v0.2.249`) are gone.
- **The sheet «saved in part» names the events written apart where they
  stand** — «26 January, 12:00 and 16 February, 14:30» (`s-agenda-24`), not
  the days of the series they stood for — and carries its icon, as the
  frame draws it.

### For consumers

Fixtures `app-event-ask/normal`, `single`, `in-part` are new (two states
`single` and `in-part` added to C-14); C-5 names the POST of an edit that
answers `AskSheet` in a row of its own.
## v0.2.257 — 2026-10-15

### Fixed

- **A link a letter makes gives the guest no more than the publish sheet
  gives untouched** (Пб38 of the scan of 15.10.2026): a calendar, an event,
  a task — the time and the title, the `.ics` allowed; a contact, a book —
  the name and the organisation, no phones, addresses, photo, note, no
  `.vcf`. Before, such a link — a record or a calendar too big to go as a
  file — gave the whole of it: attendees with their addresses, descriptions,
  places.

## v0.2.256 — 2026-10-15

### Changed

- **Edit of a linked record and the tick of its row lead to the sheet of
  copies again** (`/copies`, `/done`): the client draws `CopiesSheet` since
  `v0.4.42` (the owner's decision of 15.10.2026 held them off until then).
  The series question (`AskSheet`) still waits for its screen: Save writes
  as before `v0.2.247`.
- **A copy names the value it differs by** (`s-contacts-32`): of a phone or an
  address, the first one the other copy does not hold — «Email differ ·
  v.lebedeva@example.org», not the first address both share.
- `/app/tasks/…/{uid}/done` takes `back`: the sheet stands over that page.

### For consumers

Fixtures `app-record-copies/normal` (Вера Лебедева in three books, one read
only) and `app-task-done/normal` are new; `app-event`, `app-task`,
`app-note`, `app-tasks-all` re-shot — Edit leads to `/copies`, the linked
tick to `/done`.

## v0.2.255 — 2026-10-15

### Changed

- **The collections of the move sheet stand as `s-contacts-2` draws them**:
  within a server the one the record is in first, then those it can go to,
  then the read-only ones.

## v0.2.254 — 2026-10-15

### Changed

- **The density of Appearance is the default of «Compact rows»**: a
  section this browser chose nothing for draws compact rows when the
  density is Compact. A section's own choice is kept as 1 or 0 (until now
  unticking dropped the cookie, and the density would tick it again), and
  «Reset remembered view settings» forgets those too.

## v0.2.253 — 2026-10-15

### Added

- **Appearance keeps the theme, the density and the lists' habits in this
  browser** (appearance-1; from-typecase note 50): one form — Theme (Auto,
  Light, Dark), Density (Comfortable, Compact), Lists (keep the details
  panel open, show the source column, open notes in full width), Dates and
  times —, the first three in a cookie of this browser (`carrel_look`), the
  week and the clock in the account as before. Every page carries them
  (`Shell.look`, put in as the page is drawn); «Reset remembered view
  settings» forgets them and the columns every section remembered.
  `AppearancePage` is one `form` with `reset` now.

## v0.2.252 — 2026-10-15

### Changed

- **The import's question about records already here, as the frames ask
  it** (from-typecase note 52, `s-io-5`…`-9`, `-14`): the question is the
  label of Into — «5 records of this file are already in Контакты» —, the
  answers «Skip them» · «Replace them» · «Import as copies» («… it», «…
  a copy» for one) with a hint under them (`ImportExisting.hint`); «ready»
  counts what the press writes and «already here» replaces «UID
  collisions»; nothing to write says «Nothing to import: every record is
  already here.»; the step's message no longer says «Import always
  creates». The report keeps the records skipped as already here apart from
  those of another kind (`ImportReport.here`, `.heres`) and names those
  replaced (`.replaces`).

## v0.2.251 — 2026-10-15

### Changed

- **The rewrite sheet chooses per address and per group** (the owner's
  decisions of 12.10.2026, А4 and А5; Typecase's task 157, `s-contacts-29`,
  note 65): each replaced address is a choice `to.N` — the card's addresses
  and «Do not rewrite» —, offered the one typed in its own line of the form,
  else «Do not rewrite» (until now each went to the card's default
  address); each group of risk — on servers that schedule, on a server that
  did not say, organised by others, past — is a tick `group` with its count
  and its records, and a group not ticked keeps its records; a record in two
  groups is rewritten only when both are ticked. The sheet carries
  `Tally` and the button's words by count for the script to count again
  (`RewriteSheet.groups`, `.both`, `.tally`, `.count`, `.countOne`;
  `RewritePair.choice`). A form without the ticks rewrites what was offered.
  Questions to the frames — Typecase's task 185.

## v0.2.250 — 2026-10-15

### Added

- **The question before the save of a series** (the owner's decision of
  11.10.2026, А1; Typecase's note 56, `s-agenda-22`…`-24`): an edit that
  shortens a series, gives it another rule or stops it repeating, and so
  leaves meetings changed by hand without their day, is answered with
  «Keep the meetings that were moved by hand?» (`view.AskSheet`) over the
  edit as typed — Keep writes each as an event of its own first, Remove
  takes them out, Cancel goes back to the edit; nothing is written before
  the answer. A meeting that cannot be written apart stays in the series
  as it was, and the sheet says so (`saved in part`, Try again —
  `?op=apart`). Held off with the other sheets until the client draws them
  (`sheetsOnPages`): until then Save writes as before. A day left out the
  series no longer has goes without a word, sheets or not.

## v0.2.249 — 2026-10-15

### Changed

- **The pages do not lead to the sheets of copies yet** (the owner's
  decision of 15.10.2026): the client draws no sheet, and answered Edit of a
  linked record and the tick of a linked task with JSON, where before
  `v0.2.247` Edit opened the copy's form and the tick marked the first copy.
  They do so again until the client draws sheets (`sheetsOnPages`); «Which
  copy to edit?» and «Which copy to mark done?» answer their own addresses
  as before. The frames wait for Typecase's task 183.

## v0.2.248 — 2026-10-15

### Added

- **A sheet opened by its address carries the screen under it** (the
  owner's decision of 15.10.2026; Typecase's Overlay, `s-contacts-2`): the
  page it was opened from — its `back`, else the record's or the list's
  page —, asked of the same handler its address has (`view.Under`;
  `MoveSheet.under`, `CopiesSheet.under`). A sheet asked for as a fragment
  carries none.

### Changed

- **The move sheet is a form, as `s-contacts-2` draws it**: the collection
  is a radio under `Target.field` («to», `TargetOption.value`
  «connection/collection»), Move the form's button; nothing chosen, the
  first collection it can go to is. A collection whose move needs other
  words than the sheet said (another account) is not moved: the sheet comes
  back with it chosen and its words (S-18). The links of the sheet and
  `to` + `into` keep working. Questions to the frames — Typecase's task 183.

## v0.2.247 — 2026-10-15

### Added

- **Which copy of a linked record** (from-typecase note 57, Typecase's
  Р-58): Edit of a linked record's page opens «Which copy to edit?»
  (`s-contacts-32`, `GET …/{uid}/copies`), its radios the copies — what each
  holds otherwise and what it holds of it, a read-only copy off with its
  reason —, and Edit this copy opens the form of the copy chosen, which
  says before its fields that it saves that copy only (`s-contacts-31`).
  The tick of a linked task in the combined list posts to
  `…/{uid}/done` and is answered by «Which copy to mark done?»
  (`s-tasks-16`): the copies as boxes, the opened one ticked; Mark done
  marks the ticked ones as Mark done does, subtasks with them. One view
  model, `CopiesSheet`. The client draws no sheet yet, so these answer JSON
  there, as Move does. Questions to the frames — Typecase's task 182.

## v0.2.246 — 2026-10-15

### Changed

- **The tasks and the notes offer an Attendees column**, as the agenda
  does, now that a task and a note have people (after Р3 of
  docs/plan/scans/kinds-parity.md): the Columns of each end with
  Attendees, off until chosen, and a row carries how many
  (`TaskRow.attendees`, `NoteRow.attendees`: «2 attendees», the organiser
  one of them). No frame draws it yet (Typecase's task 181).

## v0.2.245 — 2026-10-15

### Changed

- **A task chooses its time zone** (Р6 of
  docs/plan/scans/kinds-parity.md), as an event does: its edit's When ends
  with Time zone (`tz`) — the zone of its start, else of its due date, «as
  sent»; the form's times are in it, no longer in the instance zone, and
  another zone rewrites the start and the due date at the clock the form
  names, with its VTIMEZONE. A new task with a time is written in the
  instance zone, as a new event is. No frame draws it yet (Typecase's task
  180).

## v0.2.244 — 2026-10-15

### Changed

- **A task and a note have attendees** (Р3 of
  docs/plan/scans/kinds-parity.md), as an event does — VTODO and VJOURNAL
  hold ATTENDEE too: their edits carry the field of attendees
  (`TaskEditPage.attendees`, `NoteDoc.attendees`; a form without its line
  changes no one), their cards and panels name the people, the organiser
  first (`TaskPage.attendees`, `NoteMeta.attendees`, `Details.attendees`),
  and `ATTENDEE` — a note's `ORGANIZER` too — is no longer kept as foreign.
  No frame draws them yet (Typecase's task 179).

## v0.2.243 — 2026-10-15

### Changed

- **The agenda and the notes offer Status and Priority columns** (Р13 of
  docs/plan/scans/kinds-parity.md), as a task's list does, now that an
  event has both and a note a status: the agenda's Columns ends with
  Status and Priority (`AgendaRow.status`, `.priority`), the notes' with
  Status (`NoteRow.status`; a journal has no priority), off until chosen.
  No frame draws them yet (Typecase's task 178).

## v0.2.242 — 2026-10-15

### Changed

- **An event has a priority** (Р7 of docs/plan/scans/kinds-parity.md), as
  a task does — RFC 5545 gives VEVENT one too. Its form chooses it after
  Status, from the task's list; its card shows «Priority» after Status,
  monospaced. `PRIORITY` is no longer foreign to an event. No frame draws
  either yet (Typecase's task 177).

## v0.2.241 — 2026-10-15

### Changed

- **A note has a status** (Р4 of docs/plan/scans/kinds-parity.md), as an
  event and a task do: a journal's STATUS (Draft, Final, Cancelled). Its
  edit chooses it after Tags, «No status» writing none; its page names it
  (`NoteMeta.status`); `STATUS` is no longer foreign to a note. No frame
  draws either yet (Typecase's task 176).

## v0.2.240 — 2026-10-15

### Changed

- **An event and a task choose their collection in their edit** (Р19 of
  docs/plan/scans/kinds-parity.md), as a note chooses its notebook:
  «Calendar» and «List» last in the head of the form, the same choice as
  Notebook (the collection's mark, a read-only one off with its reason).
  Saving into another collection writes the change and moves the record
  there (S-18); a form without the field leaves it where it is. No frame
  draws the field yet (Typecase's task 175).

## v0.2.239 — 2026-10-15

### Changed

- **A note exports as .ics too** (Р20 of docs/plan/scans/kinds-parity.md).
  An event and a task hand over one record whole; a note gave only its
  text, as Markdown, and its other properties had no way out. Its «⋯» now
  has «Export .ics» after Copy: the journal as the server holds it
  (`…/export?uid=…&as=ics`). The bar's Export Markdown is unchanged. No
  frame draws the item yet (Typecase's task 174).

## v0.2.238 — 2026-10-15

### Changed

- **A task repeats as an event does** (Р1 of
  docs/plan/scans/kinds-parity.md). The task's form ends with the event's
  «Repeats» rubric, the rule edited whole (S-11). The series counts from
  the start, or from the due date when there is none — the way the server
  already read a repeating task and moved it on when it was done. A task
  with neither date gets no rule: Repeats comes back with «Set a start or a
  due date: the task repeats from it.», and the hint under Frequency says
  so before. A save that Done moves on writes the rule itself, and the
  form's count is the one left. No frame draws the rubric yet (Typecase's
  task 173).

## v0.2.237 — 2026-10-15

### Changed

- **Every kind edits its link** (Р8 of docs/plan/scans/kinds-parity.md).
  An event's and a task's card showed `URL` as «Link» and no form could
  change it; a note neither showed nor edited it and named it among the
  properties kept as sent. Now the forms of all three have «Link» before
  «Related to» (an event's and a task's in «Other», a note's after Tags);
  a change writes that one line. A note's column shows the link
  (`NoteMeta.link`), and `URL` is no longer foreign to a note. No frame
  draws either yet (Typecase's task 172).

## v0.2.236 — 2026-10-14

### Changed

- **A contact's form edits its websites and its anniversary** (Р26 of
  docs/plan/scans/kinds-parity.md), which the card showed and nothing could
  change: «Websites» is a rubric of rows after Addresses, each labelled
  home, work or other (`TYPE`), with «Add website»; «Anniversary» is a day
  typed as the birthday is, `DD.MM.YYYY` or `DD.MM`, written in the form
  `ANNIVERSARY` came in (never Apple's year marker — that is the
  birthday's). Empty, both fold under «Show N empty fields». The card shows
  the anniversary as it shows the birthday: a date, monospaced. No frame
  draws either yet (Typecase's task 171).

## v0.2.235 — 2026-10-14

### Changed

- **A list of tasks offers Where** (`TaskRow.where`; Р10 of
  docs/plan/scans/kinds-parity.md), as the agenda does: the task's
  location, off until chosen. No frame draws it yet (Typecase's task 170).

## v0.2.234 — 2026-10-14

### Changed

- **An event's form says where the event is and when it was read**
  (`EventEditPage.source`; Р18 of docs/plan/scans/kinds-parity.md), as a
  task's form does. A new event has none. No frame draws it yet
  (Typecase's task 169).

## v0.2.233 — 2026-10-14

### Changed

- **A task's form shows what Carrel does not know of it, as it came**
  (`TaskEditPage.foreign`; Р17 of docs/plan/scans/kinds-parity.md), as an
  event's form does — a task showed it nowhere. No frame draws it yet
  (Typecase's task 169).

## v0.2.232 — 2026-10-14

### Changed

- **Send from an event, a task and a note** (Р23 of
  docs/plan/scans/kinds-parity.md): the record goes into a letter from its
  own page, as a contact's card does. The frames do not draw it yet
  (Typecase's task 168).

## v0.2.231 — 2026-10-14

### Changed

- **New task from a note and from a contact's card** (Р15 of
  docs/plan/scans/kinds-parity.md): the quick sheet of a task related to the
  record, as an event and a person's timeline have it. The frames do not
  draw it yet (Typecase's task 168).

## v0.2.230 — 2026-10-14

### Changed

- **Kinds are equal — the first gaps closed** (the owner's rule of
  12.10.2026; docs/plan/scans/kinds-parity.md): tasks sort by «Title» and
  their quick sheet has «Remind», as the event's has; notes sort by
  «Recently changed», one list; duplicate tasks are asked which place
  stays, as events are.

### Not done

- The frames draw none of these yet (Typecase's task 167).

## v0.2.229 — 2026-10-14

### Changed

- **A birthday without a year on a vCard 3.0 card is written as Apple writes
  it** — `BDAY;X-APPLE-OMIT-YEAR=1604:1604-04-12`, which Baikal and Nextcloud
  read — where the card does not hold another client's `--0412` already:
  vCard 3.0 has no date without a year (Б30 of the audit of 11.10.2026).
- **A display name that is the organisation typed beside it is not cut into
  a given and a family name** (Б30): it is an organisation's card.

## v0.2.228 — 2026-10-14

### Changed

- **The folder of attachments goes with its folder** (Б42 of the audit of
  11.10.2026). Renamed or moved in Files — itself or a folder above it —, it
  stays the folder of attachments: the setting names its new place, and the
  next large attachment no longer makes a folder at the old path.
- **The sheet of a deletion says what is lost**: a folder that holds the
  folder of attachments, and how many published links stop opening.

### Not done

- Links to files and folders do not follow a move in Files.
- The words are the developer's.

## v0.2.227 — 2026-10-14

### Changed

- **A published link goes where Carrel takes what it opens** (Б41 of the
  audit of 11.10.2026). A record moved — as a copy or by MOVE, under its UID
  or a new one —, merged into another, a collection moved: the link opens
  it in its new place, on another connection with that connection's own
  credentials. Where Carrel cannot follow — the record deleted, a UID the
  server did not say, the collection removed, a guest who found nothing —
  the owner's list says the link is dead (`PublishedRow.gone`), and the
  guest reads «No longer there», not «Try again later».
- The test stand answers a listing of a calendar or an address book that is
  not there with 404, as Radicale and Nextcloud do.

### Not done

- The words of the dead row are the developer's.

## v0.2.226 — 2026-10-14

### Changed

- **A linked group that came apart says so** (Б16 of the audit of
  11.10.2026). When the other copies of a linked record are gone —
  deleted, moved, their collection removed —, the card of the one left says
  «No longer linked» and where the other was, once; the screen of
  duplicates says how many groups came apart. A collection that could not
  be read loses no members, as before.

### Not done

- The words are the developer's; the timeline of a person says nothing of
  it.

## v0.2.225 — 2026-10-14

### Changed

- **A merge, and a move that changes a UID, mend the links of cards too**
  (Б33 of the audit of 11.10.2026). RELATED of a person and MEMBER of a
  group in the ticked books that named the card that went name the one that
  stays — written as before, `urn:uuid:` or the bare UID, with their
  parameters; a group that named both names the kept one once. Events,
  tasks and notes were mended already.

## v0.2.224 — 2026-10-14

### Changed

- **An event made on the quick sheet with a time and no end lasts an hour**
  (Б31 of the audit of 11.10.2026), as a new event of the full form does. It
  used to be written lasting no time. An end typed stays; a whole day stays
  a day.

## v0.2.223 — 2026-10-14

### Changed

- **«Replace» of a folder moved or copied onto a taken name merges** (А11 of
  the audit of 11.10.2026; the owner's decision of 12.10.2026). Nothing that
  stood there is deleted any more: each member goes in under a free name at
  once, and each name taken there is a question of its own in the queue — a
  folder among them merges the same way when it is answered «Replace». A
  move takes away the folder it came from once nothing is left in it.

### Not done

- A source folder emptied later, by the answers, stays where it was.
- «merged with the folder there» is the developer's wording.

## v0.2.222 — 2026-10-14

### Changed

- **An edit that gives a connection another server or login asks what
  becomes of its published links** (Б40 of the audit of 11.10.2026; the
  owner's decision of 12.10.2026). The first Save saves nothing and brings
  the dialog back with «Published links»: keep them — they open on what the
  connection points at now — or revoke them, neither chosen. Another
  password, label or fingerprint moves the links along without a word.

### Not done

- No frame draws the question (Typecase's task 164); the words are the
  developer's.

## v0.2.221 — 2026-10-14

### Changed

- **A published calendar, event, task list and task show the visitor the
  time and only what is ticked** (Б39 of the audit of 11.10.2026; the owner's
  decision of 12.10.2026). The publish sheet of these kinds has «What the
  visitor sees»: Time, always; Title, ticked; Location, Description,
  Attendees, Attachments, Tags, not. The page, the rows of a calendar and of
  a list, and the `.ics` are made of a record built of what is allowed: a
  reminder, the class of access, a property Carrel does not know never go.
  Without the title the visitor reads «Busy». A plain POST gives the time
  and the title. «Show event descriptions» became «Description».
- A link made before is what it was: everything, the descriptions taken out
  if they were.

### Not done

- No frame draws the ticks (Typecase's task 163); «Busy» and «Always shown.»
  are the developer's words.
- The rows of a published calendar still carry «linked ×2».

## v0.2.220 — 2026-10-14

Typecase `f136966` (Typecase's note 63), client `v0.4.19`. Nothing changed in
the running server: a row of the agenda already carries every badge
(`AgendaRow.badges`); C-11 now says the row draws all that fit, not the
first.

## v0.2.219 — 2026-10-14

Typecase `14fa0c2` (Typecase's note 62), client `v0.4.18`.

### Changed

- **The days of a series a server did not keep are said as days** (Typecase's
  Р-60, the owner's answer of 14.10.2026; `states-36`). The report of an
  edit names each by its start in the series and what became of it — «the
  meeting of 24 August, which was skipped, is back», «the meeting of 31
  August, which was moved to 1 September, 15:00, is back on its day at
  12:00» — in a message of its own (`LossReport.days`); EXDATE and
  RECURRENCE-ID are no longer among its properties.
- **The report's first sentence is its own field** (`LossReport.title`), bold
  as the frames draw it; `note` is the words before the properties.

### Not done

- The reports of a move, a merge, an import and an address rewritten still
  name the exceptions EXDATE and RECURRENCE-ID.
- «Nor did it keep …» — properties and days lost at once — is the
  developer's wording; no frame draws it.

## v0.2.218 — 2026-10-14

Typecase `629c31e` (Typecase's notes 50–61), client `v0.4.17`.

### Changed

- **A reminder of one's own** (Typecase's task 156, `s-agenda-19`). A
  reminder's list has «Custom…» after the steps; each row carries «How
  long» and «Unit» — a record of a whole day also «At» —, hidden while a
  step is chosen (`FormRow.hidden`, `Option.shows`). The numbers are read
  only while «Custom…» is chosen; chosen without them, the form comes back
  with them open and a reason, and nothing is written. A reminder another
  client wrote at an offset none of the steps is comes in as «Custom…» with
  its numbers, not as one more item of the list. A whole day's card says
  «1 day before, 09:00».
- **«Related to» of an event stands in «Other»**, before the attachments,
  as the task's (Typecase's task 160, `s-agenda-3`, `-15`).
- **The card of a linked record opened from its collection is that copy**
  (Typecase's Р-58, note 57): its own values, and the badge «linked» on a
  field another copy holds otherwise (`Field.badge`; a note's tags —
  `NoteMeta.tagsBadge`). It no longer takes the other copies' title, place
  and fields. The rows of the copies say «where», «priority», «tags differ».
  A note's edit no longer shows the same note in other notebooks.

### Not done

- The sheets «Which copy to edit?» and «Which copy to mark done?», and the
  card of the entity at an address of its own (`s-contacts-31`…`-33`,
  `s-tasks-16`): until then nothing shows the values a person chose.
- The live layer of notes 58–61, the sheets of notes 53 and 56, the
  agenda's columns (note 55), the import's question (note 52), the files of
  notes 50, 60 and 61, «Appearance» keeping its ticks — docs/plan/server.md,
  «Переход на Typecase `629c31e`».
- The reasons of «Custom…» are the developer's words.

## v0.2.217 — 2026-10-12

### Changed

- **A move that takes a meeting off a server which schedules says its
  attendees may hear of a cancellation** (А12 of the audit of 11.10.2026;
  the owner's decision of 12.10.2026: say this, and take on nothing more).
  The sheet of a record's move says it when the event has attendees and
  goes to another account; the sheet of a calendar's move, when the
  calendar leaves such a server. `MoveSheet.cancels`,
  `CollectionMoveSheet.cancels`. «May»: Carrel knows that the server
  announced scheduling (RFC 6638), not whether a letter goes out.
- A copy, an event without attendees, a server that does not schedule or
  did not say, and a move inside one account say nothing. Nothing is said
  of invitations anywhere: where the record goes is the person's choice.

### Not done

- A record's deletion has no sheet to say it in yet (А8).
- The words are the developer's, not the owner's; no frame draws the line.

## v0.2.216 — 2026-10-12

### Changed

- **A task marked done is 100 per cent done** (Б17 of the audit of
  11.10.2026; the owner's decision of 12.10.2026). The mark used to leave
  the percent as it was. With the mark taken off nothing is brought back,
  because what the task was before was kept nowhere: Needs action, no
  percent, no date of completion. A percent given in the same form stands.
- **A contact's display name follows its given and family names** while the
  person has not made it their own (Б30; the owner's decision of
  12.10.2026). «Anna Kovaleva» of Anna and Kovaleva, with the family name
  changed to Belova, becomes «Anna Belova» — in the order it stood in;
  every list shows the display name, and it used to stay as it was. A
  display name written by hand («Anya (the printer's)») is left alone.

### Not done

- Of Б30: «Acme Trading Ltd» typed as a display name is still cut into a
  given and a family name, and a birthday without a year on a vCard 3.0
  card is still written in a form 3.0 does not have.

## v0.2.215 — 2026-10-12

A relation made in Carrel is «see also», not subordination (Б29 of the
audit of 11.10.2026; the owner's decision of 12.10.2026).

### Changed

- **A relation added in «Related to» is written with its type**,
  `RELTYPE=SIBLING` — in the form of a task, an event and a note, and by
  «Write a note», «New event» and «New task» of a person's timeline. It
  used to be written bare, and a bare `RELATED-TO` is a parent by the
  standard: other programs drew the task a subtask of what it only named.
- **So a task marked done closes real subtasks only** (`v0.2.212`): tasks
  whose relation is a parent — bare, or `RELTYPE=PARENT`, as other programs
  write a subtask. A task Carrel related to it is left alone.
- A relation a record already had keeps the type it came with.

### To know

- Relations between tasks made in Carrel before this version are bare:
  they read as subtasks, here and elsewhere, until the relation is taken
  off and added again.

## v0.2.214 — 2026-10-12

Typecase `22f594a` (Typecase's note 49), client `v0.4.16`. The copy of the
mockup is `22f594a` byte for byte (429 frames). Nothing in the running
server changed: the rail of the settings already counted davloom's devices
and the backup jobs, as the frames now draw them.

### Not done

- The selection bar of the files with nothing ticked: the server sends its
  operations always, and its words still carry a count — the note asks for
  words without one ([server.md](../../docs/plan/server.md), «Переход на
  Typecase `22f594a`»).

## v0.2.213 — 2026-10-12

An event relates to other records as a task and a note do, and «New event»
on a person's timeline invites nobody (А10 of the audit of 11.10.2026; the
owner's decisions of 12.10.2026).

### Changed

- **An event has «Related to»** — the field of a task's and a note's form,
  after Description in the event's. The owner's rule: what one kind of
  record has, every kind has.
- **«New event» on a person's timeline relates the event to the person.**
  It used to carry the person as an attendee in a hidden field of the quick
  sheet, and you became the organiser: on a server that sends invitations
  itself, one went out. Now the sheet carries the person's card as the
  event's relation, like «Write a note» and «New task»; the event stands in
  the timeline by it. Whom to invite is said in the event's form. The
  button no longer needs the card to have an address.

### For developers

- `timeline.event` leads to `/app/calendar/quick?related=<uid>`; the
  address parameter `attendee` of a quick sheet still works for who uses it.
- The event's frames do not draw the field: Typecase's task 160.

## v0.2.212 — 2026-10-12

### Changed

- **A task marked done closes its open subtasks with it** (А9 of the audit
  of 11.10.2026; the owner's decision of 12.10.2026: without a question).
  They used to stay open under a done task, and nothing said so. Every
  level under the task is closed, each subtask as «Mark done» would close
  it. A subtask that repeats goes on to its next time and stays open, as a
  repeating task marked done does, and what is under it is left alone.
  Taking the mark off the parent opens nothing again.
- Nothing is said when all of them closed. A subtask the server would not
  take is named in a notice on the task: «The task is done, 1 subtask still
  open».

### For developers

- `closeSubtasks`, `subtaskParents`: a subtask is a task of the same list
  whose `RELATED-TO` names the parent with no `RELTYPE` or with `PARENT`.
  One more read of the list after the parent's write.
- The words of the two notices are the developer's, not the owner's.

### Not done

- Subtasks in another list than the parent's are not looked for.
- A parent closed from the conflict screen closes no subtask.

## v0.2.211 — 2026-10-12

An import asks what to do with the records the collection already holds
(А7 of the audit of 11.10.2026; the owner's decision of 12.10.2026).

### Changed

- **Records of a file that are already in the collection are skipped unless
  you say otherwise.** An import used to create a second record under a
  fresh UID for each of them — a file exported from a calendar and imported
  back doubled it. The preview now says how many there are («38 of 40
  records of this file are already in Work.») and offers three answers:
  «Skip them», which is what happens without one; «Replace them with the
  file's version»; «Import as copies», which is what every import did
  before. A record is «already there» by its UID.
- **«Replace» writes over a record** — the first thing an import overwrites.
  It goes to the record's own address with the version read just before:
  one that changed in between is refused and named in the report. The
  report counts replaced records apart (`ImportReport.replaced`) and no
  longer says «Nothing there was overwritten» when there were any.
- The button counts what the press writes («Import 2 contacts» of a file of
  40 when 38 are skipped) and is off, with why, when that is nothing.

### For developers

- `ImportPage.existing` (`ImportExisting`: `note`, `options` — GET links to
  the same preview with `existing=skip|replace|copies`);
  `ImportReport.replaced`. The answer lives in the draft of the session.
- Tests that pinned «an import always creates» answer «Import as copies»
  first (`asCopies`).
- The words of the question, of its answers and of the rows are the
  developer's, not the owner's. No frame draws the question: Typecase's
  task 159.

### Not done

- A restore from a backup copy is not an import of this wizard and asks
  nothing.
- Records with another UID and the same content are still created: the
  duplicates screen finds them afterwards.

## v0.2.210 — 2026-10-12

A merge of duplicates asks whose time to keep (А6 of the audit of
11.10.2026; the owner's decision of 12.10.2026: the person decides what to
take from where).

### Changed

- **The merge screen of two events or tasks whose time differs has a row
  «When»** (`choice.WHEN`). The time, the length, the due date, the
  repetition and the days left out used to come from the kept record only,
  and the screen said nothing of the other's. Now each member's time is an
  option, said in words and with where it comes from; without an answer the
  kept record's stays, as before.
- **The chosen time is taken whole**: the start with its zone and the
  zone's definition, the end or the length, the rule, the days added and
  left out, and the occurrences rewritten by hand — they become the kept
  record's. The kept record's own rewritten occurrences go with the time it
  gives up; «What changes» says how many.

### For developers

- `dupes.When`, `Plan.Time`, `Plan.Dropped`; `cal.TimeProps`, `cal.TimeKey`,
  `cal.TakeTime` — a document edit, like `cal.SetAlarms`.
- A time written differently is another time: `…T100000Z` and the same
  moment with a `TZID` are two options.
- The words of the row and of the two lines of «What changes» are the
  developer's, not the owner's.

### Not done

- The duplicates screen's own questions of a linked group («Keep these
  values») do not ask about time.
- No frame draws the row: the merge screen of the client is not built.

## v0.2.209 — 2026-10-12

### Changed

- **The tag a list is filtered by stands first in a row's tags** (Typecase's
  note 44; contacts, agenda, tasks, notes). The cell «Tags» shows the first
  tag and a count of the rest, so a filtered row now says why it is in the
  list. The other tags keep the record's order, and a list without a filter
  is as before.

## v0.2.208 — 2026-10-12

A photo changed on a card no longer loses what was typed into its form (Б9
of the audit of 11.10.2026). Client `v0.4.14`.

### Fixed

- **Add, Replace, Crop and Remove of a card's edit form send the form.**
  They posted on their own, the frame screen came back to the card as the
  server holds it, and a name or a phone typed before pressing them was
  gone without a word. Now the buttons are the form's (`Action.Submits`),
  as on a new card: the typing waits with the photo and is back in the
  form after Save photo, after Cancel and after Remove. The step writes the
  photo only; Save of the form writes the typing.
- **A file too large sent with the form** is still called «The photo is
  too large», not «could not be read».

### Changed

- `PhotoBlock.Actions` of an existing card carry `submits: true`. A client
  that gave each of them a form of its own must send the edit form to the
  action's address instead (with the file, for Add and Replace).
- The form a photo step comes back to names the version it was typed at.
  A photo written over that same version moves it; a card changed elsewhere
  in between does not, so Save of the form then meets the conflict screen.

### Not done

- The typing is shown once: the form a photo step returns to. A loss report
  after Save photo shows the card as the server holds it, and the typing
  comes back with the next opening of the form.
- A new form's photo sent by a script and too large for the body's cap is
  probably still not called too large (read in the code, not run): the new
  form's steps parse the body the old way.

## v0.2.207 — 2026-10-12

Typecase `f30d992` (Typecase's note 48), client `v0.4.13`. The copy of the
mockup is `f30d992` byte for byte (429 frames).

### Changed

- **«Appearance» and «Attachments» have Discard beside Save** (the owner's
  decision of 12.10.2026, Typecase's Р-57; `appearance-1`, `attachments-1`).
  `AppearancePage.Account.Actions` and `AttachmentsPage.Form.Actions` start
  with `appearance.discard` and `attachments.discard` — a GET of the page
  itself, which drops what was typed and not saved — and their Save is the
  primary button now, as the frames draw it. A client that drew the first
  action of these forms as Save must find it by its ID.

### Not done

What else the note asks of the server is listed in
[server.md](../../docs/plan/server.md), «Переход на Typecase `f30d992`»: the
administrator's settings as one form with Discard, the tree of the move
sheet (a node that cannot be the target is off with its reason, Move does
not depend on the chosen node), the empty list of published links.

## v0.2.206 — 2026-10-12

An edit written from the conflict screen is the edit the person made (Б8 of
the audit of 11.10.2026).

### Fixed

- **«Use only my edit» writes the attendees as the form had them.** An edit
  refused because the record changed elsewhere kept the rows of each field
  and not what the form sends beside them. Written from the conflict
  screen, it lost the attendee typed into the line, and did not take away
  the attendees unticked — the field read as one that only adds.
- **A task's «All day» is the person's there too.** It was taken from the
  server's version, so a task given a whole day came out with its time.

### For developers

- `changedValues` carries the attendees' line and a task's box;
  `postRecordConflict` puts them back beside their fields. A line typed
  with the list unchanged makes the attendees a row of the conflict screen.
- Not done, Б9: a photo replaced or cropped on an existing card is a write
  of its own, and what was typed into the form before it is lost.

## v0.2.205 — 2026-10-12

Two new forms keep their own files (Б7 of the audit of 11.10.2026).

### Fixed

- **The files attached to a new, unsaved event, task or note are not thrown
  away by another new form.** A session had one place for them, and the
  first Attach of a second new form took it: the first form then saved
  without its files, and nothing said so. Each new form now has its own;
  a session holds four at once, and past that the oldest goes.

### For developers

- `attachDrafts`, `attachDraftsKept` in `internal/web/attach_new.go`; the
  slot `DraftNewAttach` holds a list.
- Not done: a form whose files went — the fifth new form, or the session
  ended — still saves without them and without a word; a session may now
  hold four times the files it held (each draft stays within the limit of
  its collection).

## v0.2.204 — 2026-10-12

The question about a replaced address is not lost, and the conflict screen
says when it cannot tell whose change a field is (Б44 and Б43 of the audit
of 11.10.2026).

### Fixed

- **The question about a replaced address survives a loss report.** When
  the save of a card came back with «the server did not keep…», the
  question — rewrite the old address in the events and tasks that name it?
  — was dropped and never asked. Now the report comes with a notice, «An
  address on this card was replaced», and its action opens the question.
- **The same question is asked after the conflict screen.** A card that
  changed elsewhere, written from that screen with a new address, went
  back to the card without the question.
- **The conflict screen says when it lists more than the person's
  changes.** A refused edit is measured against the version the form was
  opened with; when Carrel no longer holds it, every field that differs
  from the server's is listed — also what only the other client changed,
  and «Use only my edit» would put the old value back. A notice now says
  so.

### For developers

- `finishCardWrite`; `editDraft.Unsure`. The action of the new notice is
  `contact.rewrite`.
- S-15.4 said «the report comes first, and the question is then not asked»
  since `v0.2.64`; the rule is changed there, and the owner has not seen
  the change.
- An address only taken away still asks nothing (the decision of
  24.09.2026).
- The words of both notices are the developer's, not the owner's.

## v0.2.203 — 2026-10-12

An import is read back, and a record put where it was not made keeps its
time of change (Б35 and Б34 of the audit of 11.10.2026; Б32 looked into).

### Fixed

- **An import says what the server did not keep.** It was the one write
  nobody read back: a server that keeps less than it is sent answered 201,
  and the row said «Created» over a record that had lost a part of itself.
  Now the row says «Created. The server did not keep LOCATION», and the
  note of the report counts such records («1 record not kept whole by the
  server»). No more requests: the read the import already made to learn
  the identities is the one compared.
- **A record moved to another server, copied or imported keeps when it was
  changed.** It was written with «changed now» — `LAST-MODIFIED` of an
  event, a task, a note, `REV` of a card —, so every list sorted by Changed
  put it first and the real day was gone.

### For developers

- `cal.Provider.Place`, `card.Provider.Place`, `collectionCtx.place`;
  `destRecords`; `importLanded` answers how many records lost something.
- Б32 — a note whose edit changed only its Notebook moves without the
  form's version being compared — loses nothing: what moves is the note as
  the server holds it, read for the move
  (`TestANoteMovedByItsNotebookTakesWhatWasWrittenMeanwhile`). No change.
- A restore from a backup is an import and so is read back too; it was not
  tested apart.

## v0.2.202 — 2026-10-12

A wrong number on the duplicates screen is said (Б45 of the audit of
11.10.2026).

### Fixed

- **A threshold or a weight that is not a number in range is not dropped
  without a word.** The screen came back showing the old value as if it
  were the one typed. Now it comes back with a notice — «The threshold was
  not changed», «1 weight not changed» — and says the range; the weights
  that were right are saved.

### Changed — contract

- `POST /app/duplicates/decide` with `do=threshold` or `do=weights` and a
  number out of range answers with the page and a notice instead of a
  redirect.

### For developers

- `duplicatesRefused` in `internal/web/duplicates.go`. The words of the
  notices are the developer's, not the owner's.

## v0.2.201 — 2026-10-12

The matrix «every field × a hard record» for tasks, notes and events of
other clients, and the one loss it found (Б11 of the audit of 11.10.2026).

### Fixed

- **A tag with a comma of its own stays one tag.** `CATEGORIES:Smith\, John`
  is shown in the tag field as «Smith, John», and any edit of the tags
  wrote it back as two tags, «Smith» and «John». Now the items that stand
  side by side as a tag of the record does are that tag. So for events,
  tasks and notes; a tag with a comma still cannot be typed anew in the
  field, and the tags of a contact are as they were.

### For developers

- `keptTags` in `internal/web/record_edit.go`.
- `field_matrix_test.go`, `field_matrix_note_test.go`: one field changed in
  a task, an event and a note another client wrote, and the lines that
  change. Nothing else was found: the place, the times, the status, the
  priority, the percent, a relation taken away (the ones kept keep
  `RELTYPE`), the title and the text of a note with two `DESCRIPTION`
  lines change their own lines only.
- Not in the matrix: the contact beyond the seed of `v0.2.188` (the test
  server respells every line of a card it hands out, so «only this line
  changed» cannot be told there), attendees, the fields of a new record.

## v0.2.200 — 2026-10-12

A backup and a letter no longer leave out, without a word, a record Carrel
could not read (found with Б3, `v0.2.197`).

### Fixed

- **A backup archive holds the records Carrel could not read**, as the
  server holds them. They were left out, and the run still called the
  calendar whole. A record that cannot be put in — the server will not
  give it — makes the collection «not whole» in the manifest and on the
  screen, with the names of the records; such a run rotates nothing.
- **A calendar attached to a letter** holds them the same way; when one
  cannot be put in, the letter does not go and says which records.

### For developers

- `unreadKept` in `internal/web/export.go`; the count of a calendar in a
  backup's manifest includes the records that went in raw.
- Not covered by a test: the record that cannot be put in (the test server
  cannot refuse one GET and still take the archive), and the letter.

## v0.2.199 — 2026-10-12

The owner's decisions of 12.10.2026 on the table of S-11 «an edit of the
time → what is written» (Б23, Б25, Б26, Б28 of the audit of 11.10.2026).

### Changed — behaviour

- **The start of a series is always its first occurrence; the rest go by
  the rule.** A series on Fridays given a start on a Tuesday is written as
  typed, as before — and is now shown so: that Tuesday, then the Fridays.
  Carrel left the Tuesday out, in the agenda and in the «next times» of the
  card. The days ticked are not moved by the start.
- **Another frequency chosen in the simple fields drops what those fields
  do not show.** «Yearly» made «Monthly» kept `BYMONTH` and so still came
  once a year. Now every N, the end of the series and the start of the
  week stay, the ticked days stay for «Weekly» only, and `BYMONTH`,
  `BYMONTHDAY`, `BYSETPOS` and numbered days go. With the frequency
  untouched they stay, as before.
- **An event that becomes a whole day takes its reminders to the day
  before.** A reminder closer than a day — «15 minutes before» would ring
  at 23:45 the night before — becomes «1 day before»; one a day away or
  more, one at a set moment, and what the reminder does, stay. The other
  way nothing is touched.
- **The end of an event keeps its own zone when another zone is chosen.**
  A flight from Berlin landing in Tokyo, moved to Moscow time, keeps its
  length and its end in Tokyo; the end was rewritten into the start's zone.

### For developers

- `cal.seriesOf` adds the start to the set of a rule; `repeatRule` rebuilds
  the rule on a change of frequency; `buildChange` rewrites the reminder
  rows and the ticked days before they are read, so the form afterwards
  shows what was written (S-7.3).
- Which records fall in a week of the agenda is the server's answer: a
  server that does not count a start outside its rule will not hand over a
  week that holds only that start (the test server does not).
- Not done: a reminder of the person's own offset — decided, waits for a
  frame (Typecase task 156); a task that becomes a whole day keeps its
  reminders; a series with `COUNT` and a start outside its rule shows the
  start on top of the count.

## v0.2.198 — 2026-10-12

A series goes with its start, and so does what is written by the start's
clock (Б21, Б22, Б24, Б27, parts of Б25 and Б28 of the audit of 11.10.2026,
`docs/plan/scans/silent-choices.md`); S-11 has the table «an edit of the
time → what is written».

### Fixed

- **A series whose `UNTIL` is the start of its last occurrence can be moved
  later again.** Most clients write `UNTIL` so. Moved an hour later — or to
  another zone — the series would have lost its last day, the check of
  `v0.2.188` saw it, and the answer was «Carrel could not save this edit»:
  such a series could not be moved at all. Now the last day stays the day
  the form shows: `UNTIL` is left alone while the rule still reads as that
  day, and is otherwise written as the form writes it, the end of that day.
- **A whole day given a time can be saved in an instance that is not in
  UTC, and is written in the instance's zone.** It was written as an
  instant in UTC — a series would keep that instant through the change of
  the clocks — and outside UTC the same check refused the save. It is now
  written as a new event is: `TZID` and its `VTIMEZONE`.
- **`UNTIL` is a date exactly when the start is one** (RFC 5545 §3.3.10):
  «All day» ticked or unticked rewrites it with the start.
- **The days added to a series (`RDATE`) go with it** as the days left out
  do: another time, another zone, a whole day.
- **A reminder whose time is changed stays the reminder it was** — its
  action, sound, text and recipients. It came back as a plain display one.

### Changed — behaviour

- **A start past the last day of the series is an error of the form**
  («The series would end before it starts…» under Repeats) and writes
  nothing. Before, the series was saved with its start after its `UNTIL`.
- **«Until» is a day of the form's zone** — the event's own, as the form's
  start and end are; it was read and written in the instance's.

### For developers

- `untilKept`, `rowAt` in `internal/web/record_edit.go`; `cal.Alarm.At`;
  `cal.RetimeExceptions` moves `RDATE`; `editAlso` names `RDATE` and
  `RRULE` among what a start and a zone take with them (S-7.3).
- Left as they were, for the owner to decide (S-11 lists them): `BYDAY`
  of a series moved to another weekday, the parts of a rule the simple
  fields do not show when the frequency changes, the end's own zone when
  the zone changes, relative reminders when «All day» changes.
- The words of the new error are the developer's, not the owner's.

## v0.2.197 — 2026-10-12

An export no longer leaves out, without a word, a record Carrel could not
read (Б3 of the audit of 11.10.2026, `docs/plan/scans/silent-choices.md`).

### Fixed

- **A record the server handed over and Carrel could not read goes into the
  `.ics` as the server holds it.** It was left out of the file and nothing
  said so — also in «Download .ics first» before a calendar is deleted. Now
  the record is asked for again, raw, and its components are copied into
  the file untouched.
- **What cannot be put into the file is said before any file is handed
  over.** When the server will not give the record, when nothing in it is a
  component of a calendar, or when the file is the Markdown of notes, the
  export answers with the collection's screen: «The file would lack 2
  records», the records named, and «Export without them».

### Changed — contract

- `/app/{kind}/{account}/{col}/export` may answer with the screen of the
  collection and the action `collection.export.without` (the same address
  with `&without=1`) instead of a file.

### For developers

- `cal.Provider.Raw`; `exportUnread`, `exportLacks`, `allRecordsAndUnread`
  in `internal/web/export.go`; `writeCalendar` and `writeExport` take the
  raw documents.
- Not changed, and they still leave such records out without a word: the
  file of a public link (on purpose: nothing checked what of the record
  the link may show), the attachment of a letter, the backup archive.
- An address book has no unreadable records at this level: the card
  provider does not report them.
- The words of the notice are the developer's, not the owner's.

## v0.2.196 — 2026-10-12

Attach takes every file its form carries (Б6 of the audit of 11.10.2026,
`docs/plan/scans/silent-choices.md`).

### Fixed

- **Attach no longer drops the files after the first.** A form that carried
  several files had the first written into the record and the rest dropped
  without a word. Now every file goes in, each by the rule of S-20 — inside
  the record, or to the folder for attachments with a link — and all of them
  by one write of the record. A file that cannot be attached stops them all:
  the record is not written, the files already put into the folder for it
  are taken away again, and the notice names the file when it is not the
  first («… could not be attached, so none of the files was»).
- **So for a new event, task or note:** the files of one Attach wait for
  Save together, or none is kept when together they would not fit inside
  the record.

### For consumers

The page's own Attach sends one file — its field lets one be chosen — so a
browser sees no change. A client that sends several files in one form gets
them all attached or a notice; before it got the first and a redirect.

### For developers

- `nextFile`, `takeBack`, `settled` in `internal/web/attach.go`;
  `readNewAttachUpload` returns every file. Fields between the files are
  kept (`uploadBody.keep`): the sizes of the files after them.
- The words of the notice are the developer's, not the owner's.

## v0.2.195 — 2026-10-12

The counts of the rail while a poll runs (Typecase's note 44; the owner,
12.10.2026: the frames are right, and the row of the combined view has a
dash while the poll runs). The last of what the transition to `43ca489`
left the server.

### Changed — behaviour

- **While the poll of a combined view runs, a collection that has not
  answered it has a dash, and so has the row of the combined view** —
  whatever the session knew of them before. Until now the dash stood only
  where the number was not known at all: a book read an hour ago kept its
  old number while it was being asked again, and the combined row its old
  sum. **The sections above keep their numbers** (`states-8`, `-13`).

### For developers

- The partial fixtures are taken in a session that has seen its
  collections before — an hour ago, so each is asked again —:
  `app-contacts/partial` (the section says 7 with «Команда» held),
  `app-agenda-all/partial` («Дежурства» answers and «Проекты», on the same
  stand, is held: `davtest.Server.StallAt` holds the requests of one path).
- U-5.5 rows 19 and 24 of the client are gone: `states-8` and `states-13`
  match.

## v0.2.194 — 2026-10-12

The words of a poll that is still running (Typecase Р-55, the owner's
decision of 11.10.2026; left over from the transition to `43ca489`).

### Changed — behaviour

- **A running poll speaks in the present**: «Polling · 2 of 3 answered · 6
  cards» over a combined list (`states-8`, `-13`, `-19`, `-26`) — how many
  of those asked have answered and what has come so far —, and «Polling 6
  sources…» before anybody has (`states-3`). A poll that ended says it as
  before: «Polled 2 of 3 sources · 6 cards». The bar of a person's timeline
  and of the search are as they were.

### For developers

- Fixtures `app-contacts/partial` and `app-agenda-all/partial` carry the new
  line. U-5.5 rows 19 and 24 of the client stay: what still differs in
  `states-8` and `states-13` is the counts of the sections while a poll runs
  (`docs/plan/server.md`).

## v0.2.193 — 2026-10-12

Files: «Replace» and a move between storages no longer destroy what nobody
looked at (Б13 and Б14 of the audit of 11.10.2026,
`docs/plan/scans/silent-choices.md`).

### Fixed

- **«Replace» replaces the version it was asked about.** The question about
  a taken name was answered for the file that stood there then — and the
  write went without a condition, however long the question had waited: a
  version somebody put there in between was overwritten unseen. Now what
  stands there is looked at again, an upload goes with `If-Match`, and
  another version stays: the row says what stood there changed while the
  question waited, and nothing was overwritten. A server that gives files
  no version is written as before.
- **A folder moved to another storage is deleted only when the copy is
  whole and the folder is what it was.** The original went as soon as the
  copy ended: a file put into the folder while it was being copied went
  with it, unseen. Now the folder is listed before the copy and after it,
  and the copy is listed too; a folder that changed, or a copy that lacks a
  file or holds one of another size, keeps the original and takes the copy
  back, and the row says which file. So for the move of a whole file
  storage too — there both stay, and the notice says why.
- **A folder copy that breaks off leaves nothing under the folder's name.**
  A folder is written beside its place, under a name of the transfer, and
  takes the place only whole; before, half a folder stood there under the
  real name.

### For developers

- `file.Provider.UploadReplace`, `.Tree`, `file.TreeDiffers`; `movedWhole`
  in `internal/web/files_ops.go`. A move of a folder costs two more
  listings of the original and one of the copy.
- Not covered by a test of its own: the move of a whole file storage
  (`collection_files.go`) — it asks the same `movedWhole`.

## v0.2.192 — 2026-10-12

Typecase `462ffb5` (Typecase's notes 45 and 46, and its answer to Carrel's
task 152), client `v0.4.11`.

### Changed — behaviour

- **The calendar shows events and nothing else** (Typecase Р-56, the
  owner's decision of 11.10.2026). The ticks «Events · Tasks due · Notes» of
  the agenda are gone, and with them the tasks and notes on its line: they
  live in their own sections. An address that still carries `with=` is the
  calendar all the same.
- **The agenda has Columns** (`s-agenda-18`): Time and Title always; Where,
  «Attendees and status» and Source shown until a person chooses; Tags and
  Changed to add. `POST /app/calendar/columns` and
  `/app/calendar/{account}/{col}/columns`, the cookie `carrel_cols_calendar`,
  as the other lists keep theirs. One calendar's menu offers no Source.
- **The end of an event follows its start** in the form (the owner's
  decision of 11.10.2026; Б19 of the audit): End date names Start date,
  End time names Start time and the date that goes with it over midnight
  (`FormRow.Follows`, `FormRow.Day`); the library's script moves the end
  before Save, so the length stays. Without a script nothing moves, and
  what is on the screen is what is written (S-7.3).

### Changed — contract

- `ListBar.Show` is gone. `AgendaRow` gains `Tags` and `Changed`, and its
  `Kind` is always `event`. `FormRow` gains `Follows` and `Day`.
  `ColumnItem.Fixed` may stand on more than one column: the agenda's two.

### For developers

- The set of the mockup: Вера Лебедева of «Архив» has two addresses
  (`vera@example.net`, `vera.l@work.example`), so the attendees' suggestion
  draws her twice and her name alone no longer says which address — typed
  by name she is refused with the reason. Fixtures are re-shot (37); the
  edit of the note reads the tasks first, so both its relations are drawn
  by their names (`s-notes-20`).
- The rows of the agenda carry every column in any choice; what the client
  draws of a choice other than the default waits for the library — the
  frames draw the menu, not a line of chosen columns (to-typecase task 154).

## v0.2.191 — 2026-10-11

Three places where data was lost or given out without a word (the audit of
11.10.2026, `docs/plan/scans/silent-choices.md`: Б2, Б36, Б37).

### Fixed

- **An import of a .vcf reads past a card it cannot read.** A card cut
  short — without its `END:VCARD` — swallowed the card after it: two people
  came out as one contact, with both names and both UIDs, and the preview
  showed a clean file. Each card is now read by itself; one that cannot be
  read is counted «with errors» and named in the preview, and the cards
  after it are the file's all the same.
- **A letter does not publish a revoked link again.** A link put into a
  letter and revoked, or run out, before the letter went was published anew
  by Send — thirty days, a new address. Now the letter does not go, and
  says so: remove the link or publish it again.
- **A letter does not turn some days of a calendar into the whole of it.**
  The events of a range of days that did not fit the letter went as a link,
  and a link opens the whole calendar. Now the letter does not go, and says
  so: remove it or attach fewer days.

### For developers

- `card.Split` answers a third value, the cards it could not read
  (`card.Broken`). A restore from a backup and a feed of cards read past a
  broken card too, and do not name it.

## v0.2.190 — 2026-10-11

A repeating task marked done goes on to its next time (the owner's decision
of 11.10.2026; А2 of the audit, `docs/plan/scans/silent-choices.md`).

### Changed — behaviour

- **Done on a task that repeats is this time done, not the series.** Carrel
  wrote `STATUS:COMPLETED` on the whole series: there was no next Monday,
  and nothing said so. Now the start and the due move to the next time of
  the series — days left out (`EXDATE`) passed over —, and the task stays
  open: Needs action, no percent, no day of completion. A rule with `COUNT`
  counts one fewer. A task on its last time is completed as any task is.
  So from the card, from the row of a list and from the form; choosing
  «Completed» in Status does the same.
- **The form and the card say it before the tick**: under Done, and as the
  hint of «Mark done» — «The task repeats. Done is this time: it moves on to
  19 Jan and stays open.», or that this is its last time.
- No trace is left in the record that the time before was done; ending a
  series early is taking its repetition away in another client — the task
  form has no Repeats.

### For developers

- `cal.NextTime`; `goOn` rewrites what the form asks for before the fields
  are walked, as `doneAndStatus` does, so the check of S-7.3 reads the
  result as the form.

## v0.2.189 — 2026-10-11

A write is read back whole wherever it is checked (Б1 of the audit of
11.10.2026, `docs/plan/scans/silent-choices.md`).

### Fixed

- **A move no longer deletes the original of a record the other server
  did not keep whole.** A record or a collection carried to another server
  was compared there by the properties of its main component alone: a
  reminder that server dropped, an occurrence changed by hand it no longer
  tied to its day, went unseen, and the original was deleted. Now the copy
  is read back with its reminders and the exceptions of its series, and a
  loss keeps the original: «Copied, and the original kept», naming `VALARM`,
  `EXDATE` or `RECURRENCE-ID`.
- **A lost reminder is reported after every write**, not only after a move:
  an edit, a merge of duplicates (the others are kept), an address rewritten
  in other records. A reminder is the same reminder when it does the same
  thing at the same time — a server that spells `-PT15M` as `-PT900S`, or
  adds its own marks to it, has lost nothing.

### For developers

- `writeLoss` (`internal/web/record_edit.go`) is the one comparison of a
  write with what was read back; nothing calls `model.CompareAfterPut` for
  that beside it. `cal.MissingAlarms`, `cal.AlarmLoss`.
- Not run against the live servers: a server that rewrites reminders will
  now say `VALARM` in its loss report.

## v0.2.188 — 2026-10-11

A save is the edit and nothing else (the owner's decision of 11.10.2026,
S-7.3). Found by the audit of what Carrel decides for a person when it
writes (`docs/plan/scans/silent-choices.md`).

### Changed — behaviour

- **A save that is not the edit is not sent.** Before a form's save goes to
  the server it is made in memory and looked at twice: the form read from
  the result is the form the person sent, and the lines of the record that
  changed belong to the fields that changed and to what those are said to
  take with them (the table of S-7.3). A save that fails either is not
  written: the form comes back as entered under «Carrel could not save this
  edit» — nothing was saved, the fault is Carrel's, the fields and
  properties are named, the lines are in the details. The server log names
  the fields and properties, never what they hold. So for an edit, a new
  record and a record combined from a conflict.
- **A field left as the form showed it writes nothing**, however its value
  is spelt on the server. A description that ends in a line end, a phone
  with a space after it, were rewritten trimmed by a save that changed
  something else.
- **A field that shows the first value of a property changes that value
  alone.** A second TITLE, ORG, NOTE, DESCRIPTION or RRULE another client
  wrote went with the first when it was edited; now it stays. Emptied, the
  field takes the value it showed, and shows the next one.
- **A phone, address or e-mail row keeps its group and its other labels.**
  Changing the number of `item1.TEL` untied it from its `item1.X-ABLabel`;
  choosing «home» for `TYPE=WORK,CELL` wrote `home` and dropped `CELL`. Now
  the value keeps its group and parameters, and the label chosen replaces
  the label shown, where it stood: `home,CELL`.
- **An attendee line that is no mail address stays** — a room by
  `urn:uuid:`, a phone — when the list is changed, and so does a second line
  of the same address. The form never showed them.
- **An event written by its length stays so.** A changed time replaced
  `DURATION` with a `DTEND` in UTC. Now the length is what the form's start
  and end say: moved whole, the event keeps its `DURATION` line untouched;
  a new length is written as `DURATION` (`PT2H30M`), and no `DTEND` appears.
- **Done and Status of a task are one state.** Changed together they were
  written as Done alone and the status was dropped. Now one follows the
  other when one is changed, and both changed have to agree: a task ticked
  done with «In process» chosen is not written, and Status says why.

### For developers

- Two patch operations: `model.SetTextAt` (one value's text; parameters and
  group stay) and `model.RemoveValue` (one value; the last takes the
  property). `textOps` writes through them; `wholeOps` is the old «one
  value for the whole property», used by tags.
- `checkEdit` (`internal/web/edit_check.go`) is the check; `readsAs` is the
  one measure of «the field is as the form showed it», used by
  `buildChange` and by the check. A new field needs its consequences in
  `editAlso` if it writes more than its own property.
- What the check does not see is said in S-7.3: writes that are not a
  form's (move, merge, import, attachments, photo, address rewrite), and the
  meaning of a change inside a property the field owns.

## v0.2.187 — 2026-10-11

Typecase `43ca489` (Typecase's note 44: tasks 144 and 145), client `v0.4.10`.

### Changed — behaviour

- **No list shows tags in its rows until asked to.** The columns a list of
  tasks stands with are Title, Due, Complete, Source; a list of notes —
  Title, Excerpt, Date, Source. «Tags» is in the menu, off, after them
  (`s-tasks-12`, `s-notes-23`). A choice a person made before is kept.
- On the duplicates screen «Link them» of a group still to decide and «Keep
  these values» of a linked one are the main buttons (`Action.Primary`,
  `s-dup-1`).

### For developers

- The set of the mockup has a sixth card — «Лебедева Вера» in «Контакты»,
  the same person as Вера Лебедева of «Архив» (`make-fixtures.py`): six rows
  and seven cards in the lists, and a pair to merge on the duplicates
  screen. Fixtures are re-shot; `app-duplicates-merge` is that pair.
- Not done yet, open in `docs/plan/server.md`: the counts of the sections
  while a poll runs and its words («Polling · 2 of 3 answered»), a merge
  that chooses the collection, the tag a list is filtered by first.

## v0.2.186 — 2026-10-11

A series given another time takes its exceptions with it, and a server that
loses them is named (the owner's decision of 11.10.2026).

### Fixed

- **A series moved to another hour or another zone left its exceptions
  behind.** Carrel rewrote the start of the series and nothing else: a day
  left out (`EXDATE`) and a day changed apart (`RECURRENCE-ID`) stayed at
  the old time, where the series no longer had an occurrence. The day left
  out came back, the changed day hung loose beside its series — on every
  server, without a word; Mail.ru then threw the loose ones away. Now both
  are moved as the start was and written the way the start is written. The
  time a day was moved to is not touched. Events and tasks.

### Changed

- **The report of what a server did not keep reads the exceptions by the
  instant they name.** A server that writes them in UTC beside a series in
  a zone has lost nothing, and `EXDATE` no longer stands in the report for
  that. A day left out that is gone is `EXDATE`; a changed day that is
  gone, or kept without what ties it to its series, is `RECURRENCE-ID` —
  also when every property of the series itself came back.

### For developers

- `CARREL_SERIES_REPORT=1` turns the live checks of an event's fields into
  a report of what a server does with such a series, step by step
  (`integration/series_probe_test.go`); `TestMemorySeriesReport` gives the
  same against the in-memory server, which keeps exactly what Carrel
  writes. The picture of twelve servers is in `docs/servers.md`.
- The live check of a series knows its exceptions by instant, not by the
  line as sent: «Mail.ru takes no `EXDATE` and no `RECURRENCE-ID`» (SV-36)
  was that check reading UTC for absence.

## v0.2.185 — 2026-10-10

A connection the server refused says so (the owner's word of 10.10.2026:
said as it is, nothing guessed beyond it).

### Changed

- **«Could not connect» names a refused login and password.** When the
  server answers 401 to a new connection, an edited one or a re-discovery,
  the notice reads «The server refused the login and password. Nothing was
  saved.» (or «…changed.») in place of «Discovery stopped at the principal
  step. …». The steps and the server's answer stay in the trace under it.
  Any other failure still names the step it stopped at.

## v0.2.184 — 2026-10-10

What the Linux gates with the race detector found on `v0.2.183`, the first
such run since `v0.2.155`.

### Fixed

- **A data race in the count of an uploaded file's bytes.** An upload that
  its server refused could end while the body was still being read, and the
  size shown in the queue's row was read at the same moment it was written
  (`TestQueueRowSaysWhyItFailed` under `-race`). The count is atomic now —
  for a file uploaded into a folder and for one attached to a record.

### For developers

- **The race run fits the dev machine again.** The container of
  `scripts/linux-gates.sh` gets 2 GB and no swap (it had 3 GB, more than the
  machine's other tenants leave: the host's OOM killer ended the run twice
  and could as well have ended one of theirs) and `GOMEMLIMIT=700MiB`.
- **Under the race detector an archive's key is stretched at 2^12**, not
  2^18 (`internal/backup/work_race.go`, build tag `race`): at 2^18 every
  sealing and opening takes 256 MiB and the detector's shadow of it does
  not fit. No shipped binary is built with the detector; the product's
  factor is unchanged and is now held by `TestArchiveIsSealedAtFullWork`.

## v0.2.183 — 2026-10-10

Two of the owner's answers of 10.10.2026: a browser asks before it shows a
file's thumbnail, and «Name and organisation» of a published contact can be
taken off.

### Changed — behaviour

- **A thumbnail is asked for before every showing.** `?thumb=1` answers with
  `Cache-Control: private, no-cache`, as a contact's photo does, in place of
  `private, max-age=31536000, immutable` of `v0.2.179`: the browser keeps
  the thumbnail and asks Carrel each time; an address that names the version
  it holds is answered 304 with nothing read or made. Signed out, the
  browser is told nothing and shows nothing of what it holds. Thumbnails a
  browser took under `v0.2.179` stay in its cache for up to a year; clearing
  the browser's cache removes them.
- **«Name and organisation» can be taken off** on the sheet of a contact and
  of a book (`s-publish-3`): it stands, as before, and is no longer
  read-only. Taken off, the visitor is given the name alone — no
  organisation and no title on the page, in the rows of a book or in the
  `.vcf`. The link keeps the choice (`PublishTarget.HideOrganisation`,
  `hideOrganisation` in the state). Only a post of the sheet takes it off —
  one that carries `lives`; a post without it publishes the name with the
  organisation, as the sheet stands. Links made before show the
  organisation, as they did.

### Unchanged

The side of a thumbnail (480 px), two made at a time, and the public page of
a file showing the original — the owner confirmed the three. The date of the
file panel stays in figures, as its frame draws it (`s-files-1`).

## v0.2.182 — 2026-10-10

«Send by letter» of a link no longer throws away the letter being written
(the scans' Пб15).

### Fixed

- **A letter half written survived only until the next «Send by letter».**
  Sending a link by letter — from the sheet of a link just made, or from a
  row of «Published» — replaced the letter with a new one. Now the link
  joins the letter being written: its recipients, subject, text and
  attachments stay, and the same link asked for again is not doubled. With
  no letter begun, the letter is started and named after the link, as
  before.

The sheet «Send this link» the frame draws (`s-mail-4`) — a letter of the
link alone, apart from the letter being written — is not built yet.

## v0.2.181 — 2026-10-10

The sheet of a published folder by its frame (`s-publish-5`; the scans'
Пб4).

### Added

- `PublishSheet.note` — the message a sheet always carries, before the link
  and after it. A folder's: «The visitor gets the folder with everything
  inside it, read-only, and the same address mounts as a drive in Windows,
  macOS and Linux.» Empty for the other kinds.

### Fixed

- **`expires` of a folder's sheet began with a full stop** — «. Whoever has
  the link can also mount the folder as a read-only drive.» — and lost the
  sentence once the link was made. `expires` is the day the link lives to
  and nothing else; empty before the link.

### Changed

- **A folder's link lives 14 days unless changed** (was 30), as its frame
  draws. A post that names no lifetime gets what the sheet of its kind
  stands at: a week for a contact, two for a folder, a month for the rest
  (in `v0.2.180` such a post of a contact still got a month).

## v0.2.180 — 2026-10-10

A published contact shows what its owner ticked, and no more (`s-publish-3`;
the scans' Пс11, Пб2). Before, the page of a contact gave the visitor every
field of the card, a book the phone and the address of every row, and the
`.vcf` the cards whole.

### Changed — behaviour

- **The sheet of a contact and of a book asks what the visitor sees.** A
  set «What the visitor sees»: «Name and organisation» (on, not to be
  changed), «Phone numbers», «Email addresses», «Photo», «Note» — off —
  and «Allow .vcf download and subscription» — off. The link lives 7 days
  unless changed (30 for the other kinds, as before).
- **The page, the rows of a book and the file give only that.** They are
  made from a new card of the allowed properties, not from the card cut
  down: an address, a birthday, a website, a property Carrel does not know
  are never given — there is no tick for them. Without «Allow .vcf» the
  page offers no file and `/p/{token}/download` is 404.
- **Links of contacts made before this release show the name and the
  organisation only.** Their file is still served, cut to the same. To
  show more, make the link again.
- «Photo» decides whether the photo is in the `.vcf`; the public page draws
  initials, as its frame does.

### Fixed

- A calendar published without descriptions: a record whose text could not
  be taken out was given with it. It is now left out of the file.

## v0.2.179 — 2026-10-10

Where a file's thumbnail is kept (the owner's decisions of 10.10.2026): in
the browser, by the version of the file, and in a budget of the session's
cache that is its own.

### Changed

- **The browser keeps a thumbnail.** An address that names the version of
  its file (`?thumb=1&v=…`, as the rows give it) answers with
  `Cache-Control: private, max-age=31536000, immutable`: the browser holds
  the thumbnail on the person's machine and does not ask again, across
  sessions. Asked again with `If-None-Match`, the server answers 304 at
  once — before, it made the thumbnail again when its own copy was gone.
  An address without the version, or with one the file no longer has, is
  asked every time, as before.

### For operators

- **Two new settings.** `CARREL_CACHE_MAX_FILE_THUMB_BYTES` (64 MiB) and
  `CARREL_CACHE_MAX_FILE_THUMB_ENTRIES` (2048) are the session's budget for
  the thumbnails of files. `CARREL_CACHE_MAX_THUMB_BYTES` and
  `…_ENTRIES` are the photos of contacts alone again: a book of contacts
  and a folder of pictures no longer push each other out. A session that
  looks at many pictures may now hold up to 64 MiB more;
  `CARREL_CACHE_MAX_PROCESS_BYTES` (256 MiB) still bounds all sessions
  together.
- Thumbnails are never written to the server's disk.

## v0.2.178 — 2026-10-10

A thumbnail for every picture the server can read (the owner's decision):
BMP and TIFF beside PNG, JPEG, GIF and WebP.

### Changed

- **A BMP and a TIFF have a thumbnail** (`FileRow.thumb`, the panel's
  preview) — a JPEG, like the others, so a browser that shows neither
  format shows the thumbnail. Over 16 MiB they say «too large», as
  `s-files-5` draws «панорама.tif». A picture is known by its bytes: a file
  named `.png` that holds a JPEG has its thumbnail too.
- **Dimensions** of a chosen BMP or TIFF are read from its head; a TIFF
  whose directory lies past the first 64 KiB says none.
- `.bmp`, `.tif` and `.tiff` have their media types on every machine
  (`image/bmp`, `image/tiff`), whatever the system's table says.
- A contact's photo is one of four formats, as before.

## v0.2.177 — 2026-10-09

The thumbnail of a file is made by the server (the owner's decision; S-17.1
said the opposite): the original no longer travels for it.

### Changed

- **`FileRow.thumb` and the preview of the file's panel are a new
  address**: `…/download/{path…}?thumb=1&v={version}` (was the original,
  `?inline=1`). The answer is a JPEG that fits a square of 480 px, the
  whole picture with its proportions and its orientation; it carries an
  `ETag` and answers 304. A client that printed the address as the source
  of an image has nothing to change.
- **A thumbnail is made for a picture of up to 16 MiB** (was 512 KiB):
  `FileRow.note` «too large» starts there. `s-files-5` draws pictures of
  640 KB, 712 KB and 2.8 MB as thumbnails.
- **The address may answer 404** — a file named as a picture that does not
  decode, or one of more pixels than `CARREL_PHOTO_MAX_PIXELS`: the icon of
  the row stays.
- `?inline=1` still gives the original in place: Open of a picture.

### For operators

- A thumbnail is kept in the session's thumbnail cache
  (`CARREL_CACHE_MAX_THUMB_BYTES`, 16 MiB by default) by the version of the
  file, beside the photos of contacts: a version is read from its server
  once. Making one holds the picture decoded, so at most two are made at a
  time in the process.

## v0.2.176 — 2026-10-09

The files section, the client's second scan (ФН7, ФН16): the data of the
mockup set under the frames, and the folder a sheet has just made.

### Changed

- **A folder just made stands first.** The node that New folder of a tree
  has just made (`FileTreeNode.fresh`) is the first among the nodes beside
  it, whatever its name — «2026-08 · just created» above «2026-07»
  (`s-notes-11`). The sheet opened again knows no fresh node, and its order
  is by name, as before.

### Fixtures

- `app-files-folder/tiles` is the folder `s-files-5` draws: «2026-07» of
  the attachment folder, seven items, 89.4 MB — pictures, a PDF, a folder
  nobody opened (no count) and one opened and empty («0 items»).
- `app-files-folder/readonly` is `states-34`: «Public» ticked («3 of 3»),
  two files, 5.3 MB.
- `app-files-picker/normal` is `s-notes-11`: the folder «2026-08» just
  made in the attachment folder and chosen — the head says «Documents /
  carrel / attachments / 2026-08».
- `app-files-folder/normal` — the panel has Dimensions, «1920×1080»
  (`s-files-1`). `app-settings-attachments/normal` counts the files of
  «2026-07».

## v0.2.175 — 2026-10-09

The files section, the client's second scan: the panel of a chosen file by
its frame (`s-files-1`).

### Changed

- **Changed of the file's panel is in numbers**, the same words as the
  row's `FileRow.modified`: «2026-08-10 11:07» (was «10 August, 11:07»).
  The frame draws one file twice — in the table and in the panel — and
  says its date the same way in both.

## v0.2.174 — 2026-10-09

The files section, the client's second scan (ФН8, ФН20): a row of the queue
that was refused (`s-files-24`), and fixtures of two states.

### Added

- `TransferRow.reason` — a row that was refused says why beside its name,
  in a few words: «the server needs the size» for a storage that writes
  only what it was told the length of; for other refusals, what the
  queue's `note` says of the row. What to do about it stays in `note`.
- Fixtures: `app-files-folder/refused` — the queue with a file uploaded and
  one refused for its size (`s-files-24`); `app-files-published/no-mail` —
  the links of an instance that sends no letters, no Send in the rows
  (`s-publish-17`).

## v0.2.173 — 2026-10-09

The files section, the client's second scan (ФН21): the queue hides and
shows (`s-files-2`, `s-files-3`).

### Added

- `TransferQueue.hidden` and `.toggle` on a folder's page: Hide
  (`queue.hide`) is the folder's address with `?queue=0`, where the queue
  comes hidden — its head alone is drawn — with Show (`queue.show`), the
  folder as it was. The queue an upload answers with (the fragment) has no
  `toggle`: it knows no folder.

## v0.2.172 — 2026-10-09

The files section, the client's second scan (ФН15, ФН22): the root with no
storage ticked (`states-35`) and Attach of From Files (`s-files-16`).

### Changed

- **The root of Files with nothing ticked** says «0 of 3 storages» under
  the title (the servers are not counted) and, in `empty`, «No storages are
  ticked» — «Tick the storages to show in the rail, or all of them at
  once.» — with «Tick every storage», as before. Fixture `app-files/empty`.
- **Attach of From Files has no icon** (`files.picker.attach`).

## v0.2.171 — 2026-10-09

The files section, the client's second scan (ФН9; the owner's decision of
09.10.2026 — the frame is right): the head of a sheet with a tree chooses
the server (`s-files-4`, `s-notes-11`, `s-files-16`).

### Changed

- **A sheet's tree holds the storages of one server.** The sheets of a
  move and a copy (`FileOpSheet`), of the menu's upload, of From Files and
  of Browse of the attachment folder (`FilePickerSheet`) carry `servers` —
  every connection with storages of files, the one shown `on`, the others
  with `href`, the same sheet with that server (`server=` in its address).
  The tree's roots are that server's storages alone and no longer name it:
  `FileTreeNode.stamp` of a root is empty. Without `server=` the tree is of
  the chosen folder's server; with another one asked for, nothing of it is
  chosen yet, so «Move», «Copy» and «Upload» stand off until a folder is.

## v0.2.170 — 2026-10-09

The files section, the client's second scan (ФН13): New folder of a
folder's head and of the rail's foot is a sheet (`s-files-23`).

### Changed

- **New folder opens a sheet.** A POST of `…/mkcol/{path…}` without a name
  answers `FileOpSheet` with `op` «new» (new in C-8): «New folder», «in
  Documents / carrel / attachments», the field «Name», Create — the form's
  button — and Cancel. A name taken or not fit for one is the sheet again,
  the refusal in `failure` and the name as typed; a folder made is a
  redirect to its folder. The address answers the zone `sheet` (was
  `main`).

### Removed

- `FilesPage.name` — the line of a new folder's name on the page.

## v0.2.169 — 2026-10-09

The files section, the client's second scan (ФН14; Typecase's note 38): the
sheet «Upload files» by its frame (`s-notes-11`, `s-files-19`).

### Changed

- **The upload from the menu remembers its folder itself.** «Upload» of
  the sheet (`files.picker.upload`) goes to the chosen folder's upload with
  `?menu=1`, and an upload that carried a file remembers that folder for
  the next one from the menu. Choosing a node remembers nothing; an upload
  from a folder's own page does not touch the menu's folder.

### Removed

- «Choose this folder» (`files.picker.choose`) of the sheet «Upload files»:
  no frame draws it. Browse of the attachment folder
  (`?for=attachments`) keeps its own.

## v0.2.168 — 2026-10-09

The files section, the client's second scan (ФН17–ФН19; the owner's word of
09.10.2026 — dates as the frame draws them, for now): the tables of files
and of published links by their frames (`s-files-1`, `s-publish-1`).

### Changed

- **Dates of the two tables are numbers.** `FileRow.modified` —
  «2026-08-10 11:07» (was «10 August, 11:07»); `PublishedRow.created` and
  `.expires` — «2026-08-10» (was «10 August 2026»), `.lastSeen` —
  «2026-08-10 08:41». «No expiry» and «never opened» are as they were.
  Everywhere else dates are said in words (C-10), as before.

### Added

- `PublishedRow.address` — the link whole, with the scheme and the host
  (`CARREL_PUBLIC_URL`, else the address the request came to): what Copy
  takes. `link` stays the path.
- `PublishedRow.unopened` — nobody opened the link: its «never opened» is
  drawn dimmed.

## v0.2.167 — 2026-10-09

The owner's words of 09.10.2026: what cannot be done is switched off, and
the button of a move is «Move».

### Changed

- **«Move» and «Copy», not «Move here» and «Copy here»** — the confirming
  button of the three sheets: files (`file.move.confirm`,
  `file.copy.confirm`), a record (`record.…confirm`), a collection
  (`collection.…confirm`). The identifiers are the same. The frames still
  draw «here» (Typecase's task 147).

### Fixed

- **The button of a file move stands off again while the folder chosen
  cannot be the destination** — «These items are already here.», «The
  chosen storage is read-only.», «A folder cannot go inside itself.» are
  its reason, as before `v0.2.165`, which had left it on and refused after
  the press. The sheet's form (`FileOpSheet.form`) stands either way; a
  node's `href` is the sheet with that node chosen and the button as it is
  for it.
- **Rename of the selection bar stands off, with why, when the address
  selects more than one** (`v0.2.166` had left it on).

## v0.2.166 — 2026-10-09

The files section, the client's second scan (ФН5; the owner's decisions of
09.10.2026 — the frame is right, and without scripts the selection bar
always stands): the selection of a folder is the form of its operations
(`s-files-5`).

### Changed

- **`FilesPage.select` is a POST of the folder's operations**
  (`…/op/{path…}`) with the hidden `_csrf`, not a GET of the folder. A tick
  of a row or of a tile is its field `selectField` — «item» — with the
  row's name as the value, ticked where `FileRow.selected` is.
- **Rename, Move, Copy and Delete of the bar are buttons of that form**
  (`submits`): the operation is in the button's address, the names are the
  ticks. They come in `bar.actions` with nothing selected too — a client
  without scripts draws the selection bar always, one with scripts while
  something is selected. An empty folder has none. Download stays a link,
  on for the one file the address selects.
- **The operations' address takes `item` from the form too.** Nothing
  ticked, or a rename of other than one, answers the folder with «Nothing
  was done» and why («Tick the files and folders first.», «Rename takes one
  item.») — it was a 404.

### Removed

- `?toggle=` and `?all=1` of a folder's address: the ticks are fields now.
  `?select=` stays — what the address selects is ticked, counted and opens
  the panel.

## v0.2.165 — 2026-10-09

The files section, the client's second scan (ФН10; the owner's decision of
09.10.2026 — the frame is right): the sheet of a move and of a copy is a
form (`s-files-4`).

### Changed

- **«Move here» and «Copy here» send the sheet's form.** `FileOpSheet.form`
  stands at a move and a copy too: a POST of the confirmed operation with
  the hidden `_csrf`. The tree names the field a node is ticked with —
  `FileTree.field`, «to» — and a node's value is its `id`. The button is
  the form's (`submits`), and it no longer stands off because of the node
  that was chosen when the sheet was drawn: «These items are already
  here.», «The chosen storage is read-only.», «A folder cannot go inside
  itself.» come as the answer to the form — «Nothing was done» — instead of
  as the button's reason. It stands off only for a move out of a read-only
  storage. A node's `href` — the sheet with it chosen — is still there.

## v0.2.164 — 2026-10-09

The files section, the client's second scan (ФН11, ФН12): the words of a
rename's and a deletion's sheet by their frames (`s-files-21`, `-22`).

### Changed

- **A rename** says where the item is — `FileOpSheet.subject` is the path
  without «from» —, its field is «Name» (was «New name»), and `warning`
  says whom the new address will lose.
- **A deletion** has `subject` without «from» too, and two messages:
  `FileOpSheet.alert` (new) — how many files, of what size and on which
  server are deleted, and that Carrel holds no copy, the count strong; a
  folder goes «with everything in it» — and `warning` in new words: nothing
  counted the links to these files. A client that drew `warning` alone
  draws `alert` before it, as an alert.

## v0.2.163 — 2026-10-09

The files section, the client's second scan (ФН1–ФН4): a folder's page by
its frames (`s-files-1`, `states-32`, `-33`, `-34`).

### Changed

- **The line under a folder's title is three fields.** `FilesPage.subtitle`
  is the connection alone; `count` — «5 items», everything the folder
  holds, whatever the filter leaves — and `access` — «read-only: nothing
  can be uploaded or made here» — follow it. A client that drew `subtitle`
  of a read-only storage adds `access`.
- **A storage that did not answer** says the words of the other sections:
  «The server did not answer as expected, so nothing on this screen comes
  from it.» `FilesPage.unread` is true, and the page carries no count, no
  rows and no queue.

### Added

- `FilesPage.empty` — an empty folder: «This folder is empty.», what to do
  with it, Upload with its arrow and New folder; on a read-only storage the
  title alone.
- `FileRow.open` — where the name of a row leads: a folder's to the folder,
  a file's to its folder with it chosen. `href` of a file is its download,
  as before.

## v0.2.162 — 2026-10-09

The frames of Typecase `0fe0244` (Typecase's note 42; the owner's decisions
of 09.10.2026), with the client's `v0.4.8`.

### Changed

- **The upload of the sheet «Upload files» is named «Upload».** The action
  `files.picker.upload` of `FilePickerSheet` (`/app/files/picker?for=upload`)
  was labelled «Upload here»; the frames (`s-notes-11`, `s-files-19`) say
  «Upload», without a count now that the files picked stand by name in the
  sheet. A client that shows the label shows the new word; the action's
  identifier, address and method are the same. Fixtures
  `app-files-picker/normal` and `refused` carry it.

## v0.2.161 — 2026-10-09

The owner's decision of 09.10.2026: a record has as many relations as it
has. RFC 5545 gives `RELATED-TO` any number of times; the card and the
panel already showed them all, the edit held the first.

### Changed

- **Related to of a task's and a note's edit holds every relation.** The
  row `related` is an `InputSelect` with `multiple`: every option chosen is
  a value under its name. A relation whose tick is gone is removed; one kept
  keeps its parameters (`RELTYPE`) wherever it stands now; the kept ones
  stay in the record's order and new ones follow. Before: the field was
  the first `RELATED-TO`, the others were kept untouched and could not be
  edited.
- **The line one more link is typed into** is the field's own part,
  `FormRow.add`: the name `related.add`, «Title or UID», «Add a link», its
  suggestions (`suggest`). It is there whether the record has relations or
  not (Typecase's task 142). What is typed is a UID, or the title of a
  record the session has loaded — which is then that record; a relation
  already there is not written twice.
- A client that sends one value under `related` keeps working: it is the
  one relation of the record.

### For consumers

- A form that sent only the first relation of a record with several now
  removes the others on save: send every chosen option.
- Every row of a form carries the key `add` — `null` where the field has
  no such line.

### Fixtures

- `app-task-edit/normal`, `-/new`, `-/loss`, `app-note-edit/normal`,
  `-/new`: `multiple` and `add` of the row; the note of the set has two
  relations, the event and a task.

## v0.2.160 — 2026-10-09

### Fixtures

- The set: the text of «Заметка со встречи 10.08» is the frames'
  (Typecase's note 37, `s-notes-2`, `-20`): no second heading «Отложено до
  следующего спринта», and its last paragraph ends «…и скажет в четверг»
  (`zapisi.ics`). `app-note/normal`, `app-note-edit/normal`,
  `app-notes-all/normal` follow. Nothing in the running server.

## v0.2.159 — 2026-10-09

Typecase `6833ef4` (Typecase's note 41, task 119), with the client's
`v0.4.6`.

### Fixed

- **The sizes of a second field of files were lost.** An upload
  (`…/upload/{path…}`) read the fields `size.<name>` only in front of the
  first file. A form with two fields named `file` — Files and Folder of
  the menu Upload (`s-files-8`) — sends the sizes of each field in front
  of its own files, so the second field's stand after the first field's
  files: its files went out without a length, and a storage that needs one
  refused them. The fields between files are read now, within the same
  ceiling of a form's bytes. A field nothing was chosen in comes as a part
  without a file's name and is no file, as before. C-5.5 says both.

## v0.2.158 — 2026-10-09

The frames of Typecase `3adc445` (Typecase's note 40; the owner's decision
of 09.10.2026), with the client's `v0.4.5`.

### Changed

- **Reminders are said in one line, apart by the dot**: `Reminders` of an
  event's and a task's card — «10 minutes before · 1 day before»
  (`s-agenda-1`, `-2`); before, a comma.
- **The panel of an event has its card's order**: `Details.fields` — When,
  Repeats, Reminders, Where, Status, Tags; before, Reminders stood before
  Repeats.

### Removed

- **A published link «made before links were kept».** Every link is kept
  whole with its token since 19.09.2026, and no instance holds an older
  one: the row of `/app/files/published` no longer has the state with Copy
  and Send switched off and that reason. S-, U-6.5 say so.

### Fixtures

- The set: «Синк-ап по релизу» has two reminders, 10 minutes and a day
  before (`lichnyj.ics`); `make-fixtures.py` writes the done task of
  `dela.ics` it had lost — the file itself is the same.
- `app-agenda-all/normal` (the panel), `app-event/normal`,
  `app-event-edit/normal`, `-/loss` (the second reminder).

## v0.2.157 — 2026-10-09

The owner's decision of 09.10.2026: a table counts what a person sees, the
rail what lies in a collection. C-6.3 says so.

### Changed

- **The line under a combined view's title and the poll's bar count rows.**
  `Subtitle` and `PollBar.summary` of the combined book, list and notebook
  count the rows shown — a linked record is one —, not the sum of the
  rail: «3 of 4 books · 2 accounts · 5 cards», «Polled 3 of 3 sources · 5
  cards» where the rail's books hold 6. The poll's bar of the combined
  calendar counts the events of the window on the screen, as the line
  under its title does (before: everything the poll brought).
- **The rail is as it was**: `SourceRow.count` is how many records the
  collection holds, the combined row their sum.
- **The section Tasks counts the overdue.** `NavRow.count` of `nav.tasks`
  is the tasks of the ticked lists that are late and not done; the other
  sections count records, as before.

### Fixtures

- `app-contacts/normal`, `app-tasks-all/normal`, `app-notes-all/normal`
  (the line under the title, the poll), `app-agenda-all/normal`,
  `-/partial` (the poll); `nav.tasks` in twelve fixtures.

## v0.2.156 — 2026-10-09

The frames of Typecase `0bf5f71` (Typecase's notes 37, 38 and 39; the
owner's decisions of 08–09.10.2026, Р-45, Р-48), with the client's `v0.4.4`.

### Changed

- **A collection the server gave no write to counts before it says so.**
  `Subtitle` of one book, list or notebook: «radicale · vps · 2 cards · the
  server granted no write access», «nextcloud · 2 notes · VJOURNAL · the
  server granted no write access» (states-4, -30). One that did not answer
  counts nothing; a notebook that did not answer says «nextcloud» alone,
  without «VJOURNAL» (states-29).
- **The line under a calendar's title names the zone and counts the
  events.** `AgendaPage.subtitle` of the combined calendar and of one
  calendar alike: the place and the window, the instance zone as a
  monospaced piece, then « · 6 events» — the events of the window. No count
  while the poll runs, nor for a calendar that did not answer.
- **The panel of a task has its card's order and its source.**
  `Details.fields`: Complete before Priority. `Details.source.path` and
  `.compared` are filled, as the card's (`s-tasks-1`).
- **`NoteDoc.foreign` is pieces, not a string**: the name of the first
  property Carrel does not know is a monospaced piece (`s-notes-20`); empty
  — an empty list. C-6 says so.
- **Rename is the primary button of its sheet** (`file.rename.confirm`,
  `s-files-21`).

### Fixtures

- `app-agenda/normal`, `app-agenda/empty`, `app-agenda-all/normal` (the
  line under the title), `app-contacts-one/readonly`,
  `app-notes/source-error`, `app-tasks-all/normal` (the panel),
  `app-note/normal`, `app-note-edit/*` (`foreign`), `app-files-op/rename`.
- `check.py`: a folder of files is held against the frame of its own
  address (`/app/files/{account}/{col}/{path…}`), its states against
  states-32 … -34, the sheets of Rename and Delete against `s-files-21`,
  `-22`. The rail of the duplicates waits for the server (`SERVER_TODO`):
  the frames draw every source of the four kinds.

## v0.2.155 — 2026-10-08

The owner's decision of 08.10.2026: a rail's tags are the section's.

### Changed

- **The rail of one collection carries the section's tags.** `Rail.tags` on
  the page of one collection are the categories of what the list read and
  of what the last poll of the section in this session read in the other
  ticked collections — in this one too when it did not answer now, and, for
  a calendar, in another window of it. Before, they were the open list's
  alone, and a list that did not answer had none. Each tag still leads to
  the open list with `?tag=…`: a tag no record of that list has gives an
  empty list. A session that has not polled the section knows the open
  list's tags only.

### Fixtures

- The states of one collection are taken after the section was read, as a
  person comes to them: `app-contacts-one/*`, `app-agenda/empty`,
  `app-tasks/source-error`, `app-notes/source-error` carry the section's
  counts and tags. «Команда» holds a second card in `app-contacts-one/*`
  (states-4 draws two letters); in `app-agenda/empty` «Общий» answers, as
  states-15 draws it. Both live in the tests alone.

## v0.2.154 — 2026-10-08

The frames of Typecase `80d1319` (Typecase's notes 35 and 36; the owner's
decisions of 07.10.2026, Р-34, Р-38, Р-39), with the client's `v0.4.2`: the
copy of the mockup came with it (416 frames).

### Changed

- **A subscription has its count in the rail.** `SourceRow.count` of a
  ticked feed is how many records the feed read holds (in a timeline — how
  many it found there), `null` until it is read or when it is not ticked;
  «ro» is no longer said in its place. The count is not in the section's
  sum nor in the combined row's.
- **New… keeps its arrow with nothing ticked.** The arrow of `record.new`
  of a combined view then lists every collection of the kind, and the
  button writes into the one last used, else the first that takes records.
- **A read-only list offers nothing that writes.** `ListBar.actions` of one
  collection that can only be read has neither `record.new` nor
  `collection.import` — Export, and Print where the list prints. Before,
  `record.new` stood there switched off with its reason.
- **A running poll says how long it runs in every section.**
  `PollBar.aside` — «0.8 s», «1 min 20 s» — for contacts, tasks and notes
  as for the calendar.
- **A list whose server did not answer has the sign in its own row.** On
  the page of one collection whose read failed, the collection's row of
  the rail is `unavailable` with the reason in `detail`. A count shown
  before stays in `count`.
- **A linked record says so in the list of one collection.** `badges`
  («linked ×2»), `sources` and `state` of a task's and a note's row count
  the members of its group that live in other collections.

### Fixtures

- The set has a third notebook, «Общие с семьёй»: read-only, not ticked,
  empty («2 of 3 notebooks»). Every rail and choice of notebooks holds it.
- `app-agenda/normal`, `app-tasks/normal`, `app-notes/normal` are one
  collection as its own frame draws it (`s-agenda-17`, `s-tasks-15`,
  `s-notes-31`): no record open in a panel; the task and the note linked
  with their copies.

## v0.2.153 — 2026-10-03

The conflict, the duplicates, the import and the collection sheets without
scripts (U-8; the client's scan of these sections: РБ2, ДбБ2–ДбБ4, ИБ4–ИБ6,
КА1, КА2, КГ2), and C-5 names what they read (ПБ9, ДбБ9, РБ9, ИБ10, КА4).

### Fixed

- **«Write this combination» wrote the person's edit over every row.** The
  radios of the conflict had no names and stood in no form, so without
  scripts the button sent no choice, and the server took «mine» for each
  row — the server's value the person chose to keep was overwritten.
  `ConflictPage.form` — POST …/conflict, `_csrf`, `etag`, `op=combine`; each
  row's radio is `ConflictRow.name` (`choice.<n>`) with `mine` or `theirs`,
  and `ConflictRow.choice` is what is chosen beforehand: a row the request
  does not name takes it. The button is the form's; Use only my edit, Use
  only the server version and Cancel stay forms of their own.
- **The threshold and the weights sent nothing.** `ThresholdControl.form` —
  `do=threshold` and the field `value`; `.weightForm` — POST
  `…/decide?do=weights`, a field per signal by `WeightControl.name`
  (`weight.<signal>`); Back to defaults sends the same form to `do=defaults`
  (the instance's weights for all). `weightActions` is gone: its buttons are
  `weightForm.actions`.
- **The answer to a group's question went nowhere.** With something to
  choose, `DuplicateGroup.form` — POST `/app/duplicates/decide`, `_csrf`,
  `group`; the radios stand in it (`MergeRow.name`, `choice.<PROP>`, the
  option's index) and its decisions are its buttons (`Action.submits`).
  `MergeRow.property` is the label: «Display name», «Organisation»,
  «Photo»; a photo's option is its server, «nextcloud».
- **Preview file** named no file field: `import.choose` carries `file:
  "file"`.
- **«Into» and the folders of notes chose nothing without scripts:**
  `TargetOption.href` — the wizard with that collection chosen, empty on a
  read-only one; `FileTreeNode.href` on the nodes of the folder tree. The
  move sheets of a record and of a collection and the export sheet carry
  `href` too.
- **The way in from a combined view chose «Into» in silence**: `GET
  /app/{kind}/import` answers the wizard's first step with nothing chosen,
  Preview file off with the reason, the rail back to the combined view.
- **The four collection sheets** have their head (`title`: «New calendar»,
  «Rename and colour», «Move calendar», «Delete on the server» …) and Cancel
  (`collection.cancel`): back to `?back=` — a path of the app; the settings
  pass `/app/settings/connections` — else the collection's list, a new
  one's kind.

### For consumers

C-5 now names the parameters of `/app/search`, `/app/duplicates/decide`
and `…/merge`, `…/{uid}/conflict`, `…/import` and the four collection
sheets (guard `TestC5NamesTheFieldsOfTheCrossScreens`). Draw the conflict's
radios by `ConflictRow.name` inside `ConflictPage.form`, its button first;
a group with choices as its form; the weights menu as `weightForm`; «Into»
items as links to `href`. Fixtures regenerated: the four conflicts, the
duplicates, the import, the record's move, the four collection sheets and
the connections page (the sheets' addresses carry `back`).

## v0.2.152 — 2026-10-03

The notebook's panel carries both attachments its frame draws (the
client's notes build, C25), and the live attachment suite follows the
card that only links an attachment.

### Changed

- Fixtures `app-notes-all/normal` and `app-notes/normal`: «Заметка со
  встречи 10.08» in the panel has its second attachment, the link to
  `запись-встречи.m4a` (24 MB, «Documents / carrel»), as s-notes-1 draws
  it; the tests of both put the link, as the note's page and edit did.
  The note's version in their actions moved.

### Fixed

- The live suite of attachments (`TestBaikalAttachments`,
  `TestRadicaleAttachments`) took Remove from the card and expected the
  link to download: since v0.2.146 Remove stands on the edit only, and a
  link into a storage opens in Files with the file chosen. It now opens
  the link in Files, downloads the file from the panel and removes the
  link on the edit.

### For consumers

Nothing in the running server. The panel of the notebook draws the
recording as the frame does; fixtures regenerated: the two above.

## v0.2.151 — 2026-10-03

What a folder holds and how large an image is, without a request per row
(the client's scan ФБ11; the owner's decision of 03.10.2026; S-17.1,
docs/servers.md SV-57).

### Added

- `FileRow.items` — a folder's «34 items», «1 item», «0 items». The folder's
  one PROPFIND now also asks for `nc:contained-folder-count`,
  `nc:contained-file-count` (Nextcloud) and `DAV:childcount`; where the
  server says nothing, a folder this session has already read is counted
  from that reading; otherwise the field is absent — unknown, not zero.
- `FileRow.dimensions` — «4032×3024» of the one image chosen (PNG, JPEG,
  GIF, WebP), and a `Dimensions` field in its panel after `Type`. It comes
  from one GET of the file's head, `Range: bytes=0-65535`; a server that
  sends the whole file is read no further, and a header past that leaves
  the size out. The result is kept per file version for the session.

### For consumers

Draw a folder's Type as «folder · 34 items» and its tile as «34 items» when
`items` is there, «folder» and nothing otherwise; an image tile chosen as
«2.8 MB · 4032×3024». Fixture regenerated: `app-files-folder/tiles.json`
(the chosen tile's size).

## v0.2.150 — 2026-10-02

The queue, the trees of the sheets and the published links as their frames
draw them (the client's scan: ФВ2, ФВ3, ФГ4–ФГ6, ФА1, ФА4, ФД1–ФД3).

### Changed

- **The queue** (s-files-2): `TransferQueue.title` — «Uploading 2 of 3»
  while a row waits for an answer, «Uploaded 2 of 3» when all are answered
  (Moving, Copying, Renaming, Transferring by the rows' kind); `.total` —
  what its files weigh; `.note` says only why something failed. A taken
  name is answered Keep both · Replace · Skip, in that order; Replace is no
  longer drawn as a danger (`transfer.overwrite`, the same answer).
- **New folder of a tree is a link** (ФГ4): `FileTree.new` is a GET of the
  same sheet with `new=1`, which answers it with the line of the name open;
  `FileTree.name` stands only then (and after a refusal), its `action` is
  the POST that makes the folder. Without `new=1` the sheet has no line.
- **A storage's node names its connection** (`FileTreeNode.stamp` —
  «sftpgo · vps»); `FileTreeNode.attachments` marks the attachment folder,
  `.fresh` the folder New folder just made (s-notes-11).
- `PublishedRow.kind` is a word, not a kind of record: a collection's link
  says its collection («calendar», «address book», «task list»,
  «notebook»), a record's its kind, and «file», «folder».

### Added

- `FilePickerSheet.where` — the chosen folder by its path, «Documents /
  carrel / 2026-08» (the head of the upload picker, s-notes-11, s-files-19).
- **Published links** (s-publish-1): `PublishedLinksPage.subtitle` — what a
  link is and, on a local instance, that it opens only from its network
  (danger); `PublishedRow.colour` and `.icon` (`i-doc` a file, `i-file` a
  folder); the refresh `published.refresh`.
- The settings rail's rows have their icons (`i-user`, `i-server`,
  `i-file`, `i-device`, `i-archive`, `i-link`, `i-theme`; davloom draws its
  mark).

### For consumers

Open the line of a tree's New folder by following `new.href`; send the
name with `name.action`. Draw the queue's head from `title` and `total`.
Fixtures regenerated: the files sheets and folders, the published links,
every page with the settings rail.

## v0.2.149 — 2026-10-02

The root and a folder of the files section as their frames draw them (the
client's scan: ФА1–ФА3, ФБ1–ФБ4, ФБ6–ФБ8, ФБ10, ФБ11, ФБ15) and a task
tagged «read» in the set (C24).

### Added

- **The root** (s-files-6): `FilesRootPage.subtitle` — «2 of 3 storages on 2
  servers»; `.note` — what the root is and, in bold, the storages not
  ticked; `FileCollectionRow.server` — the connection of the Server column;
  the refresh `files.refresh`, its `answered` — when a folder of a ticked
  storage was read last. Nothing ticked: «Tick every storage»
  (`rail.tick.all`).
- **The rail**: the root row is open on the root and the storage of the
  folder in a folder (`SourceRow.on`); the root row says «2 of 3»
  (`SourceRow.meta`).
- **A folder** (s-files-1, -2, -5, -7): one file chosen opens its panel —
  `Shell.details` with `Details.preview` (`Thumb`: the thumbnail or the
  icon of its type), Download (primary), Open (`file.view`, an image in
  place), Rename, Move, Size, Type, Changed, Path, Delete; `Close` is the
  folder with nothing chosen. `files.details` (Details of the head) hides
  and shows it (`?details=0`); `list.print` — Print in «⋯» of the head
  (`?print=1`). `ListBar.filterForm` — «Filter this folder» without
  scripts; `ListBar.size` and `.selectedSize` — what the folder and the
  selection weigh; `FileRow.icon` (`i-file`, `i-img`, `i-doc`) and
  `FileRow.note` «too large» for an image over the thumbnail's ceiling;
  the view switch has its icons (`i-menu`, `i-grid`). A folder's refresh
  says when it was read (`answered`).

### Changed

- The set has a done task «Прочитать договор аренды» tagged «read»
  (s-tasks-1 draws the tag in the rail). Every record's version in the
  fixtures moved by one.

### For consumers

Draw the root's message from `note` and the Server column from `server`;
the panel of a file from `details` (the words «Links», «Carrel keeps no
index of what points at this file…» are the screen's). `Details.preview`
is null for records. Fixtures regenerated: every fixture of the files, and
every one that carries a record's version or the tasks' counts.

## v0.2.148 — 2026-10-02

Fixtures of the files section (the client's scan, ФБ5, ФВ1, ФГ8, ФА5, ФД5,
ФД6): a folder, the operation sheet and more of the picker had none.

### Added

- `app-files-folder` (`FilesPage`): `normal` — one file chosen (s-files-1),
  `long` — three chosen and the queue of an upload, one done and one name
  taken (s-files-2), `tiles` (s-files-5), `readonly` (s-files-15), `empty`,
  `source-error`.
- `app-files-op` (`FileOpSheet`): `normal` — move (s-files-4), `copy`
  (s-files-11), `elsewhere` — to another storage (s-files-12), `refused` —
  New folder of the tree on a taken name (s-files-13), `rename`, `delete`.
- `app-files-picker/attach` — From Files (s-files-16), `/refused` — New
  folder of the upload picker refused (s-files-19);
  `app-files-published/empty`.
- New states of C-14.3: `tiles`, `copy`, `elsewhere`, `rename`, `delete`,
  `attach`.

### Changed

- `.md` is `text/markdown` on every machine: Go's table lacks it, and a
  system's table may not have it either.
- The set's files have the sizes and times the frames draw (412 KB under the
  thumbnail's ceiling, 2.8 MB over it); the files fixtures are taken after
  every section was read, so the sections' rail has its counts; a link's
  «Last opened» is the time of its last open (08:41), not a stamp of the
  run.

### For consumers

`FIXTURE_SCREEN` of `check.py` checks the folder against the `data-screen`
of s-files-1, which has no `{path…}` (Typecase task 121). The queue of an
upload is `FilesPage.queue` in `app-files-folder/long`: the fragment of
`…/upload/…` is the same `TransferQueue`. Fixtures regenerated:
`app-files/normal`, `app-files-published/normal`,
`app-settings-attachments/normal`.

## v0.2.147 — 2026-10-02

Files without scripts (U-8; the client's scan of the files section, ФГ3,
ФБ12, ФБ9, ФБ13, ФГ1, ФГ2, ФБ14): rename, New folder, selection, upload and
the choice of a folder in a tree were not reachable through the view model.

### Fixed

- **Rename** had no field for the new name: `FileOpSheet.form` — POST to
  the confirmed operation, `_csrf` and `name`, the current name first.
- **New folder** of the folder's head and of the rail posted no name and
  was refused. Without `name`, `POST …/mkcol/{path…}` answers the folder
  with the line of the name open: `FilesPage.name` («Name of the new
  folder», `folder.new.create`, `folder.new.cancel`); a refusal keeps it
  open with what was typed.
- **Selection** sent nothing: `FilesPage.select` — a GET of the folder with
  its query and `select`; a row's tick sends `toggle` with its name, the
  head's tick `all=1`; the answer redirects to the folder with the new
  `select`. «Clear selection» — `files.select.clear`.
- **Uploads** named no file field: `file.upload`, its Files and Folder,
  `files.picker.upload` carry `file: "file"`. A folder's row carries
  `FileRow.upload` (`folder.upload`) — the upload into it, for a drop on
  the row.
- **A tree chose nothing without scripts**: `FileTreeNode.href` — the same
  sheet with that node chosen. The operation sheet answers GET as well
  (`GET …/op/{path…}`: the sheet only; `go`, `mkcol`, `transfer` are
  dropped). «Choose this folder» of the upload picker names the chosen
  folder in its address.

### For consumers

C-5.5 now names every field: `select`, `toggle`, `all`, `view`, `sort`,
`filter` of a folder; `file`, `size.<name>` and `size` of an upload (S-17.1:
the sizes go in front of the files); `op`, `item`, `to`, `go`, `name` of an
operation. Draw a folder row's tick as a submit of `FilesPage.select`
(`toggle`), the head's as `all`; a tree node as a link to `href`; the rename
sheet's field from `form`. Fixture regenerated: `app-files-picker/normal`.

## v0.2.146 — 2026-10-02

The mockup copy of Typecase `bc575a8` (from-typecase note 32: tasks 78–82,
93, 94, 95, 106, 107 and the owner's decisions of 02.10.2026), with what its
frames now show.

### Changed

- **A source that did not answer** (states-16): the rail row is a mark with
  the reason in its hint — `SourceRow.message` and `.retry` are gone,
  `detail` stays. The poll bar over a list and a timeline no longer names
  the refusal in its summary («1 refused», the danger run): `PollBar.failed`
  — «1 failed», the red button of the menu of refusals; each refused source
  carries `SourceProgress.answer` — what was asked and what the server
  answered, «REPORT /dav/calendars/duty/ → 503 Service Unavailable»;
  `PollBar.retryAll` — «Retry all» when there are more than one:
  `POST /app/search/progress/{task}/retry` without `source` asks every
  refused source again. A stopped source is still said («1 stopped»).
- **The invitation shows the address and lets it change** (s-auth-12,
  s-auth-14): Email is in both forms, required — a letter's put in with
  «The address the invitation went to. Another one is confirmed by a
  letter.», a link's empty with «you@example.org» and «Confirmed by a
  letter.»; the line under the title no longer speaks of the address. Only
  the address the letter went to is proven; another one waits as pending
  and gets its own letter of confirmation.
- **An attachment's name is a link in every card** (task 93): data inside
  the record downloads, a file in Files opens there — `AttachRow.href` of a
  link is `/app/files/…`, its folder with the file chosen. Remove stands on
  the edit only: the card and the panel carry no `actions` for an
  attachment.
- **Backup** (s-backup-2…11, task 94): «Regenerate» is «New password», with
  `Action.hint` «The old password stops working; whatever runs this job has
  to be given the new one.»; the Gitea workflow is «Carrel backup · …», its
  secret `CARREL_BACKUP`, its file `.gitea/workflows/carrel-….yaml`. With no
  job, Backup in the settings rail has no count (s-backup-6).

### Added

- `Action.hint`: the hint of an action that can be taken, where its frame
  draws one.
- Fixture `app-contacts-suggest/normal` — the suggestions of the attendees'
  field (`SuggestList`, s-agenda-12).

### For consumers

Draw a refused source in the rail as an error mark with `detail` as its
hint; draw `PollBar.failed` as a red button with the menu of refusals
(`Drop`, `MenuRefusal`, `MenuFoot` with Retry all). The invitation's Email
is a field to type in. Fixtures regenerated: every page with a rail or a
poll bar, `invite/normal`, `invite/link`, the cards and edits with
attachments, `app-settings-*` (the Backup count).

## v0.2.145 — 2026-10-02

Attach on a new event, task and note (s-agenda-15; the lead's decision with
task 106, as the photo of a new card, `v0.2.133`).

### Added

- **Attach of a new form sends the form itself** (`Action.submits`,
  `file: file`): the first step issues the UID the record will have, the
  file waits under it in the session with what was typed, and the form
  comes back — `/new?uid=` — with the typing, the files in `attach` (each
  with Remove, which sends the form too) and the hidden `uid`. Its arrow:
  From this computer, and From Files, which sends the form and opens the
  tree of Files for the draft (`/app/files/picker?for=attach&new=`); the
  tree's Attach adds the file to the draft. Save creates the record under
  that UID with the files in it. Nothing is written before Save. Up to the
  attachment limit the files of a new record go inside it, together; a
  larger file of this computer is not taken (save, then attach — a new
  record has no link to keep), a larger file of Files is linked where it
  lies. `POST /app/{kind}/{account}/{col}/new?op=attach|pickfile|attachfile|detach`
  (C-5). The new note's Attach stands in the bar and the column.

### Fixed

- `TestRestoreFromHistoryIntoNewCollections`, `TestRestoreFromThisComputer`
  and `TestUploadAsksAboutTakenNames` counted the temporary files of the
  whole machine: another run at the same time made them fail. They count
  their own now (`TMPDIR`, `TMP`, `TEMP` at the test's directory).

### For consumers

Draw a new form's Attach as a submit button of the form with `formaction`
and the file field `file`; its arrow and Remove as submit buttons too.
Fixtures regenerated: `app-event-new/normal`, `app-task-edit/new`,
`app-note-edit/new`.

## v0.2.144 — 2026-10-02

The page of a note, read and in edit, as s-notes-2 and s-notes-20 draw it
(the client's scan of the notes, groups Г and Д).

### Added

- **Print over the text of a note** (`record.print`, `i-print`,
  `?print=1`); Edit has `i-pencil`, Export Markdown no icon (s-notes-2).
- **`AttachRow.place`** — where the file a link points at lies: in the
  attachment folder, where that folder lies as the rail of Files names it,
  «Documents / carrel»; elsewhere in Files, the storage and the folders
  above it; none for a file inside the record or a link outside the
  catalog (s-notes-1, -2, -20). The same for every kind: the calendar's
  decision of `v0.2.119` (Г7) said the place is not required; the frames of
  the notes draw it, so it is there.
- **`SourceBlock.uid` of a note's page**, read and in edit (s-notes-2, -20).

### Changed

- The set: «Заметка со встречи 10.08» is related to «Дописать раздел про
  кэш» too, holds the board of the meeting inside (412 KB) and two more
  properties Carrel does not know («X-JTX-COLOR and two more»); the tests
  put the recording of the meeting into the attachment folder and link it
  (24 MB), and a copy of the note into «Дневник» («Text differ»).

### For consumers

Read the bar of a note from `doc.bar`, the place of a linked file from
`place`. Fixtures: `app-note-edit/new` is new — the full form of a new
note (no frame draws it yet); regenerated — `app-note/normal` (the event
and the task read before, the attachments, the copy), `app-note-edit/normal`,
`app-notes/normal`, `app-notes-all/normal`.

## v0.2.143 — 2026-10-02

The notebook and its panel as s-notes-1 draws them (the client's scan of
the notes, groups Б and В).

### Added

- **`NotesPage.subtitle`**: «2 of 2 notebooks · 4 notes · VJOURNAL» on the
  combined notebook (the frame names no account), «nextcloud · 2 notes ·
  VJOURNAL» on one — how the notes lie on the server. **`NotesPage.empty`**:
  nothing ticked — «No notebooks are ticked» and «Tick every notebook»
  (`rail.tick.all`), `poll` is `null`.
- **Months**: the notes stand under the month of their date, «August 2026»,
  newest or oldest first, those without a date last under «No date»; by
  title one list without a rubric. **`NoteGroup.count`**.
- **«Filter by word» without scripts**: `ListBar.filterForm` of a notebook
  and of the combined one, `?filter=` — the notes whose title or text holds
  the word (C-5).
- **The panel beside a notebook without scripts**: `?open={a row's href}`
  answers with the note in `Shell.details`, `Details.close` the notebook
  without it, its row **`NoteRow.on`**.

### Changed

- **A linked row of the combined notebook** says how many sources it has:
  `stamp` «2 sources», the stamps in `sources` (s-notes-1).
- The line under a combined list is built of its pieces (`combinedParts`):
  an account named «radicale · vps» is not cut at its dot.

### For consumers

Draw the subtitle, the months and their counts, the filter form, the open
row and the panel from the notebook's model. Fixtures: `app-notes-all/normal`
is new (the combined notebook, «Что почитать про CRDT» linked, the panel of
«Заметка со встречи 10.08»); regenerated — `app-notes/normal` (with the
panel), `/source-error`, `app-notes-all/empty`.

## v0.2.142 — 2026-10-02

The card and the form of a task as s-tasks-2 and s-tasks-3 draw them (the
client's scan of the tasks, groups Г and Д).

### Added

- **`TaskPage.subtitle`**, **`TaskEditPage.subtitle`** (Runs): where the
  task is and, when it is late, by how much in alarm — «nextcloud · Дела ·
  ‹overdue by 3 days›». **`TaskEditPage.name`** — the task; «New task» for
  a new one.
- **Print** on a task's card (`record.print`, `i-print`, `?print=1`); Edit
  has `i-pencil`.
- **`TaskEditPage.source`** — Source in the form: the path, when it was read
  and what the server says of it. The card's source has its path and the
  comparison too.
- **`FormRow.suggest`** — where a field asks for suggestions as one types:
  Related to of a task and of a note asks
  **`GET /app/{tasks|notes}/{account}/{col}/{uid}/related?q=`** (`new` for
  a record not yet made) → `SuggestList`: the events, tasks and notes the
  session has loaded whose title holds what was typed — those it begins
  first, then the newest —, the title with the found part strong, the mark
  of the collection, «event · 11 Aug» in **`SuggestItem.meta`**, the UID the
  value; the record itself left out. Without scripts the field's options
  are the same records (U-6.3).

### Changed

- **The fields of a task's card** in the frame's order: Status (Needs
  action when it says none), Due (monospaced, how late in alarm), Starts,
  Complete («0%» when it says none) and Priority monospaced, Reminders as a
  field, Where, Link, Tags. «Original time zone» is a badge by Due.
- **`FormRow.disables` names every field the tick switches off, a space
  between them**: All day of a task switches off both its times —
  «start.time due.time» —, which come `readOnly` while it stands.
- **Percent complete is «0»** for a task that says none, as its card
  says «0%»: the form saved as shown writes nothing.
- No hint under Tags of a task's form (s-tasks-3).

### Removed

- `TaskPage.percent`, `TaskPage.reminders`, `TaskEditPage.done`,
  `.percent`, `.reminders`: the card's fields and the form's rows say them,
  each in one place (as the event's since `v0.2.120`).

### For consumers

Read the task's head from `name` and `subtitle`, its fields from `fields`,
its form's progress and reminders from the form's rows; switch off every
field `disables` names; ask `suggest` with `?q=`. Fixtures: `app-task/normal`
is «Собрать чемодан» (the frame's task: the trip, the copy in «Работа», the
file inside); `app-task-edit/new` is new — the full form of a new task (no
frame draws it yet); regenerated — `app-task-edit/normal`, `/loss`,
`app-note-edit/normal` (`suggest`).

## v0.2.141 — 2026-10-02

The list of tasks and its panel as s-tasks-1 draws them (the client's scan
of the tasks, groups Б and В).

### Added

- **`TasksPage.subtitle`**: «2 of 3 lists · nextcloud · 5 tasks» on the
  combined list, «nextcloud · 3 tasks» on one. **`TasksPage.empty`**:
  nothing ticked — «No lists are ticked» and «Tick every list»
  (`rail.tick.all`), `poll` is `null`, as the contacts' and the agenda's.
- **Print** in the head of a list of tasks (`list.print`, `?print=1`).
- **`TaskGroup.count`** and **`.overdue`** — the count by the rubric, an
  alarm for «Overdue».
- **The panel beside a list of tasks without scripts**: `?open={a row's
  href}` on one list and on the combined one answers with the task in
  `Shell.details`, `Details.close` the list without it, its row
  **`TaskRow.on`**.
- **The panel's fields** in the frame's order: Status (Needs action when
  the task has none), Due monospaced — and how late, in alarm, when it is
  (`Field.runs`) —, Priority and Complete monospaced («0%» when it says
  none), Where, Link, Tags. **`Details.attachHint`** under a task's
  attachments: «… up to 2 MB the file is written into the task itself,
  above that it goes to the attachments folder …» (the instance's limit).

### Changed

- **A linked row of a combined list of tasks** says how many sources it
  has: `stamp` «2 sources», the stamps in `sources` (s-tasks-1).
- **The line under a combined list names its account when there is one**:
  «2 of 3 lists · nextcloud» (s-tasks-1); more are counted, as before.
- `?open=` is taken out of every list's address (it stayed in the links of
  the lists of tasks and notes).
- The set: «Собрать чемодан» has a description, priority 5, a reminder a day
  before, its boarding pass inside (128 KB) and a relation to «Поездка в
  Казань» — an event in a file of its own, put into «Личный» by the tests of
  the tasks (the agenda frames do not draw it); «Отправить документы в
  МФЦ» has the description, priority 1, reminder and relation of s-tasks-3.
  The tests link «Собрать чемодан» with a copy in «Работа».

### For consumers

Draw the subtitle, the counts, the open row and the panel from the list's
model; a linked row's source column is its `stamp`, its hint `sources`.
Fixtures: `app-tasks-all/normal` is new (the combined list, the panel of
«Собрать чемодан»); regenerated — `app-tasks/normal` (with the panel),
`/source-error`, `app-tasks-all/empty`, `app-task/normal`,
`app-task-edit/normal`, `/loss` (the set).

## v0.2.140 — 2026-10-02

A note in edit is sent without scripts (the client's scan of the notes,
НД1; U-8): the document is a form.

### Added

- **`NoteDoc.form`** — the edit's form: where Save sends it (POST to the
  edit's address, a new note's `/new`), its hidden rows (`_csrf`, `etag`,
  `readat`), then the title (`summary`) and the text (`description`, with
  the caret) — the same values as `title` and `body`. The fields beside the
  text (`fields`) go with it. `null` while reading.

### Changed

- **Save of the edit's bar sends the document's form** (`Action.submits`):
  without scripts the title, the text and the fields go with it; its
  address carries the version as before. `Action.submits` without `file`
  sends the form itself.
- No hint under Tags of a note's edit (s-notes-20 draws none).

### For consumers

Draw the note's edit inside `doc.form`: the title and the text under the
names of its rows, the fields with them, Save as its submit button.
Fixtures regenerated: `app-note-edit/normal`, `app-note/normal`
(`form: null`).

## v0.2.139 — 2026-10-02

The organiser edits the attendees (s-agenda-3, -12 … -15; Typecase task 77,
the owner's decisions of 01–02.10.2026): an attendee is an address, the
address books only suggest.

### Added

- **`EventEditPage.attendees`** is an `AttendeeField` now (it was the
  read-only rows): the tags in columns — the organiser `fixed`, posted
  hidden; each other a ticked box under `name` (`attendee`), its `value`
  «mailto:…» —, the line typed in (`add`: `attendee.add`, «Add attendee»,
  «Name or address»), where its suggestions are asked (`suggest`) and the
  hint under it: who sends the invitations — the DAV server when it
  schedules (RFC 6638, as discovery learned), Carrel never. An invited
  person (`ORGANIZER` not one of their addresses on the connection) sees the
  tags read-only, no line, and «You are invited by …: only the organiser
  changes the list.» (s-agenda-14); their save changes no attendee.
- **Saving the attendees** (S-7): an unticked one goes, the others keep
  their values and parameters; one added comes with the name of their card,
  `PARTSTAT=NEEDS-ACTION` and `RSVP=TRUE`. What is typed without scripts is
  an address as it is, else the one person of the ticked books whose name
  holds it, if they have one address — else an error at the line and
  nothing written. A record that gains its first attendees without an
  organiser gets the person's own address and name. A form without the
  line (the quick sheet, a person's timeline) only adds.
- **`GET /app/contacts/suggest?q=`** → `SuggestList` (s-agenda-12, -13): the
  people of the ticked address books the session has read whose name or
  address holds what is typed, a row per address — the name with the found
  part strong, the address, the book and its mark, the value of the tag —;
  an address no book holds first as «Add», with «Not in the ticked address
  books.» It is the address the field names for Typecase's live layer.

### For consumers

Draw the attendees' field from `attendees` inside the form, post the tags
and the line; ask `suggest` with `?q=`. Fixtures regenerated:
`app-event-edit/normal`, `/loss`, `app-event-new/normal`.

## v0.2.138 — 2026-10-02

The attendees of an event's card and panel as the frames of task 77 draw
them (s-agenda-1, -2; the owner's decisions of 01–02.10.2026): an attendee
is an address, and the address books say who it is.

### Added

- **`AttendeeRow.href`, `.colour`**: a person found by the address in a
  ticked address book is a link to their card and the mark of their book;
  an unknown one has neither, and the screen draws the address. The books
  are the ticked ones whose lists the session has read — no server is asked,
  as for the records of Related. An address on two cards is the first
  book's, in the catalog's order. A name without CN is the card's.

### Changed

- The set of the frames: Виктор Гончаров is invited at `v@example.org`, an
  address in no book (`make-fixtures.py`, `lichnyj.ics`), as s-agenda-1 and
  -2 draw the unknown attendee.

### For consumers

Draw a known attendee by `colour` and `href`, an unknown one by `email`.
Fixtures regenerated: `app-event/normal`, `app-event-edit/normal`, `/loss`,
`app-agenda/normal`, `app-agenda-all/normal` (the address books are read
before them).

## v0.2.137 — 2026-10-02

The form of a new event as s-agenda-15 draws it (Typecase task 77): the
edit's form, empty, with what a new event starts with.

### Changed

- **The full form of a new event** (`/app/calendar/{account}/{col}/new`)
  starts at the next whole hour, an hour long, in the instance zone (chosen,
  without «as sent»); with one reminder, First «10 minutes before», and
  Every 1, nothing repeating. They are values, not samples: Save writes
  them. Still no `foreign` and no `delete`. The quick sheet is as it was.

### For consumers

A form saved as shown writes the hour and the reminder; clear Start or End
to have none. Fixture regenerated: `app-event-new/normal`.

## v0.2.136 — 2026-10-02

The agenda by the frames of task 77: a day without events is not printed
(the owner's decision of 02.10.2026), the steps of the bar are icons, and
the states have their own lines (s-agenda-1, states-13, -14, -15).

### Changed

- **`AgendaPage.days`** holds only the days with events: a week with
  events has no empty rubric (s-agenda-1 lost its empty Wednesday), and a
  window without any has no day at all. The published calendar's days too.
- **`AgendaPage.empty`** also says that a window holds nothing — one line,
  not seven empty days (states-15): «No events this week» (this day, this
  month, in this range) and «Nothing in Личный from 2 to 8 November.»; the
  combined calendar, once its poll is over, «Nothing in the ticked
  calendars …». No buttons.
- **`Range.presets`**: Previous and Next are named by the step and carry
  their icon — «Previous week» `i-back`, «Next week» `i-next`, «Previous
  day», «Next month» (s-agenda-1 draws them as icons with that title).
- **`AgendaPage.subtitle`** of one calendar is its connection, the window
  and how many events are in it — «nextcloud · 2 – 8 November 2026 · 0
  events» (states-15), no zone; with nothing ticked the combined calendar
  says only where it would look — «0 of 8 calendars · 0 accounts»
  (states-14).
- **`PollBar.aside`** of the calendar says how long the poll has run while
  it runs — «0.8 s», past a minute «1 min 20 s» (states-3, states-13).

### Removed

- **`AgendaRow.zone`**: the row of an agenda prints no zone (task 77); the
  zone an event came in is its card's, a badge at When. C-10.

### For consumers

Draw the agenda's days as they come, and its `empty` when there are none;
Previous and Next by their icon, the label their title. Fixtures
regenerated: `app-agenda/normal`, `/empty`, `app-agenda-all/normal`,
`/empty`, `/partial`.

## v0.2.135 — 2026-10-02

The mockup copy of Typecase `9e69804` (task 77): an event is edited in its
own zone (s-agenda-3, the owner's decision of 02.10.2026); `check.py` maps
the calendar's new frames.

### Changed

- **An event's form is in the event's zone.** The times of When are shown
  in the zone `DTSTART` is written in — an event from Asia/Yekaterinburg
  reads 12:00–13:00, as it was sent; UTC in UTC; a floating time, a day and
  a zone the database does not know as before, in the instance zone. The
  times typed are read in the zone chosen: choosing another zone keeps the
  clock the form shows (it used to keep the instant), Floating keeps it
  without a zone, and a new event in UTC is written at the clock typed in
  UTC. Tasks have no zone and stay in the instance zone. S-11, C-6.
- `docs/spec/check.py`: `app-agenda-all` is checked against
  `/app/calendar` (s-agenda-1 names both addresses); the agenda's states
  have their frames — `app-agenda-all/partial` states-13, `/empty`
  states-14, `app-agenda/empty` states-15; `app-event-new` is s-agenda-15,
  and its `FRAME_TODO` row is gone.

### For consumers

Show and send the event form's times as they come: they are the event's
zone's. Fixtures regenerated: `app-event-edit/normal`, `/loss`.

## v0.2.134 — 2026-10-02

The password sheet works without scripts (U-8; the lead's decision: a
defect).

### Fixed

- **`POST /app/settings/unlock` as a plain form** answered 404: the sheet
  opened by a plain link is a page, and its form could not be sent. Now the
  right password opens the window and redirects to the page the sheet was
  opened over; a wrong one answers the sheet as a page with the error at
  its field. The sheet's form carries a hidden **`back`** — `?back=`, else
  where the link was followed from —, followed only when it is a page of
  this instance under `/app/` or `/admin/`; otherwise Connections. Lock now
  reads its way back by the same check. With scripts nothing changed.
- No other sheet answered a plain POST with 404 (the pattern was looked for
  in every handler; the progress poll is a fragment by its nature).

### For consumers

Send the sheet's hidden `back` with its form. Fixture regenerated:
`app-settings-unlock/normal`.

## v0.2.133 — 2026-10-02

The photo buttons of a new contact's form send the form itself, so what was
typed is not lost (s-contacts-27, the lead's decision).

### Added

- **`Action.submits`** — the button sends the form it stands in to its
  `href` (formaction), multipart with the file field `file`: what was typed
  rides with the file. Add, Replace, Crop and Remove of the new form's
  photo carry it (Add and Replace with `file: photo`).

### Changed

- **The photo steps of a new form keep the typing** with the draft under
  the form's UID and give the form back with it: after Save photo, after
  Cancel of a first upload and after Remove — then under the same UID with
  no photo, and the next Add keeps that UID. A step that brings no form
  (the frame's own) keeps what was kept; nothing typed — as before, `/new`
  without a draft. A refused file gives the form back as typed.

### For consumers

Draw a `submits` action as a submit button of its form with `formaction`
and `formenctype="multipart/form-data"`, its file field inside the form.
Fixture regenerated: `app-contact-new/normal`.

## v0.2.132 — 2026-10-02

The folder of notes is chosen in a tree, down into the folders of a
storage (s-io-4).

### Changed

- **`ImportSource.folder` is a `FileTree`** (was a `TargetPicker`): the
  storages, and the folders of the one opened, level by level down to the
  folder chosen; each node is a GET that comes back with it chosen
  (`chosen`), the folder below the root in `?in=`. «Preview folder» takes
  the folder chosen — its own `.md` files, not its subfolders' —, and the
  head of the preview says the way, «Docs › notes». A path in `in` that
  would leave the storage chooses nothing.

### For consumers

Draw the folder of notes as a tree, as the trees of Move and of the quick
upload. No fixture shows it.

The live suite of import reads the preview as one table (it still read the
two tables of `v0.2.129`) and chooses the folder in the tree.

## v0.2.131 — 2026-10-02

Notes come from a zip of Markdown files too (s-io-8).

### Added

- **A zip of Markdown files in the wizard of notes**: every `.md` in it,
  its folders too, is a note, its row naming its file («idei/crdt.md»);
  what is not Markdown is «Skipped: not Markdown», an empty one «Empty
  file: nothing to store and nothing to name it by». `Source.kinds` of
  notes takes `.zip`. The archive is somebody else's (S-29.4): told by its
  content, its members counted against `CARREL_IMPORT_MAX_RECORDS` before
  one is read, what is unpacked counted against `CARREL_IMPORT_MAX_BYTES` as
  it is read (a member past it refuses the file whole); no name of it is a
  path — one that would climb out is named and left out; an encrypted
  member is not opened.

### For consumers

Offer `.zip` where `source.kinds` says so.

## v0.2.130 — 2026-10-02

The preview of an import is the frames' one table (s-io-5…8).

### Changed

- **`ImportPage.preview` is one table**: the kind's columns and `Result`
  last — contacts Source, Name, UID; events Source, Summary, When; tasks
  Source, Summary, Due; notes File, Title, Date. The records first —
  «Ready», «Ready — the series as one record», «Ready — 40 % done, priority
  1», «New UID will be assigned» —, then what the file holds and the import
  does not write: «Skipped: a calendar takes events — import it into a list
  of tasks», «Skipped: not Markdown», a parse error in its words. The
  tables «To create in …» and «Skipped» are gone; `rubric` is empty,
  `count` the rows. A note of a folder names its file.

### For consumers

Draw `preview[0]`. No fixture shows a preview.

## v0.2.129 — 2026-10-02

The card of the external trigger hands over its tabs as s-backup-2…5 draw
them: crontab, GoSentry, Gitea, curl, and the password on its own.

### Added

- **`TriggerBlock`**: `shown` (just issued — «visible only now»; the tabs
  are empty after), `script` (the job needs a script: Download, Both),
  `command` (crontab: the line, or the script), `scriptName` («Save as
  carrel-….sh»), `cronLine` (the line of crontab that runs the script),
  `at` («03:00»), `header` (the secret's header, «Authorization»),
  **`goSentry`** (`GoSentryJob`: Name, Folder, Schedule, Command,
  Arguments one per line — for a script, its path and no arguments),
  `status` (the request of the second GoSentry job: `GET
  /api/backup/status?job=…`), `gitea`, `giteaFile`, `giteaSecret` (a Gitea
  Actions workflow on the same schedule, the secret a repository secret by
  name, never in the text), `curl` (the request alone), `endpoints` («What
  it opens»). The words of the card are the screen's (U-1.3).

### Changed

- **A job that downloads gets a script**, as the frame says («Put Download in
  the destination and what is handed over here becomes a short script»),
  not a line with `-o`.

### For consumers

Draw the tabs from these fields while `shown`. No fixture carries a trigger.

## v0.2.128 — 2026-10-02

Restoring as s-backup-8 draws it: each collection of the archive goes where
its row says, and restoring is an import, not a rollback (the lead's
decision: the frame rules).

### Added

- **`RestoreBlock.match`** (`RestoreMatch`) — the matching: a row per
  collection of the archive (`RestoreRow`: «nextcloud / personal», its
  colour, kind, how many), its tick (`pick.N`), «Restore into» (`into.N`:
  the collections of its kind that take writes on every connection, then
  New collection, Skip this one) preset to the collection matched by path —
  on a server that is gone, another one —, and «What will happen» before
  anything is written («matched by path — 4 records missing, will be
  created», «no differences», «that server is gone — another collection
  chosen», «a new collection on nextcloud — 312 records will be created»,
  «skip»; `danger` when it is an alarm). The bar of the selection — «3
  selected», Restore into (`into`) for the ticked rows, Skip, Select all —
  is the same form sent to `?op=restore-match&do=into|skip|all`; the page
  comes back with the choice. «Restore N collections» counts the ticked
  rows that go somewhere.

### Changed

- **Restoring creates what is missing and nothing else**: into an existing
  collection, a record missing there is created under its UID, the same
  record (same UID, same content but the server's stamps) is not made
  twice, a UID held there by different content gets a fresh UID and a line
  in the report; files the same way (the same bytes stay, other bytes under
  the name get a file beside it). Nothing on the server is deleted or
  overwritten. New collection makes one as before (a new folder for files).
  The report: «Restored» or «Restored in part», what went where, the fresh
  UIDs.
- **`RestoreBlock.preview` and `.targets` are gone**; `actions` holds Cancel,
  «Restore N collections» stands in the matching form. The note under it
  is the frame's. S-24.3 says the rule.

### For consumers

Draw the matching from `restore.match`; send its form with every row's
`into.N` and the ticked `pick.N`. Fixture regenerated:
`app-settings-backup/normal`.

## v0.2.127 — 2026-10-02

Lock now, as s-profile-4 and s-plan-1…5 draw it beside «credentials
unlocked · 4:12».

### Added

- **`TopBar.lock`** — Lock now (`unlock.lock`, POST), while the unlock
  window of the session is open; `null` otherwise.
- **`POST /app/settings/lock`** — ends the unlock window of this session at
  once: every secret asks for the sheet again; the sign-in, the session's
  key and its data stay (S-10.7). Nothing is released, nothing journaled.
  The answer is a redirect to the page it was pressed on — the `Referer` of
  this origin under `/app/` or `/admin/` —, else Connections.
- **The messages about it, in pieces**: under the cards of Connections while
  the window is open, «While credentials are unlocked, every account opens
  without a question — … **Lock now** ends it at once; so does signing
  out.» (`ConnectionsPage.note`; an open feed panel says its own); under the
  rows of «Anything else» **`DevicesPage.unlockNote`**, «Credentials are
  unlocked for five minutes — … **Lock now**; every reveal is written to the
  audit log.», the span in words from `CARREL_UNLOCK_TTL`.

### For consumers

Draw `top.lock` beside the mark of the window; it posts with its token.
Fixtures regenerated: every page in the shell (`top.lock: null`),
`app-settings-devices/normal`.

## v0.2.126 — 2026-10-02

Carrel installs as an application: the web manifest and its PNG icons come
from the binary (U-11, the lead's decision П3).

### Added

- **`GET /manifest.webmanifest`** (`render.ManifestPath`) — the name, the
  start at `/app/`, `standalone`, the colours, and the icons; `no-cache`.
  Anonymous, outside the buckets, as the library's files.
- **`GET /static/icon/{version}/{file}`** — `icon-192.png`, `icon-512.png`
  on the light plate and `icon-maskable-512.png` on the canvas of 120 of the
  icon reference (`docs/visual/carrel-icon.html`, 1.1), drawn in code from
  the mark's master geometry on the first request; the version is the hash
  of what was drawn, the files `immutable`, another version 404.

### For consumers

Link the manifest at `render.ManifestPath` under the base path (Typecase's
task 91 prints the link and the worker of the shell).

## v0.2.125 — 2026-10-02

The names the client hands the library are the server's constants, and
C-5 says which addresses print (stage 4 of the client's plan, С5, С6, П4,
П5).

### Added

- **`render.CSRFField`, `render.CSRFHeader`** (`_csrf`, `X-CSRF-Token`),
  **`render.PrintParam`, `render.PrintValue`** (`print`, `1`) beside the
  headers, zones and events already there: the client's glue names none of
  them as a literal. The server reads its token and writes its Print links
  by them. `render.TestTheNamesAreTheSpecs` holds every name of the package
  against C-5, C-7.1, C-7.2, C-7.4 and C-11 read from the spec.

### Changed

- **C-5: which addresses take `?print=1`** — the list of contacts and the
  agenda (combined and one collection, `/app/{kind}` with the same
  parameters as the row under it), a record's card (Print in the head of a
  contact's and an event's), a person's timeline; the same route and view
  model, the parameter unread by the server. Guard:
  `view.TestFixturePrintsAreC5`.
- **C-6.8 `Disables`**: the field it names is disabled and is not sent while
  the tick stands (the lead's decision П5, as the library does it).
- **C-11**: a POST outside a form is its own form with `_csrf`, scripts or
  none; a form sent by a script carries the field and the header (П4).

### For consumers

Take the names from `render`. Nothing on the wire changed.

## v0.2.124 — 2026-10-02

The poll bar says where to follow it, and the events of the progress stream
are two (stage 4 of the client's plan, С2, П1, С4).

### Added

- **`PollBar.stream`, `.poll`, `.every`** — the address of the progress
  stream (empty with `CARREL_PROGRESS_MODE=poll`), of the fallback poll, and
  its interval in milliseconds (`CARREL_PROGRESS_POLL_MILLIS`), each with
  the query of the page the bar stands on (a list's filter, window,
  switches). A bar without a task has none.
- **`render.EventPoll`, `render.EventPollFinished`, `render.Events()`** —
  the names of C-7.4.

### Changed

- **C-7.4 is the stream's two events**: `poll` — the bar changed, its last
  one with the outcome; `poll-finished` — the poll is over, and its data is
  the **rail** of the page the poll stands on (its counts, the nav's, and
  the states of the rows that did not answer are the poll's, C-7.3), or the
  bar when that page is gone. It carried the bar a second time. The six
  other names of C-7.4 were never sent and are gone from it: what they
  named is a notice, a move, an out-of-band zone or an empty fragment.
- **The fallback poll of a finished task** answers the bar with the page's
  rail out of band.

### For consumers

Follow `stream` while `running`; without it, ask `poll` every `every`
milliseconds. On `poll-finished`, replace the rail with its data and close
the stream. Fixtures regenerated: every screen with a poll bar.

## v0.2.123 — 2026-10-02

Every action and form says which zone a fragment answer of it replaces
(stage 4 of the client's plan, С1).

### Added

- **`Action.target`, `Form.target`** — the zone of C-7.2 a fragment answer
  replaces, by C-5: `sheet` where the address answers with a sheet (Move,
  Publish a link, the collection sheets, the password sheet, the quick
  sheets, the picker, Reset a password…), `rail` for the rail's form,
  `poll` for Stop and Retry of the poll bar, `queue` for an upload, `main`
  for a new folder. No key — a move to another page: ask for no fragment
  there (C-7.1). A button of a form (no `href`) has none; its form has it.
  The server puts it in on rendering, as the token (`Action.csrf`).
- **`ContactRow.target`, `AgendaRow.target`, `TaskRow.target`,
  `NoteRow.target`, `TimelineRow.target`** — `details`: a row of a list, and
  a record's row of a person's timeline, opens the record in the panel, a
  fragment at the record's address (C-5.4, C-7.2). The same address from a
  menu's Open, a row of the card's timeline and a related record are moves.

### Changed

- C-5: the rewrite sheet answers «лист (GET); страница, редирект (POST)»;
  `/app/sources/{kind}` names `files` among its kinds.
- The poll bar of the progress stream carries the session's token on its
  POSTs, as the pages' bars do (it went out without).

### For consumers

Aim a fragment request at `target` and nowhere else; an action, a form or a
row without it is a page to go to. Fixtures regenerated: every screen with
a sheet, a rail, a poll bar or a list.

## v0.2.122 — 2026-10-02

Three answers that were fragments outside every zone of C-7.2 are pages
(the lead's decision П2, stage 4 of the client's plan).

### Changed

- **`POST /app/duplicates/decide`** answers the duplicates page as it now
  stands (`DuplicatesPage`), what there is to say in `notices` — not the
  group (`DuplicateGroup`) aimed at `main`. A form without scripts and
  nothing to say is still a redirect to `/app/duplicates`; «This group is
  not among the loaded records» now reaches it too (it was dropped by the
  redirect).
- **`POST /app/settings/connections/test`** (Test only) answers the
  connections page with the dialog as typed — the password is not given
  back — and the trace in the new **`ConnectionsPage.trace`**; without
  scripts it works too (it was 404).
- **`POST /admin/settings/mail-test`** answers the settings page with its
  `test`, asked for a fragment or not.

### For consumers

Ask these three for a page, or for the zone you show of it (C-7.2): no
fragment of C-5 is outside the zones any more. Fixture regenerated:
`app-settings-connections/normal` (`trace: null`).

## v0.2.121 — 2026-10-02

The full form of a new event has its fixture (the client's scan of the
calendar, Д9).

### Added

- **Fixture `app-event-new/normal`** — `/app/calendar/{account}/{col}/new`,
  the full form of a new event in «Личный»: «New event», the calendar, no
  `delete`. Its frame is not drawn yet: `check.py` holds it in `FRAME_TODO`
  (to-typecase task 77).

### For consumers

Nothing in the running server. The client's registry of fixture screens
knows `app-event-new` (client `v0.1.3`).

## v0.2.120 — 2026-10-02

The form of an event as s-agenda-3 draws it (the client's scan of the
calendar, group Д, and Г11).

### Added

- **`EventEditPage.name`** (the event; «New event» for a new one) and
  **`.subtitle`** (the calendar) — the form's head.
- **`EventEditPage.foreign`** — «Kept as the server sent them»: the
  properties of the event Carrel does not know, name and value as they are,
  monospaced (`SEQUENCE`, `X-MOZ-GENERATION`, …).
- **`FormRow.valueAside`** — dim after the chosen value: «as sent» by the
  time zone the event came with, while it is chosen.

### Changed

- **Advanced RRULE is empty**, the record's rule its sample (`placeholder`)
  and the hidden `repeat.base`: a rule typed there wins over the fields;
  left empty it is no rule typed — the series keeps its rule, changed by the
  fields. «The advanced value takes precedence when set.» under it.
- No hints under Tags and Count of the event's form (the frame draws none).
- **`EventEditPage.reminders` and `.recurrence` are gone**: the reminders
  are the rows `reminder.N` of the form, the rule is its Repeats; the line
  under the deletion stays `DeleteNote`. `Attendees` lists the organiser
  first, as the card does.

### For consumers

Read the head of the event's form from `name` and `subtitle`; send
Advanced RRULE empty unless it is typed, with `repeat.base` as it came.
Fixtures regenerated: `app-event-edit/normal`, `/loss`. The full form of a
new event (`/new`: «New event», no `delete`) has its head too; its fixture
waits for the client's map of fixture screens (`app-event-new`).

## v0.2.119 — 2026-10-01

The event's card and its panel beside the agenda as s-agenda-1 and
s-agenda-2 draw them (the client's scan of the calendar, groups В and Г).

### Added

- **The panel beside the agenda without scripts**: `?open={a row's href}`
  on an agenda (one calendar and the combined one) answers with the record
  in `Shell.details` — `Details.close` is the agenda without it — and every
  row of that record `on` (**`AgendaRow.on`**), as the contacts' since
  `v0.2.86`.
- **`Field.runs`** — a value in pieces: When of an event is «‹10:00 –
  11:00›, Monday 10 August» (the year only when it is not this year's; a day
  — «all day, Monday 10 August»), `value` empty. **`Field.badge`** — a badge
  after the value: «created 12:00 Asia/Yekaterinburg» by When of an event
  made in another zone (the row «Original time zone» is gone), «until
  2026-12-28», «5 times», «once» by Repeats; no key without one.
- **Repeats and Reminders are rows of an event's fields**: «Weekly on Mon»
  with its end as the badge, «10 minutes before»; the card's order is When,
  Repeats, Reminders, Where, Status, Tags, the panel's When, Reminders,
  Repeats, … (s-agenda-2, s-agenda-1). `RecurrenceBlock` stays (the rule in
  words, the raw rule, the next times, the line under the deletion).
- **Print on an event's card** (`record.print`, `i-print`, `?print=1`); Edit
  has `i-pencil`, New task `i-task`, and New task is there in a calendar
  that holds no tasks too — the quick sheet in the task list a new task goes
  to, related to the event.
- **`Action.file`** — the field of the file of a POST that carries one:
  «file» on Attach and From this computer (`attach.add`), «photo» on Add and
  Replace of a contact's photo. Without scripts the action is a
  multipart/form-data form with that file input and `_csrf`.
- **The source of an event** says its path and when it changed:
  «changed on server 3 days ago» from LAST-MODIFIED (`SourceBlock.compared`;
  «changed on server since» when the read list holds another version).

### Changed

- **The organiser is the first of an event's people** (`Attendees`, status
  «organiser») and no field; statuses are written in lower case,
  «accepted», «no reply» (s-agenda-1, s-agenda-2). An organiser who is an
  attendee too is listed once.
- **`EventPage.reminders` is gone**: the reminders are the row Reminders.
- **Sizes** have no «.0» and no decimal from a hundred up: «3 KB», «2.8
  MB», «412 KB» (C-10).
- **The fixture set**: «Синк-ап по релизу» carries a reminder ten minutes
  before, its checklist inside the event (3 KB, «in the event»), and
  `SEQUENCE`, `X-MOZ-GENERATION`, `X-APPLE-TRAVEL-ADVISORY-BEHAVIOR` (its
  `X-JTX-COLOR` is gone); the event card's test links it with a copy in
  radicale · vps · Проекты («Holds the same») and reads the notes first, so
  Related names «Заметка со встречи 10.08 · note».

### For consumers

Read When and Repeats from `runs`, `value` and `badge`; Attach is found by
its name (`attach.add`) wherever the frame stands it; send a file by
`Action.file`. Fixtures regenerated: `app-agenda/normal` and
`app-agenda-all/normal` (with the panel, Синк-ап open), `/partial`,
`app-event/normal`, `app-event-edit/normal`, `/loss`, the edits and cards of
tasks and notes and the contact's edit (`file`), `app-mail-compose/normal`
(sizes).

## v0.2.118 — 2026-10-01

The agenda and the shell of the calendar as s-agenda-1 draws them (the
client's scan of the calendar section, groups А and Б).

### Added

- **`AgendaPage.subtitle`** (Runs): «6 of 8 calendars · 3 accounts · 10 – 16
  August 2026 · ‹Europe/Moscow›» on the combined calendar (subscriptions are
  calendars, not accounts), «nextcloud · 10 – 16 August 2026 ·
  ‹Europe/Moscow›» on one (C-10: the window's phrase).
- **`AgendaPage.empty`**: nothing ticked — «No calendars are ticked» and
  «Tick every calendar» (`rail.tick.all`), `poll` is `null`, as the contacts'.
- **`ListBar.show`** — «Events · Tasks due · Notes» as one GET form on the
  agenda: the window and the category hidden, an empty `with` (the form
  spoke) and the check `with` with `events`, `tasks`, `notes`. Events is a
  tick like the others (s-agenda-1 draws it ticked, not locked). An address
  without `with` shows the events, as before. `null` on every other list.
- **Range** among the windows (`range.range`, `view=range` with the shown
  days; `on` when the window is a range); Previous, Today, Next stay.
- **Print** (`list.print`, `i-print`): the agenda's address with `print=1`.
- **«Compact rows»** on the agenda (`ListBar.compact`), cookie
  `carrel_compact_calendar` (S-9.6).
- **`AgendaDay.count`**.
- **Badges of an event's row**, by importance, the first the one the row
  draws: «tentative» / «cancelled» of its status (alert), «linked ×2»,
  «4 attendees» (the organiser and the attendees), «repeats weekly»
  («repeats every 2 weeks», «repeats»). «Due» and an instance's
  «Cancelled» stay first.
- **`PollBar.aside`** — the right of the bar once the poll is over: «2 min
  ago», «from cache · 2 min ago»; **`PollBar.note`** — under the rows when a
  source did not answer: «What arrived in time is shown. Records from the
  unavailable source are missing — …» (states-6; contacts «Cards … book»,
  states-9).
- **`RailSection.feeds`** — the section of subscriptions (the frame's badge
  «external» is the screen's word); in the answer only when true.
- **Refresh of one collection's list** (`list.refresh`, the list's address
  as it stands; `answered` — when this session read it), every kind; the
  combined view with nothing ticked shows `poll.again` off, «Nothing is
  ticked to ask.».

### Changed

- **The poll of the calendar names the source that did not answer**:
  «Polled 5 of 6 sources · 14 events · ‹baikal · Общий unavailable›» (was
  «… · 1 refused»); the other lists count as before.
- **The day's rubric is «Monday · 10 August»**, an all-day row's time «all
  day» (C-10).
- **New event of the combined calendar** writes into the last used
  calendar when it is ticked, else into the first ticked one — never into
  an unticked one (it led to «Дни рождения»). The fixture set's last used
  calendar is «Личный».
- **«Tick every book / calendar …» is primary** (states-7; the client's stop
  C19).
- `Bar.segments` of the agenda is empty: its kinds are `show`.
- Two rows of the agenda at the same time with the same title stand in the
  order of their sources' stamps («nextcloud · Работа» before «radicale ·
  vps · Дежурства»), not of the identifiers Carrel issued — the order
  changed from run to run.

### For consumers

Read the agenda's kinds from `bar.show` and send it as the form it is; the
first badge of a row is its caption. Fixtures: `app-agenda-all/normal` is
new (the combined calendar, s-agenda-1); regenerated — every agenda, every
list (`bar.show`, `null`), the one-collection lists (refresh), every poll
(`aside`, `note`), the rails with subscriptions (`feeds`),
`app-contacts/empty` (primary). `check.py`: `SERVER_TODO` is empty.

## v0.2.117 — 2026-10-01

Two findings of the client's scan of the calendar: the «empty» week was not
empty (Б16), and the end of a series was shown a day late (Г3).

### Fixed

- **The day a series ends is the last day it can start on** (C-10):
  `UNTIL=20261228T235959Z` of «Синк-ап по релизу» (Mondays, 12:00
  Asia/Yekaterinburg) is 29 December, 02:59 in Moscow, and the card said
  «Weekly on Monday, until 29 December 2026», the form's Until 2026-12-29 —
  while the last Monday is the 28th. An UNTIL whose clock in the instance
  zone is before the series' time of day now ends it the day before; a date
  UNTIL is that day; what the form writes (the end of the day typed, in the
  instance zone) reads back as that day. `RecurrenceBlock.summary`, the
  form's `repeat.until` and the block's Until field agree.
- **`app-agenda/empty` is empty**: the fixture showed the week of 2
  November, and the weekly series falls on its Monday — the occurrence was
  right, the week was not («1 event», `loaded` 1, and a row). The fixture
  now shows 4–10 January 2027, after the series; its test checks the week
  holds nothing before comparing.

### For consumers

Fixtures regenerated: `app-agenda/empty` (another week), `app-event/normal`,
`app-event-edit/normal`, `/loss` (until 28 December).

## v0.2.116 — 2026-10-01

The range form of the agenda put the session's token in the address (the
client's scan of the calendar, Б6).

### Fixed

- **No GET form carries `_csrf`** (C-11): the agenda's `Range.form` (GET
  `/app/calendar…` with `view`, `from`, `to`) sent the session's token as a
  hidden field, so «Show» wrote it into the address — the history, the logs,
  the Referer of the next page. It is gone; the range never needed it, a GET
  changes nothing. `TestFixtureGetFormsCarryNoToken` walks every GET form of
  every fixture (red on `v0.2.115`: the four agenda fixtures).
- The range's To has no hint «Both days are in the list.» (s-agenda-1 draws
  none, as the timeline's since `v0.2.88`).

### For consumers

Put `_csrf` in a POST form only. Fixtures regenerated: `app-agenda/normal`,
`/empty`, `app-agenda-all/empty`, `/partial`.

## v0.2.115 — 2026-10-01

The mockup's copy is Typecase `29413d6`'s (Carrel's tasks 60–76): the frames
say the same in new markup, and the server's answers stand as they were.

### For consumers

Nothing in the running server; no view model changes. The frames the
server's checks read: a closed «⋯» is a `details` whose summary is its
button (s-contacts-6, task 60); «Compact rows» is a `tc-checkform`, a GET
form (s-contacts-1, task 67) — as `ListBar.compact` is since `v0.2.94`;
Attach with its arrow is a closed menu in the rubric (s-agenda-1, s-tasks-1,
s-notes-2, task 71) — as `attach.add` with `arrow` From this computer and
From Files is; the time zones of s-agenda-8 are radio buttons of the edit
form, `tz` and the zone (task 73) — as the row `tz` sends; the days of a
repeat are checkboxes (`Seg.Multiple`, task 72) — as the check row
`repeat.on` with an option a day is (the frame's field name `byday` is the
frame's, as `csrf` in components-42). `check.py`: the contacts' states
(`states-7` … `-12`) in `STATE_FRAME`; check 9 holds no bar tick's Apply
against an action and keeps `SERVER_TODO` («Tick every book»).

## v0.2.114 — 2026-10-01

Lines under titles and messages in the pieces the frames mark (C-6.2 Runs).

### Changed

- **`BackupPage.subtitle` is `Runs`** (was `Text`): a job whose destination
  is not WebDAV alone names it in bold — «Job · destination **WebDAV and
  download** · last copy …» (s-backup-3); WebDAV alone goes unsaid
  (s-backup-2).
- **`RestoreBlock.subtitle`** (new): the open archive — its job, what the
  manifest holds, where it lay monospaced (s-backup-8). **`RestoreBlock.note`
  is `Runs`**, «Restoring **creates** …».
- **`DavloomPage.subtitle`** (new): «One address for every device instead of
  an account for every server», and on a local address (loopback, private
  network, localhost, .local, .lan, .home.arpa) « · ‹danger›a phone on
  mobile data will not reach a local address» (s-davloom-1).
- **Public pages**: `lede` of a file is one monospaced piece «type · size ·
  changed day» (s-publish-9…11, was «Read-only. Published from Carrel.»); of
  a note, its day monospaced, with the time when it has one (s-publish-8).
- **`ConnectionsPage.note`** (new): with the panel of a subscription open,
  «A feed is read again on every visit … **Check** asks whether the address
  still replies.» (s-profile-10).
- **`IssuedPassword.message` is `Runs`**: the login in bold (s-admin-14,
  -17).
- **`MailSheet.replyNote` and `.limits` are `Runs`**: «Signed **name** ·
  replies go to …» (s-mail-1), «Published links» in bold.

## v0.2.113 — 2026-10-01

The settings and administration rails drew a state block at their foot that
the server did not send (components-35).

### Added

- **`Rail.status`** (C-6, `RailStatus`: `tone` — C-8 `StatusTone` `on`,
  `off`, `alert` —, `title`, `line` as Runs, the time monospaced «2026-08-16
  09:41»). Settings: davloom — off («No device connects until it is
  started.») or «davloom · running · N devices» with the last answer to a
  device since the start; the copies when there are jobs — «Backups are
  running», «Last backup … · verified», alert when the newest is older than
  a week or not whole, «No backup yet» before the first. Administration:
  mail — «SMTP not configured» (alert) or «Mail is going out» with the last
  letter sent since the start; the installation check — «Checks passed» or
  «N checks failed» with when, «Not checked since the start» before one.
  Other rails have no `status` key.

### Fixed

- The preview of a notes folder with no Markdown said only «The folder
  holds no Markdown file.»: the message before it was lost when the two
  were joined (v0.2.110).

## v0.2.112 — 2026-10-01

The head and the empty state of Backup by their frames (s-backup-1,
s-backup-6): Restore is found where the copies are, with or without a job.

### Changed

- **`BackupPage.actions`** (C-6): New job (primary, `?job=new` — the page
  of a job not made yet: «New job», the form, Save and Cancel) and Restore
  (`?restore=1` — the page with `restore.open`), on the list with jobs and
  without.
- **`BackupPage.subtitle` on the list**: «Three jobs · their own sources,
  destinations and triggers», «One job · …», «No jobs yet» (was empty).
- **`BackupPage.empty`, `.emptyNote`** (Runs): with no job, what a copy does
  (‹.ics›, ‹.vcf› monospaced) and that Restore needs no job (**Restore**).
- **`RestoreBlock.open`**: the restore is open — from Restore in the head,
  or with an archive open.

## v0.2.111 — 2026-10-01

A refused new collection left the sheet with Create only; the frame's foot
is the way on (s-profile-6).

### Changed

- **`CollectionNewSheet.actions` after a refusal** (C-6): «Back to the
  form» (primary) — POST of the form with `op=back`: the sheet again with
  what was typed and no refusal, nothing sent to the server — and «Try
  again», the same form sent once more (`collection.create`). A sheet
  without a refusal keeps Create alone. The same for a folder of a storage.

## v0.2.110 — 2026-10-01

The import wizard by its frames (s-io-3…9): what the steps, the head, the
counts and the report draw, the server now sends.

### Changed

- **`ImportPage`** (C-6): `steps` — «1 · Source», «2 · Preview», «3 ·
  Report», the current one by `step`; `subtitle` (Runs) — on the preview the
  file monospaced, its size and the format it was read as («vCard 3.0»,
  «iCalendar 2.0», «Markdown»), for a folder its name, size and how many
  files it held; on the report the collection the records went into;
  `counts` — «ready», and «with errors» (`error`), «UID collisions»,
  «skipped» when there are any.
- **`ImportReport`**: `failed` and `failures` — what was not written and
  why, a line a record (a file of a folder that could not be read, a write
  the server refused); `renames` — «old → new» for every UID that changed.
  `skipped` no longer counts what failed.
- **`ImportSource.note` is `Runs`** (was `Text`), in the frames' words: on
  the first step what the preview does, for notes what front matter brings
  with ‹X-› monospaced; on the preview what the import does with records of
  the kind. `hint` — the line under the file, the other kind's extension
  monospaced; `folderHint` — the line under the tree of folders.
- **Notes from a folder**: an option of `source.folder` is a GET that comes
  back with the folder chosen; `source.preview`, «Preview folder», takes it
  (POST, off with a reason until a folder is chosen). A file of the folder
  that is not Markdown is named and counted as skipped; one that cannot be
  read, as an error. A note without front matter is dated by the time the
  server gives its file.

## v0.2.109 — 2026-10-01

The line of a subscription card ended at «read 2 min ago»; the frames go on
with what the last read weighed and how long it took (s-profile-1, -10).

### Added

- **`SubscriptionCard.size` and `.took`** (C-6): «34 KB», «0.4 s» — the
  last answer of the feed read in this session: by its list, a record of it,
  search or Check. Whole kilobytes under a megabyte, bytes under a
  kilobyte; tenths of a second, milliseconds under a tenth. Empty when the
  feed has not been read in this session. Checking an address before
  subscribing notes nothing.

## v0.2.108 — 2026-10-01

The full form of a new contact drew no photo (s-contacts-27): the photo steps
were a card's, and before Save there is no card.

### Added

- **A photo for a new contact** (C-5.4, S-13). The new form's `photo` is a
  card's without a photo: Add and «JPEG, PNG, GIF or WebP.». Add posts the
  file to `/app/contacts/{account}/{col}/new/photo-preview?op=upload`, which
  issues the UID the card will have and keeps the upload in the session's
  photo draft under it; the frame steps are the card's, on that address with
  `uid=`. Save photo writes nothing: it goes back to `/new?uid=`, where the
  form shows the chosen frame with Replace, Crop and Remove and carries the
  UID in a hidden `uid`. Save of the form creates the card under that UID,
  with the photo as framed. Cancel of a first upload, Remove and a form left
  unsaved write nothing. The upload's caps, types and refusals are a card's;
  a `uid` not in the shape the server issues is not taken.
- A form whose `uid` names a photo the session no longer holds (another
  card's photo took the one draft) is not saved silently without it: the
  form comes back, «Nothing was saved», without the photo; saving again
  saves the card.

## v0.2.107 — 2026-10-01

Mentions (mockup 7.23) stay deferred until after the first working version;
what is ready for them, and what is to be settled when they are taken up,
is written down.

### Changed

- Nothing in the running server. docs/architecture.md, «Упоминания —
  подготовка»: what v0.2.101…v0.2.106 prepared and where Mentions plug in;
  the open questions — relinking of @-links when a UID changes, the form
  and case of `urn:uuid:`, a «group» kind, the timeline and `urn:`
  references, servers that do not keep links — are also in plan/server.md
  under the deferred item.

## v0.2.106 — 2026-10-01

«Mentioned» now belongs to Mentions (mockup 7.23): the timeline is not
described with it any more.

### Changed

- Words only: `TimelineRow` (C-6.5) is the records where the person is an
  attendee or the organiser, the records tied to their card by RELATED-TO,
  and files with their name — not records where they are «mentioned». The
  examples of `why` are the ones the server sends: «attendee · accepted»,
  «organiser», «related to this person», «name in the file name». The
  glossary of the client spec says the same of `Timeline`.

## v0.2.105 — 2026-10-01

Preparation for Mentions: the ways a record points to a person are a list.

### Changed

- Nothing a consumer sees. The timeline asks `timelineMatchers` in order,
  strongest first — the person's own line, a line they stand behind,
  RELATED-TO their card — each a `personMatcher`; the first answer is the
  row's Why, as before. A new way is one more matcher.

## v0.2.104 — 2026-10-01

Preparation for Mentions: the records this session has loaded are walked
in one place.

### Changed

- Nothing a consumer sees. `eachIndexed` walks every record of the loaded
  lists of the catalog, `indexedIn` those of one collection; the hints of
  the search box, the titles of links and the list of Related to read them
  there instead of three loops of their own. Every answer is the same, byte
  for byte.

## v0.2.103 — 2026-10-01

Preparation for Mentions (mockup 7.23, deferred until after the first
working version): a reference by UID is read in one place.

### Changed

- Nothing a consumer sees. `splitUIDRef` reads a value that names a record
  by UID — `urn:uuid:` in the case it was written, or a bare UID — and the
  import's relinking (`linkPatch`) uses it; the timeline and the relinking
  after a move or a merge are as they were.

## v0.2.102 — 2026-10-01

The GET of one card asked for no version (docs/servers.md, SV-1): sabre/dav
(Baikal, Nextcloud) answers it in 3.0 without what only 4.0 holds, though
every report already asked for 4.0.

### Fixed

- **A card read by its address asks for vCard 4.0**: `Accept: text/vcard;
  version=4.0`, as the reports ask by `address-data`. Such reads are the
  ones that find a record under another identity — after a create, a move
  or an import into a server that writes its own UID. A server that answers
  406 or refuses the version is asked again without `Accept`.

## v0.2.101 — 2026-10-01

A link in a note went wherever its text said: `[a](javascript:…)` reached
`Block.href` as written, on the note's page and on its public page `/p/`.

### Security

- **A link of a note goes only to `http`, `https`, `mailto`, `tel`** (C-6.9,
  C-11): `http` and `https` with a host. `javascript:`, `data:`,
  `vbscript:`, `file:`, a relative or scheme-relative address, a scheme in
  any case, behind `&#58;` or behind a space, a tab or a control character
  get no `href`: the link's words are plain text in the run of the
  paragraph. The address is read as Markdown itself writes it — character
  references resolved, spaces and the rest percent-encoded — so `href` of a
  link with such characters is now the encoded form.
- An e-mail autolink `<anna@example.com>` has `href` `mailto:anna@example.com`
  (it was the bare address, a relative link).

## v0.2.100 — 2026-10-01

The cross of the card in the timeline's panel went nowhere without scripts
(the client's stop C16).

### Changed

- **The timeline's card closes** (C-6.5 `ContactTimelinePage`, s-contacts-7):
  `card.close` is the same timeline — its range and kind kept — with
  `?card=0`; there `cardClosed` is true and `card` is empty: no panel, the
  screen keeps its sense. The screen's other links do not carry `card=0`
  and open the panel again. A new field, not `card` becoming a pointer: the
  client draws `card` as a value today.

## v0.2.99 — 2026-10-01

The pieces of the server's sentences the frames mark — a command or a day
monospaced, a count or the one word that matters in bold — in the sheets
and sentences the sections and the shell show (C-6.2 `Runs`, plan item
«Выделенные куски в других фразах сервера»).

### Changed

- **`Runs` instead of `Text`**: `MoveSheet.warning` and `FileOpSheet.warning`
  — `MOVE` monospaced (s-contacts-2, s-files-4); `CollectionMoveSheet.warning`
  and `CollectionDeleteSheet.warning` — the count in bold, «all **51**
  events» (s-profile-8, -9, -19); `CollectionDeleteSheet.archive` — the day
  monospaced and **new** in bold; `ConnectionsPage.subtitle` — «Signed in
  as **mix** · …» (s-profile-10); `DavloomPage.note` — «does **not**
  disconnect devices» (s-davloom-1).
- **`Details.date`**: a note's day and time in the panel, apart from
  `subtitle`, to be written monospaced after it (s-notes-1) — as the note's
  page keeps it in `NoteMeta.date`. `subtitle` is the connection and the
  notebook alone now.
- **A message's bold first sentence is its `Title`, with its stop**, as the
  frames draw it: «Nothing is loaded yet.» (s-dup-6), «One record is
  rewritten and 1 other deleted.» (s-dup-2, -4), «Written, but not every
  other record was deleted.» (s-dup-5).

### Not yet

- The settings' and the administration's rail status lines, hints of
  fields and a few sentences of copy and mail: plan/server.md, «Открыто».

## v0.2.98 — 2026-10-01

Strings the frames draw that the server did not send, or sent otherwise
(plan/server.md, two items «Открыто»). A constant word of a frame is the
screen's (U-1.3); what depends on the data or the state is sent.

### Added

- `FilesPage.attachments`: the folder is the attachment folder — the badge
  «attachment folder» under its title (s-files-1).
- `ConnectionCard.plain`: «plain WebDAV, no principal» for a storage whose
  server named no principal; `ConnectionCard.answered`: when the server
  last answered this session, the newest read of its collections — «5 min
  ago» after the screen's «last answer» (s-profile-1).
- `PublishedFile.changed`: the day the public file last changed,
  «2026-08-10» after the screen's «changed» (s-publish-9).
- `PhotoPreviewPage.saved`: what Save photo writes — «512 × 512 · JPEG ·
  about 90 KB», measured from the frame as it would be saved (s-contacts-5).

### Changed

- The note of the rename sheet is the frame's: «the address does not
  change … The new name and colour are written to the server — other
  clients will show them at their next sync.»; a book, without a colour,
  says the name alone (s-profile-7).
- A member of a pair on the duplicates screen says its day the short way of
  a list (C-10): «12 Aug 18:00», «12 Aug», a note's «12 Aug»; was «Due 12
  August 2026, 18:00» (s-dup-1).

## v0.2.97 — 2026-10-01

The top bar of a record's page had no refresh, while every frame draws
«2 min ago» there (the client's stop C17).

### Changed

- **A record's page and its edit carry the top bar's refresh** (C-6.3
  `Refresh`): `record.refresh`, a GET to the same page — which reads the
  record from its server —, and `answered`, when this session last read
  the record's collection: the same «2 min ago» as the source's «read 2
  min ago» (s-contacts-6, s-contacts-3, s-agenda-2, s-tasks-2, s-notes-2).
  The new record's form too.

## v0.2.96 — 2026-10-01

A POST outside a form could not be sent without scripts: nothing in the
view model gave the client the session's token (the client's stop C15).

### Changed

- **Every POST action of a screen behind sign-in carries the session's
  token** (C-6.2 `Action.CSRF`, C-11): Delete and Send of a card, Retry of
  a source, the photo's steps, «Tick every book», Sign out, the buttons of
  sheets and forms alike. Without scripts the client sends such an action
  as a form of its own with `_csrf` of that value; with scripts the header
  is as before. GET actions and the pages outside the shell carry none —
  their forms have their `_csrf`. The token is put in at rendering, for
  every action whatever handler built it; the rest of the view model is
  untouched.

## v0.2.95 — 2026-10-01

The primary buttons and the monospaced fields no fixture carried and no
frame test read (plan/server.md, the item «`Primary` и `Mono`…»).

### Changed

- **Primary as the frames draw it**: Upload of a folder's bar, read-only
  too (s-files-1, -2, -4, -15); Download of a public file (s-publish-9);
  Send by letter, now first on the sheet of a link just made
  (s-publish-13); Done under a device's password (s-davloom-3); Open the
  kept record after a merge that stopped (s-dup-5).
- **A public file that cannot be shown** says its size on the button:
  «Download 84.2 MB» (s-publish-11).
- **The import's buttons are the frames'**: Preview file (was «Read the
  file»), the first step's only button — no Cancel, the rail is the way
  out (s-io-3); «Import 122 contacts» (was «Import»; s-io-5…8); the report
  has Back to list alone (was «Open the collection» and «Import another
  file»; s-io-9). All three primary.
- **The restore's buttons are the frames'**: Open the manifest (was «Open
  the archive»), «Restore 3 collections» (was «Restore into new
  collections»), Cancel (was «Close the archive»); s-backup-7, -8.
- **Related to is monospaced** (`FormRow.mono`): what is typed after «By
  UID…» is a UID (s-tasks-11).
- C-6.2 and C-6.8 say which values outside a form the frames draw in a
  monospaced read-only field — links, passwords, addresses, weights — and
  that their look is the screen's constant (U-1.3), not `Mono`.

## v0.2.94 — 2026-10-01

What the client's stop of the contacts section (plan/client.md, C12–C14)
found the contract owing: buttons of the frames with nowhere to go without
scripts, a check sent as a link, a field the client had to name itself.

### Changed

- **`ForeignShow` is an action** (C-6.3 `Details`, C-6.5 `ContactPage`):
  «Show 3» with a link to the same card with `?foreign=1`, or to the same
  list with the same panel and `?foreign=1`; there it is `null` and the kept
  properties stay — `null` means none, or open. Was the label alone.
- **«Show N empty fields» has a way without scripts** (C-6.8
  `FieldSet.Unfold`): GET to the same form with `?fields=all`, where nothing
  is folded and every field stands in its rubric; the form posts back there
  and stays unfolded.
- **«Compact rows» is a check of a form** (C-6.4 `ListBar.Compact` is a
  `Form` now, s-contacts-1, components-38): GET to the list, the list's
  query hidden, a hidden `compact=0`, and the check `compact` with one
  option `1`, selected when dense. `?compact=` with several values takes
  the last. Was a link with `On`.
- **The search box names its field** (C-6.3 `SearchBox.Name`): `q`.

## v0.2.93 — 2026-10-01

The full form of a new contact has a frame of its own, s-contacts-27
(Typecase `4af7ad7`, from-typecase note 30, task 59).

### Changed

- **A new contact's full form starts with one empty phone row**
  (s-contacts-27, C-6.8): Label «mobile» (`cell`), Number empty, Remove,
  then Add phone. A record's form still carries no empty row beforehand
  (s-contacts-3); a row left empty is not written (S-7).
- `docs/spec/check.py`: `FRAME_TODO` «app-contact-new: нет кадра» is gone —
  check 8 holds `app-contact-new/normal` against s-contacts-27, the frame of
  its route. The caret of the new contact is held against s-contacts-27.

### Not yet

- The frame draws the photograph's Add on the new form; the photo steps are
  a card's, and the card does not exist before Save (plan/server.md,
  «Открыто»).

## v0.2.92 — 2026-10-01

What the mockup copy of from-typecase note 28 (Typecase `2b50f5e`,
`da558ec`, tasks 48–53) draws on the contacts card, the person's timeline
and a calendar the server made with less than was asked.

### Changed

- **A segment's count is apart from its label** (C-6.2): `Action.tally`
  — `label` «All», `tally` «24» where `label` was «All 24». The timeline
  (s-contacts-7) folds its kind filter on a narrow strip into a choice that
  writes the two apart (`SegFold`), and the client does not cut the
  server's strings. The same on the tasks' Open / Done / All and the
  duplicates' kinds. Not `count`: the rail's rows (`NavRow`, `Shortcut`)
  already have a `count`, a number.
- **The timeline filters by `?kind=`** (C-5.4): `event`, `task`, `note`,
  `file`, as the frame's links write it; All is the timeline without it.
  The segments are `timeline.kind.all`, `timeline.kind.event`, …;
  `?show=events` and `timeline.show.*` are gone.
- **A phone's link is digits and the plus** (s-contacts-6): «+7 912
  445-10-88» goes to `tel:+79124451088`, not `tel:+7912445-10-88`. A number
  already written as a `tel:` URI keeps its form; one with an extension or a
  letter has no link.

### Added

- **«New calendar» on the notice of what the server did not take**
  (s-agenda-11, SV-52): `Notice.actions` holds `collection.new`, GET
  `/app/collections/new?kind=event&a={account}&components=VTODO…` — the
  sheet of a new calendar on this server with what was not taken ticked.
  The notice's title ends with a full stop, as the frame writes it.
- **`/app/collections/new?components=`** (GET, C-5.6) ticks the parts of
  «What goes in it»; unknown values are ignored.

### For consumers

Read `Action.tally` after the label of a segment; build nothing from
`label`. The card's «⋯» (s-contacts-6, -7) is `ContactPage.menu` and
`Details.menu` as before — the frames now draw it closed with its items
(`Drop`); the range of the timeline and the agenda is `Range.form`, a GET
form with `from` and `to`, as before (`Range.Action`). Fixtures changed:
`app-contact-timeline/normal`, `app-contact/normal`, `app-contact/outcome`,
`app-contacts/normal`, `app-duplicates/normal`, `app-tasks/normal`,
`app-tasks-all/empty`.

## v0.2.91 — 2026-10-01

Two false loss reports on Google go away (docs/servers.md, SV-56).

### Fixed

- **An end the start implies is not lost when the server leaves it out**
  (S-7.1, S-11). Since v0.2.80 an event the person gave no end is written
  with `DTEND` equal to `DTSTART` (SV-53), and Google keeps such an event
  without the `DTEND`: a new event there came back as its form with «The
  server did not keep … DTEND» instead of opening. RFC 5545 §3.6.1 ends a
  timed event without an end at its start, and a dated one a day later; a
  `DTEND` that says just that, left out by the server while `DTSTART` came
  back as sent and neither `DTEND` nor `DURATION` is there, is no loss. An
  end in another zone, or a longer one, still is.
- **A `PREF` the server adds is not a loss.** Google marks the first phone,
  address and e-mail of a card `PREF`: a contact made with a phone and an
  e-mail came back with «The server did not keep EMAIL, TEL». A `PREF` among
  the `TYPE` values that was not sent is left out of the comparison; a
  `PREF` sent and not kept is still reported.

### For consumers

Nothing in the contract changed. A yearless birthday on Google is still
reported lost — Google drops `--MMDD`; whether to write it in Apple's form
(`1604-…;X-APPLE-OMIT-YEAR=1604`) is the owner's call (SV-56).

## v0.2.90 — 2026-10-01

The binary answers with the client: a page whose view model has a screen is
HTML, the rest is JSON as before. Typecase's files are served from the
binary.

### Added

- **The client in `cmd/carrel`**: `web.New` gets the client's renderer
  (`carrel.dev/carrel/client/renderer`). A page whose view model has a
  screen — every page outside the shell: sign-in, registration, first run,
  forgotten password, the wipe, the invitation, the address's confirmation,
  the dead link, About — is the screen's document, `text/html`, with the
  status as before. Every other page, every fragment but `main` of such a
  page, the out-of-band updates and the events are the same JSON as
  `render.JSONRenderer`'s, byte for byte.
- **Static files (C-5.9)**: `GET /static/tc/{version}/css/typecase.css`,
  `…/fonts/{file}`, `…/js/theme.js`, `…/js/tc.js`; `{version}` is 12 hex
  digits of the SHA-256 of their contents. Found — `Cache-Control: public,
  max-age=31536000, immutable`; another version, a directory, an unknown
  name — 404 `no-store`. The headers of every answer (CSP, `nosniff`, `DENY`,
  `same-origin`) as they were; not metered by the session's bucket.
- **`web.Server.Static`** — mounts files from the binary at a prefix; only
  `cmd/carrel` calls it.
- **`scripts/linux-ship.sh`** — packs this repository at HEAD and the
  Typecase clone at the commit of «Typecase сборки» (docs/plan/client.md)
  side by side for the Linux machine.

### Changed

- **`go.mod`** requires `carrel.dev/carrel/client` (`replace => ../client`)
  and repeats the client's `replace carrel.dev/typecase =>
  ../../../typecase`: a dependency's replaces do not apply here. Building
  the server needs the Typecase clone beside the repository.
- **The image** is built from the directory above the repository, which
  holds `carrel/` and `typecase/`:
  `docker build -f carrel/carrel/server/Dockerfile -t carrel .`;
  `Dockerfile.dockerignore` (BuildKit) lets in the server, the client and
  Typecase only.
- **`scripts/linux-gates.sh`** wants the Typecase clone beside the
  repository, builds the image from there, mounts it for the race detector,
  and the smoke fetches `/login` (HTML) and the three files it names from
  the image.
- **`nodep_test.go`**: no package of the module but `cmd/carrel` depends on
  the client or on Typecase, directly or not; `cmd/carrel` does.

### For consumers

The pages outside the shell answer HTML: whatever read their JSON from the
binary reads HTML now (the fixture checks build their own server on
`render.JSONRenderer` and are unchanged). The pages in the shell, their
zones, the sheets and the events are JSON still. Build the image from the
parent directory with the Typecase clone beside the repository.

## v0.2.89 — 2026-10-01

The form of a contact as the frames draw it (the client's scan of the
contacts section, group Д).

### Added

- **`ContactEditPage.name`** (the person; «New contact» for a new one) and
  **`.subtitle`** (the book) — the form's head (s-contacts-3).
- **`FieldSet.folded`** — the set is folded under its legend: «Show N empty
  fields», the last of the form.
- **Fixture state `no-photo`** (C-14.3): `app-contact-edit/no-photo` — the
  form without a photograph, Add and the formats (s-contacts-4);
  `app-contact-edit/normal` now has Анна's photograph: Replace, Crop,
  Remove, «EXIF orientation applied, metadata stripped.» (s-contacts-3).

### Changed

- **No hints under Display name and Tags** of the contact's form (the frame
  draws none); the order Display name is cut in is said on the quick sheet.
- A kept photograph's `PhotoBlock.note` says what was done to it (S-13).

### For consumers

Fixtures regenerated: `app-contact-edit/normal`, `/loss`, `/no-photo` (new),
`app-contact-new/normal`.

## v0.2.88 — 2026-10-01

A person's timeline as the frame draws it (the client's scan of the
contacts section, group Г).

### Added

- **`ContactTimelinePage.subtitle`** (where and as what) and **`.linked`**
  («in 2 books», `linked`; `null` for one book).
- **`Card.avatar`** — the head's avatar and the panel's photograph;
  **`Card.books`** — the other books; **`Card.source.uid`**.
- **Icons** of Write a note (`i-quill`), New event (`i-cal`), New task
  (`i-task`) and **Print** (`timeline.print`, `i-print`, `?print=1`).
- **`TimelineGroup.count`**.
- **`RunMark` `danger`** (C-8): a source that did not answer, as an alarm.

### Changed

- **`PollBar.summary` is `Runs`** everywhere: a sentence without marks is one
  piece. A timeline's is in pieces — «Polled 13 of 14 sources · matched by
  ‹mono›a.kovaleva@example.org‹/mono› and the name · ‹danger›baikal · Общий
  unavailable‹/danger›» (s-contacts-7).
- The timeline's card has no Organisation and Title rows (the head says
  them); its tags lead to the filter and its birthday has `aside`.
- The range's To has no hint (the frame draws none).

### For consumers

Read `poll.summary` as pieces (C-6.2). Fixtures regenerated: every fixture
with a poll, `app-contact-timeline/normal`.

## v0.2.87 — 2026-10-01

The card of a contact as the frames draw it (the client's scan of the
contacts section, group В).

### Added

- **Print on the card** (`record.print`, `i-print`, the card with
  `?print=1`); Edit has `i-pencil`.
- **`Field.aside`** — dim after a value: «in 7 months» by a birthday.
  **`Field.tags`** — the row «Tags» of a record's fields carries its tags
  (`value` empty). **`Field.colour`** — the mark of the book or calendar of
  a linked member's row (`ContactPage.books`, `*Page.linked`).
- **`ContactPage.foreignShow`**, **`Details.foreignShow`** — «Show 3», the
  button that opens the kept properties.
- **`ContactPage.timelineCount`** — how many records the timeline of this
  session found (the number in the inset's rubric); `more` has `i-all`.
- **`TimelineRow.colour`** — the mark of the record's collection.
- **`SourceBlock.path`** (the record's path on its server),
  **`SourceBlock.compared`** («not changed on server», «changed on server
  since» — against the version the list of this session read).
- **`Details.books`** for the timeline's card (filled in v0.2.88).

### Changed

- **Categories are a row of the fields** where the frames draw them: a
  contact's between Birthday and Note, the last row of an event's, a task's
  and a note's panel and card. `ContactPage.tags` and `Details.tags` are
  gone.
- **Phones and a birthday are `mono`** on the card and in the panels.
- A linked member's row says why capital first: «Phone, email differ».

### For consumers

Read the tags from the field «Tags». Fixtures regenerated: the card with
Анна's photograph and the timeline's inset (`app-contact/normal`,
`outcome`), the panels and cards of every kind (tags), `app-contacts/normal`
(its panel). `check.py` draws `app-contact/outcome` into `s-contacts-16`.

## v0.2.86 — 2026-10-01

The list of contacts as the frames draw it (the client's scan of the
contacts section, group Б).

### Added

- **`ContactsListPage.subtitle`** — «3 of 4 books · 2 accounts · 312 cards»
  (s-contacts-1; the cards once the poll is over), «nextcloud · 248 cards»
  (s-io-1), «radicale · vps · the server granted no write access»
  (states-4); **`.readOnly`** for the mark by the title.
- **`ContactsListPage.empty`** — what a list with nothing to show says, with
  its button (U-6.0): nothing ticked — «Tick every book» (and `poll` is
  `null`); an empty book or empty ticked books — New contact and Import.
- **New's arrow in every combined view** (s-contacts-9): the ticked
  collections of the view, each `record.new` into it; `MenuItem.colour`,
  `.aside` («last used» — the button writes there), `.readOnly` (closed with
  its reason; the client draws «ro»).
- **Print** of a list of contacts: `list.print`, `i-print`, the list with
  `?print=1` (C-5.4, U-10).
- **The panel without scripts**: `?open={a row's href}` on a list of
  contacts answers with `Shell.details` of that card, the row `on`
  (`ContactRow.on`), `Details.close` — the list without the panel.
  `SourceBlock.uid` — the panel's «UID …».
- **`ContactGroup.count`**, **`Columns.icon`** (`i-cols`).
- **`ListBar.filterForm`** — the filter without scripts: GET to the list with
  `filter` and the kept `task`, `sort`, `tag`; the server keeps the cards
  whose name, phone, address or organisation holds it.
- **«Compact rows»**: `ListBar.compact` is an action now (`On`; a link with
  `compact=1` or `0`); the choice is this browser's, cookie
  `carrel_compact_contacts` (S-9.6).
- **Icons** of New (`i-plus`) and Import (`i-up`) in every list's head.

### Changed

- **`ListBar.compact`** was a bool nobody set; it is `null` where a list has
  no switch.
- The rail's form takes `toggle` from the query too: the empty list's button
  is a plain POST.

### For consumers

The run bar of a poll stands while the poll runs or a source did not
answer; «Loading more…» while the poll runs (C-6.4). An empty cell is the
screen's «—». Fixtures regenerated: every list (icons, `columns.icon`,
`compact`), `app-contacts/normal` with Анна in the panel, the states of the
contacts lists with `subtitle` and `empty`.

## v0.2.85 — 2026-10-01

The shell as the frames draw it (the client's scan of the contacts section,
30.09.2026, group А): «New note» with its arrow, icons, the account
button's way without scripts, the open collection and feeds in the rail,
the name and the build apart, the davloom mark, and ticks that send
themselves.

### Added

- **`TopBar.new`** — «New note», primary, icon `i-quill`, and its arrow as
  s-notes-7 draws it: Note (the button again), past a line Contact, Event,
  Task, past a line Upload file, past a line Letter where a letter can go.
- **`TopBar.userHref`** — where the account button leads without scripts:
  `/app/settings/account`.
- **`Rail.ticks`** — the rail's form: POST `/app/sources/{kind}` with
  `_csrf` and `back`. A tick is a submit button of it and sends `toggle`
  with its row's `href`; the server flips that one collection — the
  combined row ticks every one, or takes them all off when all are ticked —
  and leads back to `back` (a page of the section). `nil` on back rails and
  in the settings. The whole selection as `tick` values still works.
- **`Rail.davloom`** — the words of the davloom mark at the rail's foot,
  «davloom · running · 3 devices»; empty while the mode is off.
- **`SourceRow.on`** (the open collection: the combined row on a combined
  view, the book of a list or of a record, «Combined calendar» on a
  timeline) and **`SourceRow.feed`** (a subscription, drawn as a feed).
- **Icons**: every section of `Rail.nav` (`i-con`, `i-cal`, `i-task`,
  `i-note`, `i-file`, `i-dup`), `Refresh.action` (`i-refresh`).

### Changed

- **`Rail.about` is `RailAbout`**: `label` «Carrel» and `build` «0.0.1 ·
  786949a» apart (the frame sets the build in mono).
- **Apply left the rail's foot** (`rail.apply`): the frames draw none; the
  ticks send themselves.

### Removed

- **`TopBar.newMenu`** — it is `TopBar.new.arrow` now.

### For consumers

Read `top.new` instead of `top.newMenu`, `rail.about.build` beside its
label, and send a tick through `rail.ticks`. The fixture set now has a relay
with letters from people allowed and davloom on, as the frames draw them:
«Letter» in the arrow, Send on a card, enabled mail buttons in the
administration. Every fixture of the shell is regenerated.

## v0.2.84 — 2026-10-01

An import into an iCloud calendar writes the records whose UID another
calendar of the same account holds (docs/servers.md, SV-55).

### Changed

- **A record of an import gets a UID of its own where the account holds
  one record per UID** (SV-55, S-19; the owner's rule for a copy,
  30.09.2026). iCloud answers PUT of a record whose UID another calendar of
  the account holds 412 under any name, and such a record ended under «Not
  created» with the 412. A 412 on a name nothing answers at is now read as
  «the UID is taken»: the record is written once more under a fresh UID, its
  row says «Created under …: this server holds one record per UID across
  the account, and another of its collections holds this one», and it
  counts among «under a new identity». Records of the same file that point
  at it (`RELATED-TO`) follow it: those not yet written at once, those
  written before it by an edit after the last. The session remembers the
  account, as a copy does.

### For consumers

Nothing in the contract changed: the report's rows and counts say it.

## v0.2.83 — 2026-10-01

Two dead fields leave the contract.

### Removed

- **`InvitePage.valid` and `ConfirmEmailPage.ok`.** Both have been true in
  every answer since v0.2.76: a dead secret answers with its own page
  (`DeadLinkPage`, rest-1), never with these pages marked invalid.

### For consumers

Stop reading the two fields. Fixtures regenerated: `invite/normal`,
`invite/link`, `confirm-email/normal`.

## v0.2.82 — 2026-10-01

Columns in every list that draws them (from-typecase note 25, the owner's
decisions of 29–30.09.2026): tasks, notes, files, users, the audit log and a
person's timeline; Source only in a combined view; sorting follows the
columns.

### Added

- **`ListBar.columns` on the lists of tasks (s-tasks-12), notes
  (s-notes-23), files (s-files-20), a person's timeline (s-contacts-25),
  users (s-admin-21) and the audit log (s-admin-23, also in the bar of
  s-admin-7).** The same form as the contacts': `col` in order to
  `…/columns` (`/app/tasks/columns`, `/app/notes/…/columns`,
  `/app/files/{account}/{col}/columns/{path…}`,
  `…/{uid}/timeline/columns`, `/admin/columns`, `/admin/audit/columns`),
  `col=default` resets, a cookie per section (`carrel_cols_tasks`,
  `_notes`, `_files`, `_timeline`, `_users`, `_audit`; S-9.6). The
  administration's forms are an administrator's.
- **Row fields for every column the menus offer** (C-6.4 maps columns to
  fields): `TaskRow.priority` («5»), `.status` («In process»; «Needs action»
  without STATUS), `.changed` («10 Aug 11:02»); `NoteRow.changed`;
  `AdminUserRow.address` — the confirmed address, empty without one
  (s-admin-22); `AdminUsersPage.bar`, holding Columns for now.
- **Sorting follows the columns** (s-tasks-13). A sort shows its column
  («Recently changed» — Changed, «By priority» — Priority, «Newest» and
  «Oldest» — Date, files' «Size» — Size). A choice that takes away the
  column the list is sorted by leads to the list without `sort` and with
  `unsorted=<the order>`, and the list says so: `info` notice «Back to the
  default order». The mark leaves the address; no link of the page carries
  it.

### Changed

- **Source only in a combined view's menu.** One collection of contacts,
  tasks or notes names its source in its head, so its menu has no Source
  item (components-42 is now the combined contacts' form). A choice made in
  one collection keeps Source where the section's choice had it.
- **A timeline row's `when` is a list's day**, «10 Aug» (s-contacts-7, -26;
  C-10), no longer «10 August 2026».

### For consumers

Draw the columns a list's `bar.columns` ticks, in the order of its items;
the rows carry every column in any choice. Fixtures regenerated:
`admin-audit`, `admin-users`, `app-contact-timeline`, `app-contacts-one`,
`app-notes`, `app-notes-all`, `app-tasks`, `app-tasks-all`.

## v0.2.81 — 2026-09-30

A copy of a record within one iCloud account is written (docs/servers.md,
SV-55), and an End time typed without an End date is kept (S-11).

### Changed

- **A copy gets a UID of its own where the account holds one record per
  UID** (SV-55, S-18; owner's decision). iCloud answers PUT of a record
  whose UID another calendar of the account holds 412 under any name, and
  «Copy here» ended in «Nothing was copied». A 412 on a name nothing
  answers at is now read as «the UID is taken»: the copy is written once
  more under a fresh UID, and the original's page says so — `alert`
  notice «Copied, with a UID of its own», for other apps another record,
  and `record.open` «Open it there» to the copy. The session remembers the
  account: the next copy within the same connection goes with a fresh UID
  at once, and its sheet's `warning` says so before it starts. A server
  that answers 412 for a taken name goes on to `{uid}-2` as before; moves
  are unchanged.
- **End time without End date** (S-11, s-agenda-3): the full form reads
  it by the quick sheet's rule — the start's day, or the next day when it
  is earlier than the start's time. Before, the time was dropped and the
  event ended as it began. An edit that clears the End date and keeps a
  time reads the same way.

### For consumers

The copy's answer on such an account is `200` with the original's record
page and the notice above instead of a `303` to the copy. Nothing else in
the contract changed.

## v0.2.80 — 2026-09-30

Three iCloud calendar quirks answered (docs/servers.md, SV-52, SV-53,
SV-54): records open there, events without an end are saved, and a
calendar the server made without some of what was asked says so.

### Changed

- **A record is found without the UID filter where the server refuses
  it** (SV-54). iCloud answers a `calendar-query` with a `text-match` on
  UID 412 with an empty body, and no event or task opened there — card,
  edit, Mark done, move, delete. A 400, 403, 412 or 501 to that filter now
  sends the lookup round: `GET <collection>/<UID>.ics`, then the component
  read whole through the session's cache (S-4.2) with the UID compared
  here. The session remembers the refusal per collection and asks the
  filter no more.
- **An event is always written with an end** (SV-53, S-11). iCloud answers
  PUT of a `VEVENT` without `DTEND` and `DURATION` 404. When the person
  gives no end, `DTEND` is the start itself for a time and the next day
  (`VALUE=DATE`) for a date — the same event by RFC 5545 §3.6.1 — on the
  full form, the quick sheet and an edit that clears the end. An event
  another client wrote without an end keeps having none when edited in
  something else.

### Added

- **A notice on a calendar the server made without some of what was
  asked** (SV-52). iCloud keeps the first component of an `MKCALENDAR`
  set and says nothing of the rest. The agenda, tasks and notes lists of
  that calendar now lead with an `info` notice: «The server did not take
  tasks» — what it kept, and that tasks need another calendar. No second
  calendar is made on the person's behalf. No frame draws this notice yet
  (plan/to-typecase.md, task 53).

### For consumers

`Shell.notices` of a calendar's list may carry this `info` notice for the
rest of the session after the calendar was created. Nothing else in the
contract changed.

### Not yet included

- **Copying a record within one iCloud account** (SV-55): iCloud holds one
  resource per UID in the whole account and answers PUT of the copy 412.
  «Copy here» says «Nothing was copied» with the 412; moving works.

## v0.2.79 — 2026-09-30

What the client stopped on (plan/client.md, «Что блокирует»), decided by
the lead: a sentence of the server with marked pieces, the monospaced
field, the primary button, and the invitation by letter as the fixture of
its frame.

### Changed

- **`Plain.lede` is runs** (C-6.2 `Runs`, `Run{text, mark}`, C-8
  `RunMark`: `""`, `mono`, `strong`) on every page outside the shell and
  every public page. A sentence with nothing marked is one run, an empty
  one none. Marked now: the address the letter went to, monospaced
  (s-auth-3); the login, bold (s-auth-6, s-auth-10) and monospaced
  (s-auth-11).

### Added

- **`Action.primary`** (C-6.2, omitted when false) where the frame draws
  the button `is-primary`: the submit of the forms outside the shell,
  password, unlock, deposit, backup job, connection and its edit, Create
  user; Sign in of rest-1 and rest-2; the first card of s-auth-6; «New …»
  of the lists and the empty lists; Edit in a card's head and a note's bar
  (not in the details panel or a row's «⋯»); Save of the record forms and
  the quick sheets, Save of instance settings and of duplicate weights;
  the confirms of sheets — Move here, Copy here (records, collections,
  files), Create, Save, Create link, Rewrite N records, Write this
  combination, Save photo, Send; Test discovery, Run the check, Create
  link and Add user of the administration; Connect account and Subscribe;
  Create password of a device; Apply of Columns; Attach and Choose this
  folder of the file pickers, Upload here of the upload picker; Create of
  a new folder's line; Copy and Done of a password handed over.
- **`FormRow.mono`** (C-6.8, omitted when false) where the frame draws the
  field `is-mono`: Login at the first start (s-auth-4); dates, times,
  Every, Until and Advanced RRULE of events, tasks, notes and the quick
  sheets; Birthday, the phone numbers (and their template) and Postcode;
  Percent complete; the ranges of the agenda and the timeline and of a
  calendar's publication; Server URL and Certificate fingerprint of a
  connection and of the DAV check; Feed address; a new collection's
  Address; Folder, Keep copies and the days of a backup job; Folder in it
  of attachments; the numbers, Host, Port and Public address of the
  instance settings; temporary passwords (Add user, Reset, escrow
  recovery); the device's password, Address and User name (s-davloom-3).
- **`invite/link.json`** — the invitation by link, no address field. State
  `link` (C-14.3); no frame of its own (the caption of s-auth-12), so no
  `STATE_FRAME` row.
- **check.py 9** — every fixture against its frame: an action whose words
  the frame draws as a button is primary exactly when drawn `is-primary`, a
  row whose label the frame draws as a field is `mono` exactly when drawn
  `is-mono`.

### Fixed

- **`invite/normal.json` is the frame's case** (s-auth-12): an invitation
  by letter to anna@example.org — Email shown and switched off, the line
  under the title the letter's.

### For consumers

- Write `lede` run by run: `mono` in the monospaced face, `strong` bold, the
  rest plain. Do not cut or join the server's words beyond laying the runs
  side by side.
- Draw the button with `primary` as the primary one; the order of
  `actions` does not make one primary. Draw the field with `mono`
  monospaced.
- `invite/normal.json` now carries the Email row; the link case is
  `invite/link.json`.

## v0.2.78 — 2026-09-30

The mockup of Typecase `f86a3f0` (frames of chapters 8–12 renamed, owner's
decision): the caret stands where the frames draw it in every form the
server builds, and the steps of the pages outside the shell that have
frames of their own have fixtures.

### Added

- **Fixtures of the steps outside the shell** (C-14.3, named by the
  frame's `data-part`, drawn into it by `STATE_FRAME` of `check.py`):
  `register/sent.json` (s-auth-3), `forgot/options.json` (s-auth-6),
  `forgot/no-mail.json` (s-auth-7), `forgot/wipe-sent.json` (s-auth-8),
  `forgot/asked.json` (s-auth-9), `forgot-token/password.json`
  (s-auth-11). New states `sent`, `options`, `no-mail`, `wipe-sent`,
  `asked`, `password`.
- **`confirm-email/normal.json`** — `/confirm-email/{token}` answered with
  the address confirmed (rest-2). The route has no whole-screen frame, so
  its `normal` is held against rest-2 by `STATE_FRAME`; its dead token is
  the shared `dead-link`.

### Changed

- **`FormRow.focus`** is set on every form a frame draws with a caret, when
  the form is shown: the first field that came back with a reason, else the
  frame's field — Login of registration, setup and invitation (s-auth-2,
  -4, -12; after a login the invitation names, Password), Login or email
  (s-auth-5), Password of the wipe's second step (s-auth-11), New password
  (account-3), the password of the unlock and deposit sheets (s-plan-3,
  account-2), Server URL (s-profile-2), Feed address (s-profile-10), Name of
  a new or renamed collection or folder, of a device and a backup job,
  the name or login typed to confirm (s-profile-9, -19, s-admin-13), the
  new folder's name (s-files-13), the first field of a record's form and of
  a quick sheet — a note's text on its quick sheet (s-notes-5).
- The unlock sheet's field is **«Your Carrel password»** (s-plan-3,
  s-profile-3), not «Password».

### For consumers

- Put the caret in the field with `focus` whenever the form is shown —
  also a sheet, a dialog or a panel opened over the page. In a note's edit
  it is in the text (`NoteDoc.body`, s-notes-20), in a letter in To
  (s-mail-1): neither is a form row.
- Fixtures that gained `focus`: the edit and new forms of records, their
  `loss` states, `app-quick`, the collection sheets, `app-files-picker`,
  `app-password`, `app-settings-backup`, `-davloom`, `-escrow`, `-unlock`,
  `forgot`, `invite`, `register`, `setup`.

## v0.2.77 — 2026-09-30

The mockup of Typecase `784599e` (tasks 46 and 47): the sign-in refused
has its frame, s-auth-13, and the sign-in page says where the caret stands.

### Added

- **`FormRow.focus`** (omitted when false) — the field the caret stands in
  when the page arrives, one per form: Login on the empty sign-in page
  (s-auth-1), Password after a refusal (s-auth-13). No other form sets it
  yet.
- **Fixture state `refused`** (C-14.3) — a form sent back with a refusal:
  what was typed in place, the reason at its field, the caret in it.
  `login/refused.json` draws into s-auth-13.

### Changed

- Nothing in what a refused sign-in answers but the focus: the same page,
  status 200, the login back in its field, the password empty with «Login
  or password is wrong.» at it — the same for an unknown login, a wrong
  password and a disabled account (S-9.5, S-29.1). Now written down in
  S-9.5 and drawn by s-auth-13.

### For consumers

- A client puts the caret in the field with `focus`; `login/normal.json`
  carries it on Login.

## v0.2.76 — 2026-09-29

Notes 23 (the choice of columns — whole, the owner's decision) and 24 of
Typecase (`86e42e6`, `b64895e`, `eb0822f`): the Columns menu of the
contacts lists is a form the server keeps for this browser, and one page
answers any dead secret link.

### Added

- **`ListBar.columns`** (`Columns`: `label`, `form`, `items` of
  `ColumnItem` — `id`, `label`, `on`, `fixed`, `aside`) — the Columns menu
  (components-42) of `/app/contacts` and `/app/contacts/{account}/{col}`;
  null on every other list. The items stand in their order: the columns
  shown, then the others; until the person chooses, the order and the ticks
  of components-21 — Name («always», `fixed`), Phone, Email, Organisation,
  Birthday, Tags, Changed, Source, the last three before Source off.
- **`POST /app/contacts/columns`, `POST
  /app/contacts/{account}/{col}/columns`** — the menu's form: `col`, the
  ticked columns in the order they come; `col=default` resets. The answer
  is a redirect to the list with its query; the choice is kept by this
  browser in the cookie `carrel_cols_contacts` (a year, HttpOnly, SameSite
  Lax), common to the combined book and every book (S-9.6).
- **`ContactRow.birthday`, `.tags`, `.changed`** — the columns the menu
  offers beside phone, e-mail and organisation: «14.03.1987», «02.11» without
  a year; the card's categories as tags that filter the list; the day of
  the last change the short way, «12 Aug» (s-contacts-23, -24).
- **`DeadLinkPage`** (`title`, `lede`, `signIn`) — «This link does not
  work» — «It may have expired, been used already, or been withdrawn. Ask
  whoever sent it for a new one.», one button Sign in (rest-1). Fixture
  `dead-link/normal.json`.
- **`PublishSheet.local`** — «local» in the head of the publish sheet
  (s-publish-2), the same flag as `top.signed.local`.

### Changed

- **A dead secret link answers with `DeadLinkPage`, status 404** — an
  invitation, a wipe link, an address confirmation, a publication, used,
  revoked, expired or never issued, GET or POST; the backup trigger's
  `/api/backup/` too. Before, the answer was the plain text «404 page not
  found». It is still one answer: the same status, headers, body and delay
  whatever the link was (audit №3); the anonymous bucket is unchanged.
- **A letter that did not go is an alert** (`notices`, role alert, the
  server's line as its body): the options of `/forgot` again after
  «Send a confirmation letter» (s-auth-6), to try once more or to ask;
  after registration the page of its letter with one door back (s-auth-3).
  Before, `/forgot` said it as the lede of another page and registration as
  its lede.
- **The erase page's words** (s-auth-10): «Confirming erases what Carrel
  keeps encrypted for mix: saved connections and their passwords,
  subscriptions, published links, devices, external backup triggers and the
  deposited copy of the key.» and «The account itself stays: same login,
  role and confirmed address, the same first day of the week and clock.» —
  what the wipe did already.

### For consumers

- Every list's `bar` carries `columns` (null but on the contacts); the
  fixtures of the lists, the timeline, the search, the duplicates and the
  journal changed with it, the contacts' rows with the three columns.
- A client that shows a dead link reads the 404's body as `DeadLinkPage`.

## v0.2.75 — 2026-09-28

Copies the Typecase mockup of note 22 (Typecase `8764758`) and brings the
pages outside the shell to their frames (U-6.14): sign-in, setup,
registration, the invitation, forgotten password and its link, the confirmed
address, «about», and the password change.

### Added

- **`SignedIn.local`** (`top.signed.local` of every page in the shell) — the
  instance is local: the addresses it hands out answer only from inside its
  own network. The badge «local» stands at davloom (s-davloom-1), the
  published links (s-publish-1), a device's link (s-plan-1) and «On a clock
  of your own» (s-backup-2).
- **`BackupJob.clockHint`** — under «On a clock of your own», ticked or not:
  «Turning it on stores this job's data key under the server's key — less
  protection, for convenience: the job can then read without you signed
  in.» (S-24.4).
- **`LoginPage.tagline`, `AboutPage.tagline`** — the second line of the
  tagline, after `lede`: «Calendars, contacts, tasks, notes and files — one
  place.»
- **`LoginPage.register`** — «Create an account», only while registration is
  open; null when it is closed.
- **`RegisterPage.signIn`** — «Already have an account? Sign in» at the form,
  «Back to sign in» after it went. The form asks **Repeat password**; a
  pair that differs registers nobody and is said at the repeat.
- **`ForgotPage.back`** — «Back to sign in» in every state;
  **`ForgotPage.options`** are cards (`ForgotOption`: `title`, `body`,
  `form` of one button that carries `op` and the login as typed).
- **`ForgotWipePage.cancel`** — «Cancel» of the first step, to sign-in.
- **`AboutPage.licence`** — the AGPL line.
- **`PasswordPage.note`** — the line under the fields: «Your data stays
  encrypted under the same key; only the wrapping changes. Saved connections
  keep working.»; **`PasswordPage.back`** — «Back to settings», null after a
  temporary password: there is nowhere to go before the change.

### Changed

- **`/forgot` answers a login or an address with what can be done**
  (s-auth-6) — «Wipe the account» with «Send a confirmation letter», and
  «Ask the administrator» only for an account that deposited its key — and
  does it on the option's button: `op=wipe` sends the letter («Check your
  inbox», s-auth-8), `op=ask` records the request as the journal line
  `user.forgot.ask` («The administrator has been asked», s-auth-9). Before,
  the first post sent the letter at once, and an account that deposited its
  key was only «asked», with nothing recorded and no wipe offered. Nothing
  to do — an unknown login, an unconfirmed address, a disabled account — is
  «We cannot help from here» (s-auth-7). The page names the account by what
  was typed, never by a login found for an address.
- **The first step of `/forgot/{token}`** is its button (s-auth-10): there
  is no word to type. «Erase and set a new password» is `danger`; opening
  the link still wipes nothing. The second step's fields are Password and
  Repeat password with the password warning over them (`notices`); a
  refusal is said at its field, not at the hidden CSRF row.
- **The texts of the pages outside the shell are the frames'**: «Set up
  Carrel» with the warning, the login's sample «a-z 0-9 . _ -» and the dash
  of the optional address (`placeholder`); «Create an account» with its line
  and the warning; «Check your email» with the address the letter went to;
  «Forgotten your password?», «What we can do», «We cannot help from here»,
  «Check your inbox», «The administrator has been asked»; «Erase everything
  and start again?», «Set a new password»; «Email address» — «Address
  confirmed. You can sign in now.»; «Carrel» on «about» with «Third-party
  licences».
- **«About»'s button** — «Back to sign in» without a session; from the
  shell «Back» to the page of the shell one came from (the `Referer` of
  this origin under `/app`, else the first screen).
- **The trigger card's note** no longer says what turning it on cost: the
  hint under the tick says it.

### Removed

- **`BackupPage.local`** — now `top.signed.local`.
- **`LoginPage.about`** — no frame draws it; «about» is reached from the
  rail's foot.
- **`ForgotWipePage.warning`** — the second step's warning is in `notices`,
  the first step's words are its `lede`.
- **Hints no frame draws**: «At least 8 characters.» at setup's and the
  password change's new password, and the line under setup's address.
- **The field «Type yes to wipe encrypted data»** of `/forgot/{token}`.

### For consumers

- Every page in the shell carries `top.signed.local`; the fixtures of 7.20,
  7.21 and the backup, the password change and every shell page changed
  with it.
- A client that posted `confirm=yes` to `/forgot/{token}` still works: the
  field is ignored.

## v0.2.74 — 2026-09-28

### Fixed

- A letter's recipient suggestions label each address the way the card labels
  it. Before, the label was the raw TYPE lower-cased, so a vCard 3 address
  showed «internet», and «WORK,INTERNET» showed «work,internet». Now the first
  is unlabelled and the second is «work».

## v0.2.73 — 2026-09-28

The render contract is open: `carrel.dev/carrel/server/render` (client plan,
stage 1). Nothing changes on the wire — every answer, header and fixture is
the same byte for byte.

### Added

- **Package `render`** — what `internal/web` answers through, now importable
  by the client module: the `Renderer` interface, `OOB` and the fragment
  envelope `FragmentPayload`, the header names `Carrel-View` and
  `Carrel-Target` with the view values `page` and `fragment` (C-7.1), the
  eight target names of C-7.2 and `Targets()` in their order, `WantsFragment`,
  `FragmentTarget`, and `JSONRenderer`, which the server keeps answering
  with until the client draws HTML.

### For consumers

Implement `render.Renderer` to draw the server's answers; a fragment comes
with the whole view model of the page and the target it asks for — which
part of the model that target is, C-7.2 now says («Где зона во
вью-модели»). `web.New` takes a `render.Renderer`; `web.JSONRenderer` is
`render.JSONRenderer`.

## v0.2.72 — 2026-09-28

Copies the Typecase mockup (server note 20, to-typecase task 35): the
address label list, the quick task sheet without All day, and what the
delete sheet says points at a collection and when it was last archived.

### Added

- **`NewSheet.note`** — the line under the quick sheet's fields: «No time
  means the whole day. A task with no due date is a task all the same — both
  may stay empty.» (s-notes-10), and the contact's and the event's
  (s-notes-8, s-notes-9). Omitted when the sheet has no fields.

### Changed

- **The quick task sheet** (s-notes-10) is Summary, Due date and Due time,
  with the samples «YYYY-MM-DD» and «--:--» in `placeholder`; it has no All
  day. A due date without a time is due the whole day (`DUE;VALUE=DATE`), no
  date is no due at all, and nothing is refused (owner's decision). The full
  form keeps its All day (s-tasks-3).
- **A label another client wrote** — of an address (s-contacts-22), a phone
  or an e-mail — stands last under a line (`separator`), as it is: before it
  had no line and was shown lowercased. A `TYPE` value that is no label —
  `INTERNET`, `X400`, `VOICE`, `PREF`, `MSG`, `DOM`, `INTL`, `POSTAL`,
  `PARCEL` — is not offered as one: `WORK,PREF` chooses work, `INTERNET`
  alone chooses nothing (the row's `value` is the chosen label, or empty).
  Such values stay in the card: a save that leaves the label keeps the
  `TYPE` as it was, and a label chosen anew replaces only the label
  (`WORK,PREF` made home is `home,PREF`). Before, `INTERNET` was offered as
  another client's label, and a label chosen on `WORK,PREF` dropped `PREF`.
- **`CollectionDeleteSheet.references`** count the published links and the
  davloom devices — «1 published link — answers “not found” from that
  moment», «3 devices through davloom — it disappears for them at the next
  sync» — and name a backup job, as before (s-profile-9). Before, a line
  named each link and each device.
- **`CollectionDeleteSheet.archive`** says when a backup job that copies the
  collection or the folder last finished whole: «Last time it was in an
  archive: 2026-08-14. Restoring brings the records back into a new
  collection — not into this one, and not to this address.» («files»,
  «folder» for a folder, s-profile-19). A job that copies it and has not
  finished whole yet is said so; with no job the line is as before.

## v0.2.71 — 2026-09-28

Copies the Typecase mockup (server note 19). This part — the connections
and the forms (to-typecase tasks 26 and 28): editing a connection is a
dialog over the page, the whole storage goes elsewhere only as a copy, the
sheets of a storage's folder, and a new collection when no server of its
kind answers; the address rows of a card with their label and Remove on
every row of a phone, an e-mail and an address, and the quick sheets
labelled their own way. A table in a note (task 27) stays `table` as it is:
its «Wrap / One line» is the client's.

The second part — administration, the reset, the note and the account, the
move and From Files (to-typecase tasks 30–34): an invitation's page of its
own and its address in the list, the password once after Create user and
after a reset, the refusals at their fields as sentences, the trigger card
only while the trigger is on and «local» as the instance's, «Tags» on every
card, Related to outside what is loaded, the day of a list the short way
(C-10), a muted part of a label as data, the upload sheet's refusal beside
its line, the head of one published event — and a new event written in the
zone of its own Time zone row.

### Added

- **`InputRemove`** (`"remove"`, C-8): Remove at the end of each row of a
  phone, an e-mail and an address (s-contacts-3) — `name` «remove», `label`
  «Remove», `value` the row («tel.1», «adr.0»). The `template` of «Add …»
  carries it too, `value` «tel.{n}»: «{n}» is now replaced in `value` as in
  `name`. With scripts the client empties the row's fields and hides them,
  leaving them in the form; an emptied row goes on Save. Without scripts it
  posts the form with `remove=<row>`, and the answer is the same form
  without the row and nothing written: its fields travel hidden and empty
  under their numbers, and a hidden `removed` names it with every post until
  Save, which takes out exactly its value — the rows after it keep their
  parameters (S-7).
- **`ConnectionsPage.dialogTitle`** and **`dialogSubtitle`** — the head of
  the dialog over the page: «Connect account» (s-profile-2), «Edit
  connection» (s-profile-16), and «One connection gives calendars and
  address books alike.»; empty on the list and on a subscription.
- **`FormRow.labelAside`** — the muted part after a label: «(optional)» of
  «Email» (s-admin-1, s-auth-4), «SHA-256 · optional» of «Certificate
  fingerprint» (s-profile-2). The labels lose it.
- **`IssuedPassword.message`, `note`, `done`** — the card of a password
  shown once (s-admin-14, s-admin-17): «The password of anna is reset.»,
  «The user viktor is created. Hand this over; …», the line beside Copy,
  and Done.
- **`BackupPage.local`** — the instance answers only inside its network: no
  outside address is declared (`CARREL_PUBLIC_URL`) or it is a local name
  or a private, loopback, link-local or shared address. The badge «local»
  beside the tick (s-backup-2).
- **`Option.hint`** — under the line «By UID…» opens: «By UID: a record
  that is not among the loaded ones.»; its sample is the field's
  `placeholder`, «UID of the record» (s-tasks-11).
- **`FilePickerSheet.failure`** — a refusal of the sheet, New folder's
  under the line of the name (s-files-19).

### Changed

- **The page under a connection dialog is as it is** (s-profile-16):
  `title` stays «Connections» and `subtitle` the list's line in the dialogs
  of Connect account and of Edit — before, Connect account put its own head
  there and Edit had none.
- **An address row of a card** is `adr.N.type` — Label, picked from home,
  work, other (and a label another client wrote), written as `TYPE` — then
  Street, City, Region, Postcode, Country and Remove (s-contacts-3). The
  first part is no longer labelled «Street · home».
- **«Tags»** is the label of `categories` on the forms of a contact, an
  event and a task (s-contacts-3, s-agenda-3, s-tasks-3).
- **«Show N empty fields»** counts the fields folded — a card without
  addresses folds its «Add address» there, as one — and stays the last set
  of the form.
- **The quick sheet of an event** (s-notes-9) is Title, Date, Start, End,
  All day, Remind: the end has no date of its own and is on the start's day,
  or on the next when it is earlier than the start. Phone and Email of the
  contact's sheet carry the samples «+7 …» and «name@example.org»
  (`placeholder`, s-notes-8).
- **The whole storage** (s-profile-17) has no «Move to another server»:
  Rename and Delete on the server… are off with «This is the whole storage,
  not a folder in it.», and it goes elsewhere only as a copy.
- **Renaming a storage's folder** (s-profile-18): `stamp` names the storage
  and the folder, «vps · Docs»; `address` is empty — a folder's name is its
  address; the button is «Rename».
- **Deleting a collection** (s-profile-9, s-profile-19): `subject` is the
  server, then the collection, «nextcloud · Дни рождения»; the name to type
  is the field's `placeholder`, not its `hint`. A folder holds «files», and
  among what points at it stands «Attachments — the attachment folder is in
  it; new attachments will have nowhere to go»; a backup job reads «Backup
  job «…» — copies this collection (folder); it will run one short and say
  so».
- **New folder** (s-profile-20): the name has the hint «A folder's name is
  its address: there is nothing else to ask.»
- **No server of the kind answers** (s-profile-21): the sheet has no «On
  which server» field; `note` says why and Create is off with it.
- **Browse of the attachment folder** (attachments-4): «Choose this folder»
  is off with «Choose a folder in the tree.» until a folder is chosen.
- **A pending invitation has no login** (s-admin-1): `InviteRow.Login` is
  gone; `Address` is the first column — the letter's address, «—» for a
  link — and `Sent` of a link is «—».
- **`/invite/{token}` is s-auth-12**, not the registration: `Title` «Join»
  and the instance's name, `Lede` who invited — «mix invited you. Choose a
  login and a password; the address is the one the invitation went to.» —
  the password warning in `Notices`, then Login, the letter's address as a
  read-only Email (no field for a link), Password, Repeat password. An
  invitation made before v0.2.68 shows its login read-only. The state keeps
  who made an invitation (`by`); older ones name nobody.
- **Refusals are sentences**: every error a form names starts with a capital
  and ends with a full stop — «Login is already taken.», «Password must be
  at least 8 characters.» (s-admin-19, s-admin-16).
- **The answer to a reset** (s-admin-14) is the sheet with `Issued` only:
  no form, no warning, no actions — Done is `Issued.Done`.
- **The trigger card is sent only while the trigger is on**
  (`BackupJob.Trigger` is `*TriggerBlock`, null when off; owner's decision).
  The tick «On a clock of your own» moved from `TriggerBlock.Clock` to
  `BackupJob.Clock`. What turning it on costs — the job's key kept under
  the server's — is said in the card's note once the trigger is issued; an
  off trigger has no place to say it before (to-typecase task 36).
- **«Tags»** labels the categories of the full cards of a contact, an
  event and a task, as of the panels and the forms (s-contacts-6,
  s-agenda-2, s-tasks-2; owner's decision).
- **A day in a list is short** (C-10): «12 Aug», «12 Aug 18:00», the year
  only when it is not this year's — the due of a task row, the day of a
  note row, the rows of search, `Meta` of Related to and `When` of a
  related record (s-tasks-1, s-notes-1, s-tasks-10, s-notes-2). A record
  says it in full.
- **Related to** (s-tasks-11): a card is one pill «Анна Ковалёва ·
  contact» (`Label`, no `Meta`); a record not loaded is its UID with the
  field's `Hint` «A record that is not loaded here: its UID.».
- **The upload sheet** keeps its line — now the words of s-notes-11, «The
  folder is the attachment folder from Settings → Attachments, named once
  and remembered — unless you make a new one here. …» — and a refusal
  stands in `Failure`, not in its place (s-files-19).
- **One published event** (s-publish-16): `Lede` is when and where,
  «Monday, 10 August 2026, 10:00 – 11:00 · Переговорная 2»; the other
  records have none — the read-only badge says it.
- **A new event is written in its zone**: the new-event form has the Time
  zone row — the list without «As sent», the instance zone chosen — and
  writes `TZID` with its `VTIMEZONE` as the edit form does; UTC writes `Z`,
  Floating a floating time. The quick sheet draws no zone and writes the
  instance's. Before, a new event was written in UTC, and a weekly one
  moved by an hour at the change of summer time.

## v0.2.70 — 2026-09-28

### Changed

- **Reset a password asks the login typed.** `PasswordResetSheet.Form`
  gains a required `confirm` row, «Type the login to confirm». A POST whose
  `confirm` is not the chosen person's login changes nothing and returns the
  sheet with the error on `confirm`. Spaces around it do not count. This is
  the owner's decision of 28.09.2026: a reset wipes the account, as deleting a
  collection does, and asks for the same confirmation.

## v0.2.69 — 2026-09-28

Copies the Typecase mockup 0.7.2 (server note 18). This part — what the
frames removed and what became a toggle: a collection is published from
Connections only, deleting a task is one button, a task has one «All day»,
the role is one item, a password is reset from a sheet in the head of the
people's list with a temporary password, and the external trigger is
turned off by its own tick. Then the note, the rails and the account: the
list of Related to, the line of a note's unknown properties said by the
server, Save beside Display name, the escrow page without a copy and the
withdraw sheet as drawn. Last, the sheets of move and copy, From Files, a
server that asks no password and the rewrite of an address (to-typecase
tasks 22–25).

### Added

- **«Reset a password»** in the head of the Users page (`AdminUsersPage.add`,
  `admin.reset`, s-admin-2) opens **`PasswordResetSheet`** (s-admin-13,
  `GET /admin/reset`, `?target=` chooses the person): `title` «Reset a
  password»; `form` with **Whose** (`target`: the people of the list as
  «anna · User»; oneself — «Your own password is changed in your account
  settings.» — and a disabled account are there, off, with the reason) and
  **Temporary password** (`password`, offered by the server as on «Add
  users», with the hint «Hand it over yourself; they choose their own at
  the next sign-in. If their key is deposited, recover the account in Key
  escrow instead: nothing is destroyed then.»); `warning` — what the reset
  destroys, before the button; `actions` «Reset password»
  (`admin.reset.run`, danger, `POST /admin/reset`) and «Cancel». The reset
  is the wipe of `/forgot` whole — the connections, the published links,
  the external backup triggers and the devices go, the records on the
  servers stay — and the account opens at once with the temporary password,
  which must be changed at the next sign-in. Every wipe link of the person
  dies, every session ends. The answer is the sheet with `issued` — login,
  password, «Copy» (`admin.reset.copy`) — this once; the password is kept
  nowhere and journaled nowhere (`user.reset`, by whom, for whom). A
  refusal stands at its field and changes nothing.
- **`TriggerBlock.clock`** — the tick «On a clock of your own — cron,
  GoSentry, a job in Gitea» (`backup.trigger.clock`, s-backup-2): `on` while
  the external trigger is on, and then it posts `op=trigger-off`; unticked
  it issues the secret (`op=trigger`; outside the unlock window it leads to
  the password sheet).
- **`allday`** — the task's one «All day» (s-tasks-3), leading When.
- **`Option.meta`** and **`Option.typed`** — the list of **Related to**
  (s-tasks-10, the task's and the note's form): the records the session has
  loaded, of every collection of the catalog — events, tasks and notes, each
  with its collection's `colour` and `meta` «event · 12 August» (an event's
  start, a task's due, a note's day) — newest first, the record itself left
  out; last, after a line, «By UID…» (`typed`, empty `value`): the UID is
  typed by hand under the same name, `related`. A relation outside what is
  loaded stands first under its UID, chosen; to a card, under its name. No
  server is asked.
- **Save beside Display name** (`FormRow.extra` of `name`,
  `account.name.save`, `POST /app/settings/account?op=name`, account-1): it
  saves the name alone, as Change beside the address sends the address
  alone (`op=email` — the field is edited in place).
- **`EscrowWithdrawSheet.title`** «Withdraw my deposited key» (account-11).
- **`EscrowPage.rubric`** «What a deposit does» over the words of the page
  without a copy (account-12).
- **`FileTree.name`** — the line New folder opens over the tree (s-files-13):
  the field `name` «Name of the new folder», «Create»
  (`files.tree.new.create`, posting to `New.href`) and «Cancel»
  (`files.tree.new.cancel`, the same sheet with the line closed). A refusal
  stands under it and the line comes back with the name typed; the upload
  picker's tree has the same line.
- **`FilePickerSheet.title`** and **`.subject`**: «Attach from Files», «to
  «Sync-up»» (s-files-16); «Upload files» (s-notes-11); «Attachment folder»
  on Browse.
- **`FileTreeNode.size`** and **`.lands`** — a file of the From Files tree
  says its size and «inside» or «link», what Attach would do with it by the
  rule of an upload (s-files-16). A size the server did not say is not
  guessed.
- **`FilesPage.subtitle`** — the line under a folder's title: the storage's
  connection, and on a read-only storage «… · read-only: nothing can be
  uploaded or made here», the one reason Upload and New folder stand off
  (s-files-15).
- **`ConnectionCard.access`** — «no password» in the address line of a
  connection whose server asks none (s-profile-15).
- **`RewriteSheet.note`** — «Collections that are not ticked were not looked
  in.» under the blocks (owner's decision, s-contacts-19).

### Changed

- **The task form is the rubrics of s-tasks-3 whole**: When — All day, Start
  date, Start time, Due date, Due time; Progress — Status, Priority, Percent
  complete, Done last; Reminders — First, Add reminder; Other. **One «All
  day» for the whole task** (owner's decision, to-typecase task 13): ticked,
  the start and the due are both written as days (`VALUE=DATE`) and both
  times go; cleared, both are times again. It shows the start, or the due
  without a start. A save that leaves it alone changes the kind of neither
  time — another client's task whose start is a day and whose due a time is
  written as it was (S-7). The two boxes `start.allday` and `due.allday` are
  gone.
- **The role is one item of the row's «⋯»** (owner's decision, s-admin-12):
  «Make administrator» for a user, «Make user» for an administrator
  (`admin.user.role`, `POST /admin/?action=role&role=admin|user` — the role
  it gives, so pressing twice does not turn it back). For the last
  administrator it is off with the reason «Someone has to stay an
  administrator.»
- **`TriggerBlock.password.reveal`** is «Regenerate» only; while the
  trigger is off it is off with the reason «The external trigger is off:
  there is no line to renew.» The tick turns the trigger on.
- **`NoteDoc.foreign` is a `Text`**, the line under the text of a note's
  edit ready (s-notes-20): «Properties Carrel does not know — X-JTX-COLOR
  and two more — are kept unchanged and not shown here.», with one «A
  property Carrel does not know — X-JTX-COLOR — is kept unchanged and not
  shown here.»; empty when there are none. It was the list of the
  properties, which the client had to count.
- **The escrow page without a copy** (account-12): `subtitle` «No copy of
  your data key is deposited», the note «A deposited copy lets an
  administrator who holds the escrow master password recover this account
  if you lose your password — and, with it, read the DAV credentials you
  save here. You can withdraw it at any time.»
- **The withdraw sheet's warning** (account-11) adds «The copy of your data
  key on this server is destroyed; your records and connections stay as
  they are.» to the line beside the button.
- **The hint of «Written as»** (attachments-1): the example is
  «2026-08-10 sync-board.png», and it ends «…a counter is added.» — the
  queue saying so is the queue's.
- **A taken folder name is one line** (s-files-13), in the tree and on the
  bar alike: «There is a folder called «2026-07» in carrel already. Choose
  another name.» — «a file called» when a file holds it, «in» the folder it
  was to be made in. It was «2026-07 is already here» with a second
  sentence.
- **Attachments without a folder** says «not set» on the right of the
  shortcut (`Shortcut.place`, s-files-18) and leads, as before, to Settings →
  Attachments.
- **From Files' refusals** end with a full stop, as s-files-17 draws them:
  «No file was chosen.», «Nothing was attached: the file is no longer
  there.»
- **`ConnectionCard.password` is a pointer**, nil where the server asks no
  password: the card has no Show password at all (s-profile-15), where it
  had it off with a reason. The edit of such a connection has no «Leave it
  empty to keep the stored one.» at the field the mark turned off
  (s-profile-16).
- **Certificate fingerprint** (owner's decision; s-profile-2, -11, -14, -16):
  the label «Certificate fingerprint SHA-256 · optional», the placeholder
  «only for a certificate the system does not trust»; it stays the field
  under the password and its mark.
- **The rewrite sheet in the singular and with nothing to rewrite**:
  «1 record on a server that schedules» (s-contacts-19); where no record
  that names the old address can be written, the pair only counts them —
  «3 events and tasks name the old one», without what they lose
  (s-contacts-20). The outcome without a shortfall counts alone:
  «Rewritten in 12 records.» (s-contacts-21); «10 of 12» stays for a
  shortfall.
- `docs/spec/fixtures`: `admin-users`, `app-task-edit` (both), and the list
  heads of `app-agenda`, `app-contacts-one`, `app-notes`, `app-tasks`;
  `app-note`, `app-note-edit`, `app-settings-account`,
  `app-settings-attachments`, `app-settings-escrow`; then `app-files-picker`
  (title, the New folder line), `app-settings-connections` (`access`),
  `app-contact-rewrite` (the singular, the note), `app-contact/outcome`.
- Confirmed against the new frames, unchanged: the copy sheets of files and
  of a record (s-files-11, s-contacts-17), the message of a move to another
  storage (s-files-12), «Move 1 item», New folder off on the root of Files
  (s-files-6), Back to defaults off (s-dup-7), the birthday refusal
  (s-contacts-18), «Download .ics» on the public page of one event
  (s-publish-16); a read-only storage is a source in From Files, and the row
  of a link attachment writes no path.

### Removed

- **«Publish a link» in the head of a collection's list** (owner's
  decision, to-typecase task 12; s-contacts-1, s-agenda-1 draw New, Import,
  Export, Print). A collection is published from its menu in Connections;
  a record, a file and a folder from their own «⋯» as before. The head of
  Files sends Upload and New folder, and its «⋯» (s-files-7) is Print —
  the client's.
- **`TaskEditPage.deleteNote`**: no line under «Delete task» (to-typecase
  task 8). The series line of an event (`EventEditPage.deleteNote`, «Series
  is edited and deleted whole.») stays: s-agenda-3 draws it.
- **`UserRoleSheet`** and **`GET /admin/role`**: there is no sheet of roles.
- **«Reset password» in the row's «⋯»**, and `POST /admin/?action=reset`:
  the administrator's reset no longer issues a wipe link or a letter.
- **`TriggerBlock.off`** («Turn the external trigger off»): the tick is
  the switch.
- **«Nothing» in the list of Related to** — an empty field is no relation —
  and **the field `related.uid`** («Or enter a UID by hand»): «By UID…»
  types into `related` itself.
- **`account.save`**, the Save of the account form: each field has its own.

## v0.2.68 — 2026-09-28

The owner's deferred round, part 5 — administration: «Add users» is the
three cards of s-admin-1 — an invitation with its role and no login, a
temporary password, self-registration — and Key escrow says whom the master
password opens and takes the temporary password of a recovery (s-admin-6).

### Added

- **`AdminInvitesPage`** is three cards, `invite`, `temporary` and
  `selfReg` (`AddUserCard`: `title`, `aside`, `badge`, `lede`, `warning`,
  `form`, `link` with `copy`, `issued`), under `subtitle`, and `rows` as
  before. The page is titled «Add users», as its rail row.
  - **Invitation** («chooses their own password»): `role` (the list of
    s-admin-8, User chosen) and «Or send to»; «Create link»
    (`admin.invite.create`, `POST /admin/invites?action=link`) and «Send
    invitation» (`admin.invite.send`, `?action=send`, off with the reason
    without SMTP, and refused by the server then) — two buttons of one form, each with its href. The answer
    goes to `/admin/invites?invite=<id>`, where the card holds the new link
    in `link` with «Copy» (`admin.invite.copy`). **An invitation names no
    login:** the person chooses it on `/invite/{token}`, which now asks
    Login, Password, Repeat password (s-auth-2); a login taken meanwhile is
    said on its field and the link lives on. The row's `login` is empty
    until then (the frame's «—»). The role goes with the invitation and is
    the account's when it is accepted. Invitations made before name their
    login: the form shows it read-only, as a user.
  - **Temporary password** («hand to hand», the warning «Until the user
    changes this password, you can sign in as them.»): Role, Login, «Email
    (optional)», «Temporary password» — offered by the server, five groups
    of four characters, and the administrator may type another; «Create
    user» (`admin.user.create`, `action=user`). The account is made at once
    with the role and must change the password at the first sign-in
    (`/app/password`, as after an escrow recovery); an address typed here
    waits unconfirmed. The answer is the page itself, not a redirect, with
    `temporary.issued` — login, password, «Copy» — this once; the password
    is kept nowhere and journaled nowhere (`user.create`, `detail`
    `role=admin` or `role=user`).
  - **Self-registration** moved here from the Users page: the check
    «Publish the registration form», «Save» (`admin.selfreg.save`,
    `action=selfreg`), the badge «open» and the address `/register` with
    «Copy» while open. It cannot be turned on without SMTP: Save is off
    with the reason, and the server refuses it too.
- **`AdminEscrowPage.subtitle`** «N of M accounts open with the master
  password», **`opens`** (N), and **`covered`** now every account of the
  Users list as `EscrowCoverRow` — `login`, `covered`, `deposited` (the day,
  or «not deposited»), `lastRecovery` («never», the day and time «2026-08-14
  18:40», or «—» for one not deposited).
- **Recover takes the temporary password**: the row «Temporary password for
  the user» (`temp`), offered by the server; what is typed there is what
  the key is re-wrapped under (at least 8 characters), left empty the
  server makes one. The user must change it at the first sign-in, as
  before.
- **Resend** of an invitation is `action=resend&invite=<id>`: the same
  invitation gets a new secret and a new term, the letter goes again, the
  link sent before stops working (`user.invite.resend`). It used to be a new
  invitation for the login.

### Changed

- **`AdminUsersPage.selfReg` is gone** (it is the third card of «Add
  users»); `add` is «Add user» with `i-plus` (s-admin-2).
- **The Key escrow forms** carry their rubrics as the legend of their set —
  Policy, Master password, Recover an account — and the frame's hints: at
  «New accounts are covered» «Turning coverage off leaves copies already
  taken alone: deleting one is its owner’s decision.», at Repeat «Only the
  recovery private key is re-encrypted. The deposited copies are untouched,
  so nobody has to opt in again.», at the temporary password the recovery's.
  Recover is `danger`. The warning is the product's text alone, without a
  title.
- **`user.invite.create`** and **`user.invite.complete`** carry `detail`
  `role=user` or `role=admin`; an invitation without a login has no `whom`
  until it is accepted.
- The letter of an invitation without a login asks to choose «your login
  and password».

## v0.2.67 — 2026-09-28

The owner's deferred round, part 3 — forms: the contact form in the rubrics
of s-contacts-3, «Add phone» and «Add reminder» as buttons that add a row,
and the time zone of an event as the whole list of s-agenda-8.

### Added

- **`InputAdd`** (C-6.8, C-8, `kind: "add"`): a button under the rows of a
  repeating field — the phones, e-mails and addresses of a card, the
  reminders of an event and a task. `label` is its text («Add phone»,
  «Add email», «Add address», «Add reminder»), `name` is `add`, `value` the
  field (`tel`), and **`FormRow.template`** the empty rows one press adds,
  with `{n}` in their names for the lowest number no row of the field has
  yet (`tel.{n}.type`, `tel.{n}`). Without scripts the button posts the
  form with `add=<field>` to its action; the answer is the same form as
  typed with one empty row more, nothing written — and no record yet on a
  new form (C-7). The form no longer carries an empty row in advance
  (s-contacts-3: «not empty fields poured in beforehand»).
- **`Option.group`** (C-6.8): a heading before the option — «As sent» in
  the time zone list. Omitted when empty.
- **The time zone of an event is every zone** (s-agenda-8): the zone it came
  in first under «As sent», a line, the 489 Area/Location names of Go's
  zone database and UTC in alphabetical order, a line and «Floating — the
  same clock time anywhere», always last; an event that came floating has
  Floating under «As sent» and not again. The row's `placeholder` is «Find
  a zone», the text of the search over its options. Choosing Floating drops
  `TZID` and writes the clock time as the instance zone shows it (S-11); a
  zone outside the list is refused as before. The list is
  `internal/web/zones.txt`, written by `zones_gen.go` (`go generate`) from
  `$GOROOT/lib/time/zoneinfo.zip`; the row is about 35 KB of JSON, 5 KB
  gzipped. The binary now embeds the zone database (`time/tzdata`): the
  image has no `/usr/share/zoneinfo`, and without it every zone but UTC
  failed to load there, the instance zone included.

### Changed

- **The contact form in the rubrics of s-contacts-3.** The head — Display
  name, Given, Family, Nickname, Organisation, Title, Birthday; Nickname
  and Birthday stand there empty too (s-contacts-4) —, then **Phone
  numbers** and **Email addresses**, each row a `Label` select
  (`tel.N.type`, the list of s-contacts-12 and -13) and then `Number` or
  `Address` (`tel.N`, `email.N`), and «Add phone», «Add email»; then
  **Addresses** (five parts, the first labelled «Street» with the
  address's type, «Add address»; the frames draw none — to-typecase task
  28); then **Other**: Categories and Note. Empty Organisation, Title,
  addresses, Categories and Note fold into «Show N empty fields»; a rubric
  whose fields all folded is not sent. The field names are those of
  before, less the `.new` rows: `tel.new`, `email.new`, `adr.new.*` and
  `reminder.new` are no longer read — a row added is `name.N`, the next
  number.
- **Reminders**: the rows the record has (the first is «First», the others
  «Reminder»), then «Add reminder»; a record without reminders has the
  button only.
- **The quick sheets** hold one row of such a field, without its label list
  and without the button: «Phone» and «Email» on the contact's
  (s-notes-8), «Remind» on the event's (s-notes-9).

## v0.2.66 — 2026-09-28

The owner's deferred round, part 2: a folder of one's own downloads as a zip
from its row (s-files-10), a note's Markdown tables arrive as tables, and
C-7.5 names the action ids the server gives.

### Added

- **«Download as .zip»** (`file.download.zip`, icon `i-down`, GET) in the
  row menu of a folder, second after Open, where a file has Download
  (s-files-10). It is enabled on a read-only storage too. Its href is the
  folder's address under `download/` with `?zip=1`:
  `GET /app/files/{account}/{col}/download/{path…}/?zip=1` answers
  `application/zip`, an attachment named `{folder}.zip` in a sandbox, the
  names inside relative to the folder. The archive is written while the
  folder is read; at most `CARREL_FILES_MAX_ENTRIES` entries (files and
  folders, all levels) go in, and past that — or when a file does not open
  midway — the archive ends without its directory, so it reads as broken
  rather than whole with files missing. The same holds for a published
  folder's zip. A missing folder is 404; a listing the server refuses
  answers the folder above with the notice «{folder} could not be
  downloaded». Without `?zip=1` a folder address after `download/` is still
  the folder named `download` (S-17.5, C-5.5).
- **Tables in a note** (C-6.9, C-8). A GFM table in the rendered text of a
  note (`NoteDoc.rendered` of its page and its public page,
  `Details.rendered` of its panel) is a block `table` whose `children` are
  rows `row`, the first with `header: true`; a row's `children` are cells
  `cell`, one per column (a short row is padded with empty cells), their
  text in `text` or in `children` as a paragraph's (`strong`, `emph`,
  `link`, `code`). A cell may carry `align`: `left`, `center` or `right`,
  from the delimiter row; absent when the column sets none. `header` and
  `align` are omitted when false or empty, so no other block changes. Raw
  HTML in a cell stays escaped text. Before, a table arrived as one
  paragraph with its pipes.

### Fixed

- A backslash escape in a note's Markdown reached the rendered text as
  written: «a \* b» arrived as `a \* b`, «\|» in a table cell with its
  backslash. Every text block now carries the mark itself (`a * b`, `|`);
  code spans and code blocks keep their backslashes, and raw HTML is still
  escaped text.

### Changed

- C-7.5 names `publish.revoke`, the id the list of published links carries,
  where it said `link.revoke`, and lists the row menu of a file and a
  folder. Nothing in the running server changes.

### For consumers

Show «Download as .zip» in a folder's «⋯» as a plain link: it downloads,
nothing to post. Draw `table`, `row` and `cell` blocks as a table, the row
with `header` as its head, `align` as the column's alignment; no frame
draws one yet (to-typecase task 27).

## v0.2.65 — 2026-09-28

The owner's deferred round, part 1 — settings and connections, each brought
to its frame: Connect account in the head of the connections (s-profile-1)
opening the dialog as a state of its own (s-profile-2); the «⋯» of
collections (s-profile-1) and of the folders of a file storage
(s-profile-12); the rail foot naming what it makes; «On which server» with
only the servers that answer (s-profile-5, s-profile-13); and the folder for
attachments as a collection, a folder in it and a subfolder per month
(attachments-1, attachments-2).

### Changed

- **`ConnectionsPage.add` is `*Form`** (C-6): `null` on the list, which no
  longer carries a form. `GET /app/settings/connections?op=add` is the
  dialog Connect account: `add` is the form, `title` «Connect account»,
  `subtitle` «One connection gives calendars and address books alike.» Edit
  (`?op=edit`), the subscribe form (`?op=feed`) and every refused Connect,
  Save or Test only come with `add` as before.
- **`ConnectionsPage.actions`** is Connect account (`connection.add`, icon
  `i-plus`, GET `?op=add`) and Subscribe to a feed. New
  **`ConnectionsPage.subtitle`**: on the list «Signed in as mix · 4 servers
  · 16 collections · 2 subscriptions».
- **Connect account at the foot of every rail** (`account.connect`) and on
  the quick sheet with nowhere to write (`connection.add`) go to `?op=add`.
- **The rail foot of a kind** names what it makes: `collection.new` is «New
  address book», «New calendar», «New list», «New notebook».
- **`CollectionRow.menu`** of a calendar or an address book: Rename and
  colour (`i-pencil`), Move to another server (`i-move`), Copy to another
  server (`i-dup`), **Export .ics** or **Export .vcf** (`collection.export`,
  `i-down` — the collection whole, `…/export?whole=1` for a calendar, as the
  delete sheet hands it over), **Publish a link** (`publish`, the sheet of
  S-25), and under a line Delete on the server… (`i-trash`). The same icons
  on the rows of the kind rails.
- **The new-collection sheet** lists in «On which server» only the servers
  that answer as far as the session knows: the last check of the connection
  (Test, Look for changes, Edit) if there was one, else the last polls — a
  connection none of whose collections answered and one refused, timed out
  or was unreachable is left out. The row's `hint` says so: «Two of three
  are listed: baikal is not answering, and until it does there is nothing to
  create there.» In `target.options` such a server is `readOnly` with the
  reason «not answering». `note` is no longer «3 of 4 servers are listed…»;
  it is set only when nothing can be made.
- **`AttachmentsPage`** has no `folder` tree any more (C-6). The set
  «Folder» of its form is **`collection`** (select of the file collections,
  `value` `account|path`, `colour`; a read-only one `disabled` with the
  reason «Read-only»; a subscribed folder is not listed), **`folder`**
  «Folder in it» (text, `/carrel/attachments`, `extra` — Browse), and
  **`monthly`** «A subfolder per month» (check, `true` or empty; the hint
  names last month and this). POST takes those three and `name`; `to` and
  `sub` are gone. `GET /app/settings/attachments?to=<folder href>` fills
  collection and folder from a folder of the tree.
- **Attachments: the hint of «Written as»** ends as the frame's, «A name
  already taken is never overwritten: a counter is added, and the queue says
  so.»; the page's `note` is the frame's message, «Changing the folder moves
  nothing. …».

### Added

- **Folders of a file storage in the connections** (S-12.1, s-profile-12):
  `menu` Rename (`collection.rename`), Move and Copy to another server,
  Publish a link (`/app/publish?k=files&…&dir=1`), Delete on the server….
  `/app/collections/rename`, `/move`, `/delete` open for a folder:
  - rename is `MOVE` beside it, never over a taken name; the catalog entry
    takes the new address, and the ticks, the attachment folder and the
    upload folder follow it. The sheet has Name only.
  - delete is the typed-name sheet of S-12.4 with **Open in Files**
    (`collection.open`) where a calendar offers its download.
  - move and copy go to a connection that holds folders: a folder of the
    same name is made beside its folders (a taken one gets `-2`), the tree
    is copied a file at a time through Carrel (S-17.1), a copy that stops is
    taken back, and the original is deleted only after the copy is whole.
    Another server without folders is «no file storage».
  - the storage itself — the address the connection was made with — has
    Rename and Delete off with the reason «This is the whole storage, not a
    folder in it.»; Move of it says it can be copied, not moved.
- **New folder** in the head of a plain file storage's card (`folder.new`,
  `i-plus`, s-profile-1): `GET /app/collections/new?a=<account>&kind=file`
  — the new-collection sheet with `kind` `file`, «On which server» and
  Name only. POST makes the folder beside the storage's folders (`MKCOL`),
  a taken name is an error of `name`, and the answer is 303 to the new
  folder in Files; the catalog holds it as a file collection.
- **«In this folder now»** (attachments-1): a fourth set of the attachments
  form, present once a folder is stored — read-only rows `now.files`
  «Files» («5 · 782 B»), `now.oldest` «Oldest» (`2026-07-01`, the day the
  oldest file changed), `now.path` «Path» (the folder's whole address), with
  the frame's hint. Month subfolders count as the folder.
- **A counter added to an attachment's name is said in the queue**: a done
  row of the files queue (`FilesPage.queue`) named as the file was written,
  its note «… was taken, so a counter was added».
- **`/app/files/picker?for=attachments[&to=]`** (GET): Browse of the
  attachment folder — the tree at the folder; «Choose this folder»
  (`files.picker.choose`, GET) goes to `/app/settings/attachments?to=…`.
- **A subfolder per month** is stored with the attachment folder: a large
  attachment goes into `2026-08` inside it — the month its dated name
  carries — made on first use.

### Fixed

- The last poll of a kind (the rail of one collection, «On which server»)
  was the oldest poll the session kept: a server that answered again still
  read as refusing there.

### For consumers

Draw the connections list without a form, and the dialog when `add` is
present: `title` and `subtitle` are its head. Draw `hint` under «On which
server». On Settings → Attachments, draw Collection as a list with the
colour marks, Browse beside Folder in it, and post `collection`, `folder`,
`monthly`, `name`.

## v0.2.64 — 2026-09-28

An address replaced on a card is asked about (from-typecase note 16,
s-contacts-15, S-15.4): the events and tasks that named the old one would
drop out of the person's timeline, and rewriting an attendee is an act in
somebody's schedule, so the save goes to a sheet that names what rewriting
can set off before anything is written. The answer says on the card what
came of it (s-contacts-16).

### Added

- **`RewriteSheet`** and **`RewritePair`** (C-6) on
  **`/app/contacts/{account}/{col}/{uid}/rewrite?plan=…`** (GET, POST;
  C-5.4). `title` «Rewrite the address in their records?», `subject` «Анна
  Ковалёва · saved»; `pairs` — `from`, `to`, `note` («4 events and tasks
  name the old one; they drop out of the person’s timeline»); `effects` and
  `kept` — `Field` rows, `label` bold and `value` after the dash: «N records
  on servers that schedule», «N records on <server>» (the server did not say
  whether it schedules), «N meetings organised by others», «Past meetings»;
  «N records in <collection>» read-only, and a ticked collection that could
  not be read. `form` carries the plan and the CSRF; `actions` —
  `contact.rewrite.confirm` «Rewrite N records» (off with a reason when
  nothing can be written) and `contact.rewrite.leave` «Leave as is» (POST
  with `op=leave`). The rubrics «The address», «What rewriting can set off»,
  «What will not change» are the client's.
- **The save of a card** whose addresses changed answers 303 to that sheet
  instead of the card when an address was replaced: one removed and one
  added is that pair; otherwise every removed address goes to the card's
  default one (`PREF=1`, else `TYPE=pref`, else the first). Removals alone
  ask nothing. A loss report comes first, and then the sheet is not offered.
  The sheet with nothing to rewrite anywhere, and a plan that is gone or
  spent, is a 303 to the card.
- **Rewrite** reads every record of the plan again, writes it only at the
  version the sheet was drawn from, replaces the address in `ORGANIZER`,
  `ATTENDEE`, `SENT-BY`, `DELEGATED-FROM`, `DELEGATED-TO` of every component
  of the record (overridden occurrences too), keeps every other parameter
  and the scheme as written, puts it with If-Match and reads it back. The
  answer is the card (`ContactPage`) with a notice: `title` «Rewritten in 11
  of 12 records.», `body` naming each record not written and why («its
  organiser’s server does not take changes to attendees», «it changed after
  Carrel read it», the server's own refusal), the read-only ones by
  collection and the collections not read; `alert` when anything was not
  written, `info` otherwise. The journal has `contact.rewrite` with the
  counts only.
- **Discovery asks the principal** for `schedule-outbox-URL` and
  `calendar-user-address-set` (RFC 6638) — the trace has a step
  `scheduling`. The answer is kept with the connection when it is added,
  edited or re-discovery is applied; a connection made before this stays
  «unknown» until then.
- **Fixture state `outcome`** (C-14.3): `app-contact/outcome.json`; and
  `app-contact-rewrite/normal.json`.

### Changed

- The stand `davtest` announces the length of what it serves for a record,
  not of what it stored: a folded record was cut short on GET.

### For consumers

After Save of a contact, follow the redirect: it may be the sheet. Draw its
three rubrics, skip an empty one, and send `form` to the href of the button
pressed.

## v0.2.63 — 2026-09-28

A server that asks no password is said by a check in the connection form
(from-typecase note 16, s-profile-14): «The server asks no password» right
after Password turns the password field off. An empty active field still
means «unchanged» on edit, so the mark is what clears a stored password.

### Added

- **`FormRow.disables`** (new, omitted when empty) — on a check, the `name`
  of the row it turns off while ticked (C-6.8, a general rule). The server
  sends that row's `readOnly` for the check's current value; the client
  toggles it itself as the check is ticked and unticked. The row stays in
  place; its value is not read while the check is on.
- **Connection form** (`ConnectionsPage.add`, Add and Edit): a check row
  **`noPassword`** «The server asks no password» (`kind` `check`, `value`
  `true` or empty, `disables` `password`) right after Password. Ticked,
  Password comes `readOnly`. The edit form comes ticked for a connection
  whose stored password is empty.
- **The mark is kept with the connection.** Connect with it stores no
  password whatever the field held; Save with it clears the stored password,
  and the published links of the connection are sealed again with the empty
  one. Without it, an empty field keeps the stored password and a typed one
  replaces it, as before. «Test only» (`/app/settings/connections/test`)
  reads no password under the mark; a failed Connect or Save gives the form
  back with the mark as sent. The journal line is the same
  `connection.edit`: the connection and the number of links.

### Changed

- **Username**, not Login, labels the login row of the connection form
  (s-profile-2, -11, -14); the field name stays `login`.
- **The Connect account dialog is the frame's** (s-profile-14): Label has the
  `placeholder` «optional» and no hint; Server URL has the hint «.well-known
  is tried first, then the URL exactly as typed.»; the Add form's actions
  are Connect, Test only and **Cancel** (`connection.add.cancel`, GET
  `/app/settings/connections`). The dialog is the content of main, not a
  sheet, so its Cancel is the server's (C-7.2), as the edit form's already
  was.
- **Show password** (`connection.password`) on the card of a connection
  without a password comes off, with the reason «The server asks no
  password»; `?reveal=` releases nothing for it and journals nothing.

### For consumers

Send `noPassword=true` (or `on`) with the form when the check is ticked. For
any check with `disables`, turn the named row off while it is ticked.

## v0.2.62 — 2026-09-28

Attach has an arrow everywhere a file is attached to a record (from-typecase
note 4): «From this computer», as the button, and «From Files» — the tree of
Files choosing a file that lies there already, written by the attachment
rule of S-20. Upload of a folder has the same arrow: Files and Folder.

### Added

- **`Action.arrow`** (new, omitted when empty) — the arrow of a split
  button (`.m-split`): the items of its menu, `MenuItem` like any menu. The
  action itself is what the button does; the first item repeats it (same
  `id`, `href`, `method`, the frame's label).
- **Attach** (`attach.add`) on the card, the panel, the form of an event or a
  task and the note's page and edit carries `arrow` (s-agenda-6,
  s-tasks-7): **«From this computer»** (`attach.add`, `i-up`) and **«From
  Files»** (`attach.files`, `i-file`, GET `/app/files/picker?for=attach&k=&a=&c=&uid=&etag=[&from=edit]`).
- **Upload** of a folder (`file.upload`, `FilesPage.bar`) carries `arrow`
  (s-files-8): **«Files»** (`file.upload`, `i-doc`) and **«Folder»**
  (`file.upload.folder`, `i-file`) — both the same upload POST; which dialog
  of the system opens is the client's.
- **`/app/files/picker?for=attach`** — `FilePickerSheet` choosing a file: the
  same tree, the files of every opened folder after its folders
  (**`FileTreeNode.file`**, new, omitted when false; the `id` is the file's
  address, without «/»). `&to=<id>` opens a folder or chooses a file; the
  tree starts at the attachment folder. A read-only storage is a source like
  any other (no `readOnly` on its nodes); subscribed folders are not shown;
  no New folder. Actions: **Attach** (`files.picker.attach`, off with
  «Choose a file in the tree.» until a file is chosen) and **Cancel** (back
  to the record, or its edit with `from=edit`).
- **`?op=attachfile&file=<address>&etag=…`** on
  `/app/{calendar,tasks,notes}/{account}/{col}/{uid}/edit` (POST) — attaches
  that file with If-Match of the version shown, and answers as the upload
  attach does (303 to the record, or the record with a notice). Up to the
  inline limit (2 MB, or the collection's smaller `max-resource-size`) the
  file is read and written into the record exactly as an uploaded one;
  larger, it is **not copied** — the record links to it where it lies, the
  same link form (its address on its WebDAV server) a large upload gets in
  the attachment folder. A file gone meanwhile: «Nothing was attached: the
  file is no longer there».

### For consumers

Draw the arrow from `arrow` where it is present, and nothing where it is
absent. «From Files» opens a sheet like the upload picker; open or choose a
node by requesting the sheet's address with `&to=` set to its `id`.

## v0.2.61 — 2026-09-28

Move and copy are a sheet as their frames draw it (from-typecase note 12),
and the small things of note 13: the birthday typed as text, buttons under
the duplicate weights, the public download named by its file, Shortcuts in
the files rail.

### Changed

- **`FileOpSheet`** (POST `/app/files/{account}/{col}/op/…`, s-files-4) —
  new `title` («Move 3 items», «Copy 3 items», «Delete 1 item», «Rename»)
  and `subject` («from Documents / carrel / attachments»). `warning` of a
  move or a copy **stands on the sheet always** (was only across storages):
  inside one server «Inside one server this is MOVE: instant, no traffic. To
  another storage the file has to be downloaded and uploaded again — Carrel
  says so before it starts, not after.»; to another storage the old move
  text; a copy «A copy travels through Carrel even inside one server: …».
  New `failure` (`Notice`) — New folder did not make the folder.
- **`FileTree.new`** (new) — **New folder** in the tree's rubric
  (`files.tree.new`, POST, field `name`): makes the folder in the chosen one
  and answers the same sheet with the new folder chosen; off with a reason
  when the chosen folder is read-only. On the move/copy sheet it posts to the
  same `op` address with `mkcol=1`; on the upload picker (`FilePickerSheet`,
  s-notes-11) to `/app/files/picker?for=upload&mkcol=1` — the new folder is
  chosen, not yet remembered (Choose this folder remembers it). `null` on
  trees without it (`AttachmentsPage.folder`).
- **`MoveSheet`** (`/app/{kind}/…/{uid}/move`, s-contacts-2) — new `title`
  («Move contact», «Copy event») and `subject` («Анна Ковалёва · from
  nextcloud · Контакты»; only «from …» when the record cannot be read). The
  current collection's reason is **«already there»** (was «Already here»),
  a read-only one's **«read-only»** (was «Read-only»). `warning` stands on
  the sheet always: on one server «Inside one server this is MOVE: instant,
  no traffic. To another address book the card is copied and then deleted —
  Carrel says so before it starts, not after.» (calendar / event, task list /
  task, notebook / note); to another account the old text; a copy says the
  original stays. Opening the sheet reads the record once, for its name.
- **Birthday** of the contact form (`bday`, s-contacts-3, s-contacts-4) —
  `kind` **`text`** (was `date`), `value` **`14.03.1987`** or **`14.03`**
  without a year (was `1987-03-14`, `--03-14`), hint «Type it: 14.03.1987, or
  14.03 without the year.», new `placeholder` «DD.MM.YYYY». The POST takes
  DD.MM.YYYY and DD.MM, and still YYYY-MM-DD and --MM-DD; BDAY is written
  as before. A day that does not exist: «Type the date as 14.03.1987, or
  14.03 without the year.»
- **`FormRow.placeholder`** (new, omitted when empty) — the sample in an
  empty field; only the birthday has one.
- **`ThresholdControl.weightActions`** (new, s-dup-3) — **Save**
  (`duplicates.weights.save`, POST `/app/duplicates/decide?do=weights`, the
  `weight.<Signal>` fields as they stand) and **Back to defaults**
  (`duplicates.weights.defaults`, every `weight.<Signal>` empty; off with
  «Every weight is the default already.» while none is the person's own).
  `WeightControl.own` is what the frame marks «changed».
- **Public page download** (`PublishedPage.actions`, `published.download`,
  s-publish-6, -7, -8) — labelled by its file: **«Download .ics»**,
  **«Download .vcf»**, **«Download .md»** (was «Download»). Files and
  folders keep «Download» and «Download folder».
- **`Rail.shortcuts`** (new, `Shortcut`: an action with `place` and
  `count`) — the rubric **Shortcuts** of the files rail on every files screen
  (root, folder, the rail fragment; s-files-1, s-files-6): **Attachments**
  leads into the attachment folder (was Settings → Attachments; there still
  while none is chosen), `place` «Documents / carrel», `on` in that folder;
  **Published** with `count` — the person's links. Empty elsewhere.
  **`FilesRootPage.shortcuts` is removed.**
- **Files rail foot** (s-files-1) — **New folder** (`rail.folder.new`, POST to
  the open folder's `mkcol`; off with «Open a folder to make one in it.» on
  the root and the rail fragment, with the read-only reason in a read-only
  storage), Connect account, and **«Subscribe to a feed»** (was «Subscribe to
  a folder»; still opens the form for a folder). `rail.apply` stays: it is
  the rail's submit without scripts (C-6.3), and no frame draws it.
- **`CollectionMoveSheet`** (`/app/collections/move`, s-profile-8) —
  `subject` is «nextcloud · Дни рождения» (was «Дни рождения · nextcloud»).
  Options: reasons **«already there»**, **«files only»**, **«no calendars»**
  (were «Already here», «Files only», «No calendars on this server»); an
  eligible server's `stamp` is what it takes, «calendars» / «address books»
  (was the server's name again). `warning` for a chosen server: «Carrel
  creates a calendar there, copies all 51 events, then deletes this one on
  nextcloud. Devices, published links and backup jobs pointing at the old
  address will see it vanish — same as Delete, except the records live on.
  This is not renaming the path.» (the count as the session knows it, else
  «all its events»); a copy: «Carrel creates a calendar there and copies all
  51 events; this one on nextcloud stays as it is.» `actions` is the one
  button **«Move here»** / **«Copy here»** (was «Move» / «Copy» and «Copy
  instead» / «Move instead»).
- **Collection menus** (connections and rail rows) — **«Copy to another
  server»** (`collection.copy`, the sheet with `copy=1`) after «Move to
  another server» (s-profile-1).

### Fixtures

- Every screen's rail carries `shortcuts` (`[]`; the files root the two
  rows). `app-files-picker` has `tree.new`, `app-settings-attachments`
  `folder.new: null`; `app-contact-move`, `app-contact-edit`,
  `app-contact-new`, `app-duplicates`, `app-files` (rail foot),
  `app-collections-move` and every fixture whose rail or connections list
  has a collection menu (Copy to another server) re-taken. The fixture test
  normalises a collection path escaped as a query value too.

## v0.2.60 — 2026-09-28

The account page as its frames draw it (from-typecase note 10): the Email
block says the address's state, and Key escrow lives on the page, with a
sheet each way.

### Changed

- **`AccountPage`** (`GET /app/settings/account`, account-1) — `subtitle`
  «mix · created 11 February 2026» takes the place of the form's legend and
  of the read-only **Login row, which is gone**. `profile.sets` are the
  frame's rubrics: **Name** (Display name) and **Email** — the field is
  labelled **«Address»** (was «Email») and carries `extra` **Change**
  (`account.email.change`, POST `?op=email`: the address alone, the name
  untouched). The field shows the address in use or, invited by a link with
  nothing in use yet, the one being confirmed (was empty). The hint no longer
  carries the state. **`mail`** («Letters you send»: Sent as, Replies go to)
  **is removed**: the letter's sheet (`MailSheet.from`, `replyTo`) says it.
  `actions` lose **Key escrow** (`account.escrow`): the frame has no such
  button; escrow is on the page. New `passwordNote` beside Change password.
- **`EmailStatus`** (`AccountPage.email`, new): `state` (C-8 `EmailState`:
  `confirmed`, `pending`, `unconfirmed`, `sent`, `none`), `badge`
  («confirmed 14 February», `linked`; «pending: new@example.org»,
  `neutral`), `confirmedAt`, `pending`, `sentTo`, `sentAt` («11:42»), and
  `message` — a `Notice` (`alert`, no title) saying the consequence, with
  one action in `actions`: **Send confirmation** (s-mail-9) or **Send again**
  (account-9, «…until you open the link we sent to mix@example.org at
  11:42.»), id `account.confirm`, off with a reason when mail is not set up.
  «Sent» lasts while the link lives (48 h); an address confirmed before this
  version has no day and says «confirmed». A pending change with no live
  letter (the relay refused it, or its link burnt) carries a `message` too:
  «No link has gone to new@example.org: until one is opened,
  mix@example.org stays in use.» with **Send again**; with a live letter it
  is the badge alone (account-1).
- **Send confirmation** — a letter the relay took answers with a redirect
  (was a page with an info notice); a refused one still answers with the
  alert notice. Saving a new address likewise.
- **`EscrowStatus`** — `lastUsed` (which held the deposit's date) is
  replaced by **`deposited`** («2026-03-02») and **`lastRecovery`**
  («never» or the day); new `badge` («deposited 2026-03-02», `alert`) and
  `actionNote` («After withdrawal nobody can recover this account, including
  you.»). It is now filled on the account page too (was always empty there),
  in the words of account-1 and account-7; the escrow page's are account-4's.
  `EscrowPage.subtitle` «A copy of your key on this server» when deposited.
- **Deposit my key** (`escrow.deposit`) is GET
  **`/app/settings/escrow/deposit`** — an `UnlockSheet` over Account
  (account-2): purpose «Deposit my key», «Your Carrel password», buttons
  Deposit my key and Cancel. Its POST (`/app/settings/escrow`,
  `action=deposit`) takes `password`, opens the same unlock window under the
  same buckets and deposits; without it, only inside an open window.
  `EscrowPage.confirm` is the same sheet. Was a POST enabled only inside the
  window.
- **Withdraw my deposited key** (`escrow.withdraw`) is GET
  **`/app/settings/escrow/withdraw`** — new **`EscrowWithdrawSheet`**
  (`warning`, `form`, `actions`: Withdraw my deposited key — the old POST
  `action=withdraw` — and Cancel). The page's button no longer withdraws.

### Fixtures

- The fixture instance is the frames': key escrow on (account-1, s-admin-6)
  and people's letters from their login on the instance's domain (s-mail-1).
  `admin-escrow`, `app-settings-escrow`, `app-settings-account`,
  `app-mail-compose` (`from` «Михаил Енученко <mix@carrel.example>»,
  `replyTo` mix@example.org) and `admin-audit` (`escrow.enable`) re-taken.

### Store

- `User.emailConfirmedAt` — set when an address is confirmed (the link, an
  invitation by letter). `SecretToken.mailedAt` — set on a confirmation link
  when the relay took its letter. Both absent on older records.

## v0.2.59 — 2026-09-28

Rails: numbers and navigation (from-typecase note 15). No view model
changes shape; the values in three rails do.

### Changed

- **Search** (`GET /app/search`, s-find-1) — the rail is built from the
  search on the screen. Each source row's `count` is what that source found
  (a feed's too: here it is its finds); one that did not answer has
  `state` `unavailable`, `message` («no reply», «password · 401»…),
  `detail` and `retry` (`source.retry`, the same POST as the poll bar's).
  Without a query, or while a source is still asked, `count` is `null`.
  The section's `account` is **«Sources a / b»** — a sources asked of b
  ticked (was «14 sources»); a ticked collection that is gone is listed with
  `state` `gone`, no count, and is not in a. The numbers are as of the page;
  the poll stream still moves only the poll bar.
- **Timeline** (`GET /app/contacts/{account}/{col}/{uid}/timeline`,
  s-contacts-7) — `rail.nav` is the six section rows with their counts
  (`nav.contacts` `on`), then «Back to list» (`nav.back`, `i-back`), which
  the client draws as a group of its own (C-6.3). Was «Back to list» alone.
  A feed row's `count` is now `null` («ro»), as in the agenda, and the
  combined row's sum leaves feeds out.
- **Conflict** of an event, a task and a note: nothing changed in the
  server; their fixtures now come after the list was opened, as a person
  arrives, and carry the collections' counts like the contact's.

## v0.2.58 — 2026-09-28

The panel beside a list is for reading, and the edit of a note is on the
note's page (from-typecase note 9): the panel carries what each kind's panel
frame draws, and `/edit` of a note answers the whole page with its shell.

### Added

- **`Details`** (the panel, C-6.3) — `subtitle` (where the record lies; a
  note's also when it was written: «nextcloud · Записи · 10 August 2026,
  11:05»), `avatar` (a contact's photo or initials; `null` for the other
  kinds), `tags` (the record's categories — they are no longer a
  «Categories» field), `body` (an event's or a task's description, as text),
  `rendered` (a note's text as blocks), `attendees`, `attach`, `related`
  (the record page's rows), `foreign` (a contact's «Kept as the server sent
  them»), `deleteNote` (under the deletion of a series: «Series is edited and
  deleted whole.»). An event's panel fields gain **Reminders** and
  **Repeats** under When; a task's gain **Complete** («40%»). A note's panel
  has no Date field: the date is in `subtitle`.
- **`Rail.neighbours`** — on a note's page, read and edited: the open note
  and one on each side, in the notebook's list order (newest first, then by
  title), `on` on the open one, `icon` `i-note`, id `nav.neighbour`. Taken
  from the list the session loaded; `[]` when the notebook was never listed.
  Every page's rail has the key.
- **`BlockTask`** (`"task"`) and **`Block.checked`**: a Markdown task item
  («- [ ] …», «- [x] …») is a task block, its text without the box. Before,
  it was an `item` whose text began with «[ ] ».
- **`NoteDoc.foreign`** — in edit, the note's properties Carrel does not know
  (`X-JTX-COLOR`…): name, value, `mono`; they are kept as they are.
- **Notebook** — the first of `NoteDoc.fields` in a note's edit (`notebook`,
  select): every notebook, its value the notebook's list address, `colour`
  on each option; a read-only one is `disabled` with `reason` «Read-only».
  Saving with another notebook saves the edit, then moves the note there
  (S-18) and redirects to it. When it cannot move, the edit is saved, the
  note stays, and the answer is its page with the notice «Saved, but not
  moved»; a save that lost properties shows the loss report first, with the
  same notice (its column keeps `linked`). A save refused by version keeps
  the Notebook with the rest of the edit: the conflict screen says «Not moved
  yet» (info) and where to; «Use only my edit» and «Write this combination»
  write, then move the note — they are enabled for the move alone too;
  «Use only the server version» and Cancel leave it where it is.
- **`Option.colour`** — the mark of an option that is a collection; omitted
  elsewhere.

### Changed

- **`GET /app/notes/{account}/{col}/{uid}/edit`** and **`…/new`** answer a
  **`NotePage`** (was a bare `NoteDoc`), as a page and as a fragment alike:
  `doc` in `editing` mode, rail «Back to list» (was «Back to the card») with
  the notebooks and the neighbours, `meta` with `attach`, `linked`,
  `actions` (Attach, Delete note) and no `tags`, `related` or `date` — they
  are fields now. Read the form from `.doc`.
- **`NoteMeta.date`** on a note's reading page is the day and the time when
  the note has one («10 August 2026, 11:05», s-notes-2); was the day only.

## v0.2.57 — 2026-09-28

Row actions follow how often they are needed (from-typecase notes 5, 8, 11,
17): the frequent ones on the row, the rare ones under «⋯». An action that
cannot be taken is sent switched off with its `Reason`; one that can carries
no `Reason` — the explanation beside a working button moved to a field of
its page (C-6.2). No action is switched off without a reason, and the
segment, tab or preset shown now is `on` **and enabled** everywhere.

### Added

- **`BackupPage.Delete`, `BackupPage.DeleteNote`**: on the job's page
  (`/app/settings/backup?job=ID`, s-backup-2) the deletion at its foot and the
  line beside it; `null` and empty on the list and for a new job. The job's
  form (`Add`) there carries Save, Cancel (`backup.cancel`) and Run now
  (`backup.run`).
- **«Job is on»** — form row `on` (check, value `1`) after Name. A job saved
  without it is off: it runs from the screen only, not at sign-in; the
  external trigger answers 409 «this job is off». Jobs saved before stay on.
- **`TriggerBlock.Off`**: «Turn the external trigger off» on the trigger card;
  `null` while the trigger is not on.
- **`InviteRow.Menu`**: Extend, a line, Revoke (s-admin-11).
- **`InviteRow.Delivery`** («link», «email») and **`InviteRow.Status`**
  («pending»): the columns of s-admin-1.
- **`BackupPage.Subtitle`**: on a job's page «Job · last copy 2 days ago»
  (or «Job · no copy yet»), with the job's name as `title`; empty on the
  list.
- **`GET /admin/role?target=ID`** → **`UserRoleSheet`** (`login`, `form`): the
  Role select — User (`user`), Administrator (`admin`) — with Save and
  Cancel; it posts to `/admin/?action=role&target=ID`.
- **`POST /admin/?action=enable&target=ID`** enables a disabled account;
  journal `user.enable`.
- **`AccountPage.DangerNote`**, **`DevicesPage.Note`**: the lines the frames
  draw beside Delete account and above Generate profile.
- Timeline: **`Range.Form`** — From, To (`?from=`, `?to=`, both days in) and
  Show (`range.show`).

### Changed

- **Published links** (s-publish-1): row actions Copy, Send, Revoke; Send is
  labelled «Send» (was «Send this link») and is there when mail is on.
  `created` is the day («9 August 2026»), `lastSeen` absolute or «never
  opened». A link from before links were kept has Copy and Send switched off
  with the reason.
- **Backup job row** (s-backup-1): `actions` are Run and Edit only. Run of a
  «Download» job is `GET …?op=download&job=ID` (streams the archive); of the
  others `POST …?op=run`. `backup.download`, `backup.trigger.off` and
  `backup.delete` left the row. Outside the unlock window the trigger's
  reveal button has no `reason`; the note says «Confirm your password
  first: …».
- **Invite row** (s-admin-1): `actions` are Resend (`admin.invite.resend`,
  only for an invitation by letter; off without SMTP) and Copy link
  (`admin.invite.copy`, GET, `href` = the link). «Send again» is gone. A
  failed letter's `sent` is «failed — » and the relay's reply («failed — 550
  mailbox unavailable»).
- **User row** (s-admin-12): `menu` is Set role (`admin.user.role`, GET the
  sheet), Kill sessions, Disable or Enable (`admin.user.enable`), Reset
  password, a line, Delete (`i-trash`). «Make administrator», «Make user» and
  «End sessions» are gone. For the last administrator Set role, Disable and
  Delete are sent off with the reason; Kill sessions is off with no live
  session. `role` is «User» or «Administrator»; `created` is the day,
  `lastLogin` absolute.
- **Contact timeline** (s-contacts-7): segments read «All 24» (was «All ·
  24»). `Range.Presets` is empty — «Widen the range» is answered with the
  range form.
- **The one shown now is `on` and `enabled`, with no reason** — was switched
  off (some with «Shown now.»): list orders (A–Z, Newest, By due…), task
  segments, search kinds, audit segments, duplicates kinds, the agenda's
  Day/Week/Month, the device tabs, the files view options, the photo
  rotation, the timeline segments.
- **Segment counts as the frames write them**: task segments «Open 12»,
  «Done 48», «All 60» (were «Open · 12»…); search kinds and audit segments
  carry no count (were «All · 7», «Security · 4»…).
- **Admin settings**: «Send test» switched off carries its reason.
- `account.delete` and `device.profile` carry no `reason`.

### For consumers

Regenerated fixtures: `admin-audit`, `admin-settings`, `admin-users`,
`app-agenda` (normal, empty), `app-agenda-all` (empty, partial),
`app-contact-timeline`, `app-contacts` (normal, empty, partial,
unavailable), `app-contacts-one` (empty, readonly), `app-duplicates`,
`app-files-published`, `app-notes`, `app-notes-all`, `app-photo-preview`,
`app-search`, `app-settings-account`, `app-settings-backup`,
`app-settings-devices`, `app-tasks`, `app-tasks-all`. Mark the shown
segment by `on`, not by `enabled`. Show a switched-off action with its reason; read
explanations from `deleteNote`, `dangerNote` and `note`, never from the
`reason` of an action that works.

## v0.2.56 — 2026-09-28

The task and event forms are laid out in the mockup's rubrics, and every
select field offers the list its list frame draws (from-typecase notes 3, 6
and 7).

### Added

- **`Option.Separator`, `Option.Disabled`, `Option.Reason`** (C-6.8): a line
  before a choice, and a choice that is shown and cannot be taken, with why.
  An `InputCheck` row with `Options` is several ticks under one name: each
  selected option is posted as a value of that name. A select with no option
  selected is empty and posts empty.
- **`TargetPicker.New`** (C-6.6): the last line of the import wizard's
  «Into», under a line — New address book, New calendar, New task list or New
  notebook (`import.collection.new`). It opens `/app/collections/new?kind=…&back=…`;
  creating the collection there redirects back into the wizard with it chosen
  (`?to=…&into=…`), the file already taken recounted against it. `back` is
  honoured only for an import wizard of the same kind.
- **`AdminAuditPage.Filter`**: a GET form with one select, `action` — All
  actions, a line, then the action codes the chosen segment shows
  (`login.ok`, `login.fail`, …). `/admin/audit?action=` filters the journal;
  the segments and Export CSV keep it.
- **Settings → Attachments, «Written as»** is a choice now (`name`: `dated`,
  `original`, `titled`): «Date, then the original name» (default,
  `2026-08-10 report.pdf`), «The original name» (`report.pdf`), «Record
  title, then the original name» (`Sync-up - report.pdf`). Stored per person;
  a taken name still gets a number. **The default changed:** large
  attachments are no longer named with the record's title unless the person
  chooses it.

### Changed

- **Task form** (`TaskEditPage.Form`): sets `""` (Summary, Description),
  `When` (All day, Start date, Start time, All day, Due date, Due time),
  `Progress` (Status, Priority, Percent complete, Done), `Reminders`, `Other`
  (Location, Categories, Related to). Field names are unchanged; labels are
  the frame's: Title → Summary, Starts → Start date, Due → Due date, … at →
  … time.
- **Event form** (`EventEditPage.Form`): `""` (Summary, Location, Status,
  Categories, Description), `When` (All day, Start date, Start time, End
  date, End time, Time zone), `Reminders`, `Repeats`. **`repeat.on` is one
  row** of `Options` MO…SU posted as repeated `repeat.on` values; the seven
  `repeat.on.MO`… checkboxes are gone. Times → Count (also in
  `Recurrence.Fields`).
- **Reminder rows**: the first is labelled First, the adding row Add
  reminder. Offered: At the time, 5, 10, 30 minutes, 1 hour, 1 day before,
  a line, No reminder. The 15-minute, 2-hour, 2-day and 1-week steps are
  gone as choices; a reminder already holding one stays offered.
- **Task status**: Needs action, In process, Completed, Cancelled — no «No
  status». A task without STATUS shows Needs action, and saving it untouched
  writes no STATUS (S-7).
- **Priority**: No priority, a line, High · 1, Medium · 5, Low · 9. **Event
  status** and **Frequency** gain a line under their first choice. **Time
  zone**: «Floating — the same clock time anywhere», a line under the zone
  as sent.
- **Phone and e-mail labels** are separate lists: mobile (`TYPE=cell`), work,
  home, main (`TYPE=main`), fax, other; work, home, other. No «No label»: a
  row without a label selects nothing. A label is matched whatever its case
  and keeps the value as the card holds it (`CELL` stays `CELL`), so
  `TYPE=CELL` no longer shows twice.
- **Mail encryption**: STARTTLS, TLS, a line, None; what None is for moved
  to the field's hint. **Link lives for**: a line before No expiry.
- **Backup «Collection»**: a read-only file collection is listed, disabled,
  with the reason.
- The event and task cards show STATUS as its list names it («Confirmed»,
  not «confirmed»).

### For consumers

Regenerated fixtures: `admin-audit`, `admin-settings`, `app-collections-move`,
`app-collections-new`, `app-contact-edit` (normal, loss), `app-contact-import`,
`app-contact-move`, `app-contact-new`, `app-event`, `app-event-edit` (normal,
loss), `app-publish`, `app-quick`, `app-settings-attachments`,
`app-settings-backup`, `app-task-edit` (normal, loss). Every `TargetPicker`
now carries `new` (null where there is none).

## v0.2.55 — 2026-09-28

What stands on a record and what is under its «⋯» is now data, as the
mockup's frames draw it (from-typecase notes 1 and 2).

### Changed

- **`Menu []MenuItem` beside `Actions`** on `ContactPage`, `EventPage`,
  `TaskPage`, `NoteMeta` and `Details` (the list panel and the timeline's
  card). `Actions` are the buttons shown; `Menu` is the «⋯» in frame order,
  `Separator` marking a line before an item. Move (`record.move`), Copy
  (`record.copy`) and Publish a link (`publish`) left `Actions` for `Menu`,
  and a new item `record.export` joined them.
  - Contact card: `Actions` Edit, Send (when mail can be sent), Delete
    contact; `Menu` Move, Copy, Export .vcf, Publish a link.
  - Event card: Edit, Attach, New task, Delete event; task card: Edit, Mark
    done, Attach, Delete task. `Menu` Move, Copy, Export .ics, Publish a link.
  - Note page: `Doc.Bar` Edit, Export Markdown (moved from `Meta.Actions`);
    `Meta.Actions` Attach, Delete note; `Meta.Menu` Move, Copy, Publish a link.
  - Panels: contact — Edit, All records (`contact.timeline`, the timeline),
    Delete; event and task — Edit, Attach, Delete; note — Edit, Open the note
    page (`record.open`, icon `i-expand`), Attach, and the deletion as the
    last menu item behind a line. The panels of a contact, an event and a
    task no longer carry «Open the …»: the row opens the record. The note
    panel had no Edit at all — it has now.
  - Timeline card: Open the card, Edit, Delete contact, and the card's menu.
  - Read-only: no Edit, Move or Delete; a subscription has no Publish a link.
- **Row menus of the lists** (`ContactRow`, `AgendaRow`, `TaskRow`,
  `NoteRow` `.Menu`, components-1…4): Open, Edit, Move, Copy, Export …,
  Publish a link, then Delete behind a line; icons as in the frames. They
  were empty (a task row held only Mark done).
- **`TaskRow.Tick *Action`**: the row's done box (`task.done`). It is no
  longer an item of the row's menu. `nil` on a read-only row.
- **File rows** (`s-files-9`, `s-files-10`): Open first (`file.open` — the
  file's address, its folder with it selected; a folder's own address), then
  Download (files), Rename, Move, Copy, Publish a link, Delete. The bar over
  a folder no longer carries `publish`: a folder is published from its row.
- **Delete on the edit page** (`s-agenda-3`, `s-tasks-3`):
  `EventEditPage` and `TaskEditPage` gained `Delete *Action` (`record.delete`,
  `Danger`, `POST` with the version) and `DeleteNote Text`, the line under it:
  «Series is edited and deleted whole.» for a series, «Deleting goes to the
  server at once, and nothing can undo it.» for a task. On the edit page
  `Recurrence.Note` is empty, so the line stands once.
- **Labels follow the mockup's ellipsis rule** (§6): only «Delete on the
  server…» keeps it. Now «Edit» (not «Edit contact»), «Move», «Copy»,
  «Publish a link», «Rename», «Import», «Move to another server», «Copy
  instead», «Move instead», «Delete account», «Create password»; a file's
  «Delete». The collection's delete is «Delete on the server…», the
  connections page's rename «Rename and colour». `RecurrenceBlock.Note` reads
  «Series is edited and deleted whole.».

### Fixed

- **Export of one record** (S-19): `…/export?uid=` hands over that record
  alone, named after it, for every kind. Before, only a note did; a contact,
  an event or a task silently came back as the whole collection. An unknown
  UID is 404.

### For consumers

Read the buttons from `Actions` and the «⋯» from `Menu`; put a `Danger`
action at the foot. Take the row's done box from `TaskRow.Tick`. Render the
edit page's `Delete` and `DeleteNote` at the foot of the form. Match actions
by `ID`, not by label: labels lost their «…» and nouns.

## v0.2.54 — 2026-09-28

### Changed

- **The collection palette is the mockup's six tones** (`s-profile-7`):
  `#4a6b52`, `#3f6e8c`, `#6b4e7a`, `#a05a2c`, `#8c6239`, `#9a9280`, in that
  order. The collection sheets offer them. A collection without a server
  colour and a contact's avatar get one of them from the hash of the path or
  UID. Colours Carrel wrote to servers before are the servers' now and stay
  as they are. On the rename sheet, a colour outside the six shows no swatch
  chosen, as any foreign colour did before.
- Sending a letter over TLS (port 465) gives up when the request does. Before,
  a relay that took the connection and never finished the handshake held the
  page for 15 seconds.

### For consumers

Nothing to change: `Colour` values are still `#rrggbb`.
Internally, the core no longer assumes that every connection is a DAV
server, so the mailbox (`docs/spec/carrel-mailbox.md`) can slot in later.
See «Почта — подготовка» in `docs/architecture.md`.

## v0.2.53 — 2026-09-25

The import preview's `import.cancel` is now **Cancel**, not «Choose another
file»: the `POST` drops the file held in the session and answers `303` to the
collection's list. Before, leaving the preview any other way kept the file,
and the next import into that collection opened on it again.

C-7.2 now says whose a sheet's Cancel is: the client's when closing asks
nothing of the server, and then `Actions` may not carry it; the server's —
a `POST` in `Actions` — when there is something to drop. A page always
carries its Cancel.

## v0.2.52 — 2026-09-25

Nothing in the running server. The Linux stands no longer follow `latest`:
`docker-compose.linux.yml` pins `ghcr.io/tchapi/davis-standalone:5.4.4` and
`stalwartlabs/cli:1.0.12`, the images the live suite last passed against.
Bump them on purpose, with a gates run.

## v0.2.51 — 2026-09-25

Nothing in the running server. The development documents are compressed and
split by purpose:

- [docs/architecture.md](../../docs/architecture.md) — how the server is
  built: modules, dependencies with the pins of `go.mod`, closed forks,
  terms and limits.
- [docs/development.md](../../docs/development.md) — how the work is done:
  rules, gates, live servers, the development machine, fixtures, lessons.
- [docs/security.md](../../docs/security.md) — security forks, the Fable
  audits, what is not revisited and what is accepted as it is, and what has
  landed since the last audit.
- `docs/plan/` keeps only plans: `server.md` (open and done, 2351 lines →
  80), `reviews.md` (Opus), and the exchange with Typecase. The tails of the
  pre-S-30 review and `review-server.md` are gone; code comments name the
  review finding (Р-9…Р-13) instead of the file.

## v0.2.50 — 2026-09-25

### Added

- **Published links count their opens** (`s-publish-1`, column «Opens»):
  `PublishedRow.Opens` (C-6). A secret link is given to people outside, and
  the one way to see it went further than meant is that it is opened. Every
  open counts; a check that a link lives does not. The count is written
  with «Last opened», at most once an hour; the opens since are held in
  memory and shown, and a restart loses at most that hour's.
- **A calendar link given out for some days says which** in the list:
  «Личный · 10–31 August».
- **The files root names the storage attachments go to** (`Purpose:
  "attachments"`).

### Changed

- **The list of published links stands in the settings rail**, as the frame
  draws it, with «Published links» on.
- **A collection's own page shows what the last combined poll could not
  read** in its rail, with Retry: a row that looked fine until the combined
  view was opened again would say the wrong thing.
- **The section's number counts the collections that answered**: one that
  refused says why in its own row and does not take the sum to a dash.

### Fixture set

- File collections «Documents» and «Public» on `sftpgo · vps` and «Files» on
  `nextcloud`, the attachments folder, `baikal · Общий` refusing with 401,
  and the two links of `s-publish-1`. The screens of the files and of the
  published links are taken on data, not in their empty state:
  `app-files/empty.json`, `app-files-picker/empty.json` and
  `app-files-published/empty.json` gave way to `normal.json`.
- `docs/visual/fixtures/README.md` says how to load them by hand.

### For consumers

Show `opens` beside «Last opened»; an empty `lastSeen` is «never opened».

## v0.2.49 — 2026-09-25

### Fixed

- **A calendar has its number in the rail**, as C-6.3 has always said: how
  many events the whole calendar holds, not how many the week shows, and not
  its tasks or notes. The server showed a dash, on the ground that the
  agenda reads a week. That was a departure from the spec that nobody had
  caught, and the frames draw the number.
  - The visit that reads a week of a calendar also asks it for its events'
    addresses and versions, without bodies, and counts them
    (`dav.CalCounter`, `cal.Provider.Count`).
  - A whole read of the events counts them at no cost.
  - A week taken from the cache under an unchanged tag asks nothing more:
    the number has not changed either.
  - The section's number and the combined row sum the calendars, as for
    every other kind.
- **A feed has no number in the rail.** «ro» stands in its place in the
  frames, and the section's sum is of the person's own collections. A feed
  ticked in a combined view used to take the section's number to a dash.

### Checks

- The fixtures of the agenda and of an event are taken after the combined
  calendar, as a person comes to them. `docs/spec/check.py` is green: no
  frame disagrees with its fixture.

### For consumers

Nothing new in `view`: `SourceRow.Count` and `NavRow.Count` carry numbers
for calendars where they carried `null`.

## v0.2.48 — 2026-09-25

The fixture set holds what the frames draw (docs/plan/from-typecase.md).

### Changed

- **Subscriptions are in the fixture set.** «Праздники РФ» and «Расписание
  лиги» are served from `docs/visual/fixtures/feeds/` and subscribed to
  through the form. «Праздники РФ» is ticked in the calendar and nowhere
  else. «Расписание лиги» asks for a password and then stops taking it, as
  `s-settings` shows. Every fixture with a calendar or task rail, the
  connections page, the journal and search follow.
- **«Дни рождения» are unticked in the set**, as every calendar frame
  draws them and the set's README always said.
- **«Работа» holds its tasks.** `rabota-zadachi.ics` was never loaded
  into the fixture set, though the README says it goes into «Работа» beside
  `rabota.ics`.
- The task list's fixture is taken after the combined list, as the note and
  contact fixtures already are, so both task lists carry their number.

### Checks

- `docs/spec/check.py` takes only digits on a rail row for a number: «ro»
  beside a feed and a reason beside a failed collection stand where a
  number would, and are not one.
- A mistake of mine is corrected: until the subscriptions were in the set,
  the check of the agenda's rail stopped at the list of collections and
  never reached ticks and numbers. From that I had told Typecase that the
  card frames disagree with the list frames on events. They do not. Task 3
  in `docs/plan/to-typecase.md` now holds only what is true of the notes,
  and says so; task 4 asks about the feeds' colours, which are outside the
  palette a feed's colour is chosen from.
- What stays red is the server's: a calendar's number in the rail. C-6.3
  gives each calendar the count of its events, the frames draw it, and the
  server shows a dash, because the agenda reads a week. That is a question
  for the owner.

### For consumers

Nothing in the running server.

## v0.2.47 — 2026-09-25

### Changed

- **A person's timeline has its own rail, «Where to look»** (`s-contacts-7`;
  owner's decision 24.09.2026). It shows the calendars with the calendar's
  own ticks — one selection for both, so the rail shows exactly where the
  timeline looks. Each counts what it found of this person, and the
  combined row sums them. A calendar that did not answer says why and
  offers Retry, as in every rail since v0.2.46. Until now the timeline
  showed the contacts' back rail.
- The way out is «Back to list», to the contacts list, as the frame draws it.
- Apply in that rail changes the calendar's ticks and comes back to the
  timeline, which asks its sources again (`/app/sources/calendar?back=…`).
  The way back is taken only to a timeline of this Carrel: a path under
  `/app/contacts/` ending in `/timeline`, no host, no query. Anything else
  goes to the calendar, as before.

### For consumers

Nothing new in `view`: the rail is an ordinary `Rail`. The fixture
`app-contact-timeline` carries it.

## v0.2.46 — 2026-09-25

### Added

- **A rail says what did not answer, and offers Retry** — as eleven frames
  have long drawn it and the contract had no room for. `SourceRow` gains
  `Message` (a few words beside the name: «password · 401», «error · 503»,
  «refused · 403», «no reply», «unreachable»), `Detail` (the whole sentence,
  for a hover: «401 Unauthorized — the saved password was refused. Open
  Connections to update it.») and `Retry` (`source.retry`, the poll's retry
  of that source alone).
- The rail of a combined view marks every source the poll could not read:
  refused, timed out or unreachable. Until now it marked only an unreachable
  server, and said nothing of why; a refusal left the row looking fine.

### For consumers

- **`view`:** `SourceRow.Message`, `SourceRow.Detail`, `SourceRow.Retry`
  (JSON `message`, `detail`, `retry`). Additive; every fixture with a rail
  carries them, and `app-contacts/unavailable` shows the refused book.

## v0.2.45 — 2026-09-24

The fixture set follows the mockup of 24.09.2026 (docs/plan/from-typecase.md).

### Changed

- **«Back to list» from every card**, an event's too: «Back to calendar» is
  gone (`s-agenda-2`).
- **Connection labels in the fixture set are one word**: `nextcloud`, not
  `nextcloud · home`; `radicale · vps` stays two, as the hard case. Nothing
  in the server changes: a stamp is still the label and the collection's
  name. Every fixture carrying the old label follows, and so does
  `docs/visual/fixtures/README.md`.
- **«Дневник» is ticked in the fixture set**, as `s-notes-1` now draws it:
  its note stands in the combined notebook and the rail counts it. The
  fixtures of the notes and of search follow.
- The fixtures of the note list, the note card and the contact form are
  taken after the combined view, as a person comes to them: that is where
  the counts of every ticked collection in the rail come from
  (`s-contacts-3` draws «Архив» with its number).

### Spec and checks

- `docs/spec/check.py`: the `FRAME_TODO` lines the new frames closed are
  gone; `app-contact-new` is a sheet and has no rail of its own. What the
  new frames still disagree on among themselves is written down, one line a
  collection, and asked of Typecase (`docs/plan/to-typecase.md`, task 3):
  the cards of events and notes draw «Дни рождения» and «Дневник» otherwise
  than their lists, and put a number on calendars, which have none anywhere.
  The check is green.

### For consumers

Nothing to change in the running server beyond the label of the way back.

## v0.2.44 — 2026-09-24

### Changed

- **A new device's password is on its own page** (owner's decision
  24.09.2026; `s-plan-2`, `s-plan-4`, `s-plan-5`). Handing out the password
  is one of the page's main jobs, and «Show password» no longer sends
  anyone to Connections.
  - **Android:** a card for every ticked account, all at once — its code,
    «Open in DAVx5» and its password under the code with Copy. The one code
    of the first account and the «Code for …» actions are gone.
  - **Thunderbird** and **Anything else:** the login and the password in the
    account's row above its collections, with «Copy password».
  - The password is shown inside the one unlock window (S-10.7), and every
    showing is journaled (`connection.password.reveal`). Outside it the
    answer carries no password: Show opens the same sheet over this page,
    and Copy waits for it.
  - Apple has no password field: the profile is issued inside the window.

### For consumers

- **Breaking (`view`):** `DevicesPage.QR` and `DevicesPage.Password` are
  gone. `DeviceAccount` gains `QR`, `Login`, `Password` (`SecretField`) and
  `Actions` — `device.davx5`, `device.password.copy`. The fixture
  `app-settings-devices` follows.

## v0.2.43 — 2026-09-24

### Added

- **A davloom device is read only or read and write** (owner's decision
  24.09.2026, revisiting S-27; `s-davloom-1…3`). The add form asks «What
  this device may do: Read only | Read and write», read and write by
  default. The password's legend says what was chosen: «Phone · Password
  created · read only · 1 collection». The device's row switches it —
  `davloom.access.read` / `davloom.access.write`, `On` on the one in force —
  without a new password; the journal writes `device.access`.
- The proxy refuses a read-only device `PUT`, `DELETE`, `MOVE`, `COPY`,
  `PROPPATCH`, `MKCOL`, `MKCALENDAR` and `LOCK` with 403 and
  `need-privileges`, before anything reaches the collection's server, and
  lists its collections with the read privilege alone, so a client shows
  them read-only. There is no ceiling on the whole proxy; a collection
  read-only for its connection stays so for every device.

### Fixed

- `TestAuthenticateUnknownLoginCostsAKDF` measures the sign-in KDF at the
  cost it ships with. With the cheap test values of v0.2.30 the KDF was lost
  in the rest of a sign-in, and the test failed on noise in a full run.

### For consumers

- **`view`:** `ProxyDevice.ReadOnly` (JSON `readOnly`). Additive; the
  fixture `app-settings-davloom` carries the new `access` row of the add
  form.

## v0.2.42 — 2026-09-24

Labels tell sources apart (docs/plan/from-typecase.md, owner's decision
24.09.2026).

### Changed

- **A taken connection label is refused** (`s-profile-11`), on Connect and on
  Edit: the label field says «There is already a connection called “…”.
  Choose another label: it is how your sources are told apart.», and nothing
  is saved. What is compared is the label a connection acts under — without
  one, the server's host — so a second connection to the same server without
  a label is refused too. Letter case does not tell two labels apart. A
  label typed in is checked before the server is asked; the host is checked
  once discovery has named it.
- **Two collections of one connection under one name** — Nextcloud allows
  it — each carry their address name in brackets: «Carrel S-15
  (carrel-s-15-2)». It is the collection's label, so the rail, every stamp,
  the source column and the poll show it; others keep their names as they
  are. Only collections that share a kind count: a calendar and an address
  book of one name never meet in a rail. The mark is worked out when the
  catalog is opened (`sealed.Collection.Twin`) and never stored.
- A collection without a name is labelled by its address name decoded —
  «Мой календарь», not `%D0%9C%D0%BE%D0%B9…`.

### For consumers

Nothing to change: labels and stamps are text as before (C-4, C-6.2 say
what they carry).

## v0.2.41 — 2026-09-24

The sheets follow their frames (docs/plan/from-typecase.md, 24.09.2026).

### Changed

- **Attach a calendar** (`s-mail-10`): the menu item is «Calendar», not
  «Calendar for a range», and its key `calendar`, not `range`. The sheet asks
  «What goes in: Whole calendar | Dates» in `MailAttachSheet.Range`, whole by
  default; From, To, «Both days are in the file.» and Show come with Dates
  only. The whole calendar goes into the file as every event it holds.
- **Attach a note** (`s-mail-5`): one choice, «Send it as: Markdown · .md |
  Source · .ics», Markdown by default, with the hint the frame gives, and one
  Attach; the preview names the choice. The two «Attach as …» actions are
  gone.
- **Publish** (`s-publish-1…5`): the lifetime is a field, «Link lives for» —
  7, 14 or 30 days, or No expiry; 30 by default — and there is one «Create
  link» beside Cancel. «Publish for 30 days» and «Publish without an expiry»
  are gone, and `PublishSheet.Expires` is empty until the link exists.
  `expires=never` in a post is still honoured.
- **Publish a calendar** (`s-publish-2`): «How much of it: A date range | The
  whole calendar», a range by default, From and To with the range only. A
  range posted without its days brings the sheet back with the days asked
  for, rather than a guess. «Show event descriptions» starts unticked; «Allow
  .ics download and subscription» still starts ticked.

### For consumers

- **`view`:** `MailAttachSheet.Format` (`*Form`, JSON `format`) — the note's
  choice of format; nil for every other kind. Additive.
- The attach menu's calendar entry is `mail.attach.calendar` with
  `what=calendar`; the publish sheet's form carries `lives` on every sheet and
  `how` on a whole calendar's. Fixtures `app-mail-compose` and `app-publish`
  follow.

## v0.2.40 — 2026-09-24

### Added

- **DAViCal is run, and all seven suites are green** — records, forms,
  collections, move, search, timeline, collection delete. It stands in the
  ordinary row of `linux-gates.sh`. The stand is `fintechstudios/davical:1.1.12`
  (the last DAViCal release) over PostgreSQL 15; the image makes the database
  and the DAViCal admin itself. The suites run as an ordinary principal, not
  as the admin, made by `testdata/davical/make-account.sh` through DAViCal's
  own admin form.

### Changed

- **An empty address-book search is checked against the book** (SV-51), as a
  calendar's already is (SV-47). DAViCal answers a search for Cyrillic in
  another case with nothing whenever EMAIL or TEL stands among the
  alternatives, even for a card whose FN holds the text. The book is read
  whole — through the session cache, so once per change — and sifted here;
  what that finds comes with a note on the source («the server's search
  found nothing here, so Carrel sifted the address book itself»). An honest
  miss still says nothing. `card.Provider` gained `ReadWhole`, and
  `card.ErrServerMissed` names the case.

### Known

- DAViCal escapes a collection's name once more on the way out (SV-50):
  «Поездки & планы» comes back, to every client, as «Поездки &amp; планы».
  The catalog holds what the server shows, as everywhere (S-12.2).

### For consumers

Nothing to change.

## v0.2.39 — 2026-09-24

### Changed

- **Links inside an imported file travel with it** (S-19, owner's decision).
  A record of the file that names another of the file by UID —
  `RELATED-TO` in a calendar; `RELATED`, `MEMBER` or
  `X-ADDRESSBOOKSERVER-MEMBER` on a card, bare or as `urn:uuid:` — names it
  under the identity it ends up with. Every identity is settled before the
  first write, so a record renamed by Carrel because its UID was taken is
  followed at once. Where the server gives its own UIDs (Google, SV-25), the
  records holding such links are written once more after the import, with
  If-Match. The row of each such record says so, and says why when that
  second write was refused.
- Records already on the servers that name an imported record's former UID
  are not touched: where a UID in a file came from is not known.
- A link to a UID the file does not hold goes as it came. Of several records
  of a file under one UID the first keeps it, and links lead to it.

### For consumers

Nothing to change.

## v0.2.38 — 2026-09-24

### Added

- **Davis is run, and all eight suites are green** — records, forms,
  collections, move, search, timeline, collection delete and, new here,
  files, from `/dav/home/<login>/`. It stands in the ordinary row of
  `linux-gates.sh`, with no flag.
- The stand's one DAV account is made by `testdata/davis/make-account.sh`
  through Davis's own admin form (login, «new user», CSRF token included),
  not by hand. It does nothing when the account already answers. The suites
  default to the account it makes, `carrel` / `carrel-pass`.

### Fixed

- The Davis stand itself: `davis-setup` now gives `/data` to `www-data`,
  which PHP runs as — a database made by root was read-only to it — and
  makes the WebDAV directories, without which every DAV request was a 500.
- The contact forms suite never checked categories on any server: the seed
  card had none, so the check was skipped as «not taken by the server». The
  seed has them now; green on Davis, Radicale, Xandikos, Baikal, Nextcloud,
  SoGo and Yandex.
- The collections suite's seed address book has a name, as Carrel's form
  always gives one: Davis answers a nameless one with 500 (SV-49).

### For consumers

Nothing to change.

## v0.2.37 — 2026-09-24

### Fixed

- **An import's report names the identity the server gave.** Google keeps
  every new card under a UID of its own (SV-25); the report said «Created»
  under the UID the card was sent with, one the server does not hold. After
  the last record the destination is now read whole once, a record not found
  under its UID is read at the address it was written to, and its row says
  «Created under …: the server gave it an identity of its own» and counts it
  among the renamed. A record shown neither way says so. On a server that
  keeps what it is sent, the cost is that one read.
- What pointed at the UID an imported card came with is left alone: an
  import is a copy, and a card of that UID may live on elsewhere.
- The partial fixtures of the other kinds took any source still querying for
  the held one, and so were taken before a source of the running stand had
  answered. With every source now asking its change tag first, that moment
  grew long enough to be caught in a full run. The steady point now holds
  only the stalled stand's sources.
- `go test -race` on v0.2.36 in `golang:1.27-bookworm`: no data race.

### For consumers

Nothing to change.

## v0.2.36 — 2026-09-24

### Changed

- **A week seen before is not read again** (S-4.2). A calendar read by
  window — an agenda's week, a month, a range — is kept under that window
  and its collection's change tag, the server's answer as it came: going
  back to a week costs one PROPFIND for the tag. Another window is its own
  read, and a change to the calendar reads every window again. The week is
  not cut here out of a whole read: what falls in a window stays the
  server's judgement.

### Fixed

- `TestLeavingThePollLeavesNothingRunning` passed or failed on which
  goroutine ran first: leaving the page cancels the task, and a task
  cancelled before its source asked anything left nothing to cut (twice in a
  hundred runs on the Linux machine). The test now leaves once the far
  request is out; a hundred runs there, none failed.

### For consumers

Nothing to change.

## v0.2.35 — 2026-09-24

### Changed

- **A search of a collection already read whole asks the server nothing but
  its tag** (S-4.2). When a book or a calendar's component is kept and its
  change tag has not moved, a search sifts the kept records here, through
  the properties and with the comparison it would ask the server for: letter
  case does not count, in any script. A changed or never-read collection is
  searched on the server as before.
- **The check of an empty calendar search is paid once.** A search the
  server answers with nothing is checked by reading the component whole
  (SV-47); that read now goes through the session cache and is kept, so the
  next search of the same unchanged calendar is sifted from it.
  `cal.Provider` gained `ReadWhole` for this: unset, the check reads the
  server as before.

### For consumers

Nothing to change.

## v0.2.34 — 2026-09-24

### Changed

- **A collection read whole is kept, and read again only when it changed**
  (S-4.2). Contact, task and note lists, the timeline, the poll and the
  sources of a search read their collections whole; each such read is now
  kept in the session cache — which records at which version, and the
  bodies by path and version — under the collection's change tag. The next
  read asks the tag first: one depth-0 PROPFIND for `getctag` or
  `sync-token`. The same tag means no REPORT at all. Until now the tag was
  collected by discovery and never compared, and every screen read every
  collection whole on every load.
- A server with no change tag gets the soft limit instead:
  `CARREL_CACHE_COLLECTION_TTL` (60 s by default), measured from the read.
  Within it another client's write is not seen on a plain load; a write made
  through Carrel drops the kept reads of its collection at once, tag or no
  tag.
- **Refresh reads the servers again.** Refresh of a combined view — the top
  bar's and the poll bar's `poll.again` — now carries `again=1`, the same
  parameter a search and a timeline are run again by, and all three drop the
  session's kept reads first: asking again means what the servers hold now,
  within the soft limit too.
- What is not kept: a read by window (an agenda's week), a filtered search,
  and a read that came back with records Carrel could not parse — the list
  names those, and a kept read would hide them. A tag that cannot be asked
  sends the read to the server; a server that does not answer shows the
  refusal, never what was kept.

### For consumers

A frame follows the href it is given, so nothing to change there; the
fixtures of `app-contacts` and `app-agenda-all` now show Refresh as
`/app/contacts?again=1` and `/app/calendar?again=1`. Lists of large books and calendars on servers with a
change tag load without reading the collection again; expect one small
PROPFIND per collection per screen instead.

## v0.2.33 — 2026-09-24

### Added

- **The race detector has been run, and finds no data race.** It needs cgo
  and a C compiler, the Linux machine's host has neither, and nothing was
  installed on the host: it runs in `golang:1.27-bookworm` with a module
  cache volume of Carrel's own. It is now a step of `linux-gates.sh` behind
  `CARREL_RACE=1`: about seven minutes, worth running now and then rather
  than every time.

### Fixed

- **A test counted on how fast the machine is.** `TestHealthzAndAboutUnmetered`
  spent the session's bucket and expected the refusal at exactly request 121.
  The bucket refills as it drains, two tokens a second. Under the race
  detector every request is slow enough for a token to drip in, and the
  121st passed. The test now holds what it is for: requests far faster than
  the refill meet a 429 soon after the bucket is spent.

## v0.2.32 — 2026-09-24

### Fixed

- **The new gofmt gate failed on its first run, over a file untouched since
  v0.0.125.** The step called a bare `gofmt`, and on the Linux machine that
  is the host's own, go1.26.5. `GOTOOLCHAIN=auto` switches the `go` command
  to the 1.27 that go.mod asks for, but not a separate binary on PATH. 1.26
  and 1.27 format a multi-value return of composite literals differently,
  so `internal/web/photo.go`, formatted by 1.27, read as unformatted. The
  gate now calls the toolchain's own gofmt (`$(go env GOROOT)/bin/gofmt`).
  A formatting check has to run on the version the code is built with.

## v0.2.31 — 2026-09-24

### Fixed

- **A card moved into Google did not finish moving.** Google rewrites the
  UID of what is written to it (SV-25), and the move read its copy back by
  the UID it had sent. The copy was not found, so the original was kept: two
  cards, and the notice's link pointed at the old UID, which Google does not
  know. The copy is now read back at the address it was written to, as
  `finishCreate` reads back a new card. The move then finishes: the original
  is deleted, and what pointed at the old UID is pointed at the one Google
  gave (v0.2.30). With that, all three operations that change a card's UID
  mend its links. An identity the server chose is not counted as a loss when
  the copy is compared.
- **A copy into Google landed on an error over a record written whole.** A
  copy is a record made in another collection, and now goes through
  `finishCreate`. On a server that keeps the UID nothing changes; on Google
  the page opens the copy.

### Added

- **A calendar record is now run on Google.** Until today no suite had
  written a single event there, and the coverage table said so. A calendar
  cannot be made on Google, so the shared run does not fit.
  `TestGoogleEventInTheExistingCalendar` takes an event through its whole
  life in the calendar the account already has: made, read back, edited and
  deleted, through the pages. It is set in 2031 so that it meets nothing of
  the owner's, and no attendee is ever added, or Google would send an
  invitation. The first run tripped over an ATTENDEE that Google itself
  writes into its e-mail reminder, addressed to the calendar's owner. That
  is not an invitation, and the check now looks past reminders.
- **gofmt and vet are gates.** Both lived only in the manual race script,
  and an unformatted import order went into v0.2.30 unnoticed. It is
  formatted now.

- **A third test that failed only in a full run, and said nothing about
  why.** `TestOneConnectionOfThreeDown` opens seven screens, each starting
  a poll of its own, and then searches. A session runs at most four polls at
  once. Alone, the earlier ones are done before the search starts. Under
  load some may still run, and then the search is refused as one too many:
  the page's poll is a zero value, the wait ends at once, and the test
  reads «the answering connection lost its result». That is the ceiling
  working, not the dead connection winning. The test now waits until the
  search is let in, and on failure names the notices and every source's
  state. The mechanism is read from the code; it was not reproduced, so if
  the test fails again it will at least say why.

- **The unit suite takes three minutes instead of ten, and no longer runs
  out of time.** A full run was killed at go test's ten-minute ceiling
  mid-test, which reads as a failure and was none. Ranking the package's 337
  tests by time showed no slow test: every one paid about a second and a
  third up front. Every fixture makes an account and signs in, which means
  four Argon2id derivations at the production cost of 64 MiB (256 MiB for
  the escrow master key), all behind one process-wide semaphore. The tests
  of `internal/web`, `internal/user` and `internal/proxy` now use the
  cheapest parameters the store accepts. The parameters travel with every
  stored record, so a cheap one is checked the same way, and nothing in
  those packages tests the cost itself. `internal/crypt`'s own tests keep
  the production values. `internal/web` alone went from 453 s to 78 s, and
  `internal/user` from 92 s to 5 s. The gates also got an explicit 30-minute
  ceiling, so an arbitrary default no longer decides.

### Notes

- **Google stays on its undocumented path for now.** That is the owner's
  decision (24.09.2026): when Carrel has OAuth 2.0, the connection moves to
  Google's documented addresses. Recorded in the plan.
- Import into Google is still not covered: an imported card gets Google's
  UID too, and a link to its old one would break.

## v0.2.30 — 2026-09-24

### Fixed

- **A note or a task could drop out of a person's timeline after something
  Carrel did to the card.** The owner caught it. Write a note and New task on
  a person's timeline tie the new record to the card by RELATED-TO holding
  its UID, and by nothing else. An event carries the person's address in
  ATTENDEE as well, so events were safe. Three operations of Carrel's change
  the UID a card is known by: a move within SoGo, which gives the card a
  UID of its own (SV-18); a merge of duplicates, which deletes the other
  cards (S-16); and a write to Google, which rewrites the UID (SV-25). After
  the first two the records pointed at nothing and left the timeline without
  a word. They are now pointed at the new UID, in the ticked collections —
  where the timeline looks, the owner's decision — each record read whole,
  each value keeping its parameters (RELTYPE=SIBLING is another relation
  than a bare value, which reads as PARENT), each write conditional on the
  version read. A merge moves the links before it deletes anything. The page
  says how many records were pointed anew and names each one that could not
  be, with why.
- **A card moved within SoGo is now found under its new identity.** The
  destination is listed before the move, so afterwards the card is the one
  resource that was not there. The notice opens the card itself rather than
  the collection it is somewhere in. With no new resource, or several, it
  still opens the collection: that would be a guess.

### Notes

- Google is the third case and is not covered yet. A card moved into Google
  does not finish moving today: the copy is not found by its UID, so the
  original is kept, its links stay whole, and the person ends up with two
  cards. That is a defect of the move's copy path, recorded in the plan with
  the rest.
- The rule is in S-15.4: an operation after which a card is known by another
  UID must point everything that referred to the old one at the new.

## v0.2.29 — 2026-09-23

### Fixed

- **A resource that is not there was taken for one that is**
  (docs/servers.md, SV-48). A multistatus response carries either propstat
  blocks or a status of its own (RFC 4918 §14.24), and Xandikos says a
  missing event, card or anything else the second way: 207, one response,
  404. The decoder read only propstats, so
  such a response came out with status 0 and no properties, and `Stat` —
  which skipped only non-zero non-2xx statuses — built an entry from the
  href. On such a server every free name looked taken, and Carrel checks a
  name through `Stat` before an upload precisely because some servers ignore
  `If-None-Match`. The fix is in the decoder, so every PROPFIND reader gets
  it, not only `Stat`.
- **The gates could not bring Stalwart up; alone it came up fine.** Two
  missing flags. `run stalwart-setup` without `--no-deps` brought the
  dependency up to *its own* configuration, where recovery mode is unset and
  so off, and recreated Stalwart out of recovery mode before the plan ran.
  And the last `up -d` without `--force-recreate` could find nothing to
  change, so the listener the plan had made never started: port 8088
  published, nobody behind it. My manual run had `--force-recreate`; the
  gates did not.

### Changed

- **The coverage table in `docs/servers.md` was wrong, and has been checked
  row by row.** The owner caught Google and iCloud marked «no CalDAV». Both
  serve it. Google's answers an app password at `google.com/calendar/dav/`
  (SV-24). iCloud's refuses *our* Apple ID, whose calendar service is
  switched off (SV-28): a property of the account, which I had written down
  as a property of the server. The first edition had been filled in from
  memory. This one is checked against `grep '^func Test' integration/`, the
  SV entries and live probes, and says «not checked» where nothing is known.
  That is Mail.ru's CardDAV, for one: no address for it exists in our
  credentials, yet the first edition said «yes».
- **Then the owner asked for the whole table to be checked against primary
  sources, and it moved again.** Every protocol cell is now sourced from the
  server's own documentation, which is linked under the table, and confirmed
  by a live probe of our stand or account. Three findings. OpenCloud declares
  CalDAV and CardDAV through an integrated Radicale since 2.3.0; the first
  edition had «no», and on our instance it is switched off (`/caldav` answers
  405). Mail.ru declares CardDAV (`carddav.mail.ru`) and WebDAV for its Cloud.
  And the one that matters most: **Google documents CalDAV and CardDAV only
  through OAuth 2.0 and says Basic gets a 401.** The addresses Carrel uses
  with an app password (SV-24) are not in its documentation, so Carrel works
  with Google on behaviour Google does not promise, and Carrel has no OAuth 2.0.
  That is recorded for the owner to decide on.
- The table now says where each check runs. Only the stands that come up on
  the Linux machine run in the gates. Everything remote runs by hand, from
  wherever `dev/credentials.txt` is, because that file never travels to the
  machine. The paragraph above the lineup had said that Baikal and SFTPGo
  run in the gates. Their steps are there, but they always skip.
- **Xandikos does not serve files, whatever it accepts.** A probe showed a
  plain folder taking MKCOL and a file coming back, and I put that down as a
  capability and ran the files suite against it. The owner pointed out that
  Xandikos promises no such thing. Its README calls it a «CardDAV/CalDAV
  server», lists core WebDAV «except for COPY/MOVE/LOCK» and WebDAV Mount as
  «intentionally not implemented». The suite is gone and the row is back to
  «no». The table's protocol columns now come from what a server documents,
  with probes only confirming it: behaviour a server never promised is not
  something to build checks on. SV-48 stays, because a missing event or
  card draws the same answer.

## v0.2.28 — 2026-09-23

### Added

- **Xandikos runs green, all seven suites, and stands in the ordinary row
  of the gates** beside Radicale and Stalwart. It took six fixes on our side
  (SV-40 to SV-44, SV-47) and taught two of the live suites to ask the server
  about itself instead of carrying a list of names: whether it checks
  passwords at all, and whether it has MOVE.

### Fixed

- **A calendar search found nothing on a server whose text-match barely
  works** (docs/servers.md, SV-47). Xandikos matches only a whole value, and
  only one with no space in it: «Two words» does not find «Two words». It
  answers 207 with nothing, which is exactly what an honest miss looks like.
  An empty calendar search is now checked against the collection itself and
  sifted here — the owner's decision. The note that Carrel did the searching
  goes on the screen only when the sifting found what the server did not; an
  honest miss on a server that searched well stays silent.

### Notes

- **The cost of that check, and the cache that would remove it.** Every
  calendar search that finds nothing now reads the whole component, on
  every server. The owner pointed out that what is read once should be
  searched from memory afterwards — and S-4.2 already prescribes exactly
  that: a version map per collection checked against its change tag, bodies
  kept by version. **That cache is not in the code.** The change tag is
  collected by discovery and stored in the catalog, and never compared
  before a read; the slot called the version map holds the index behind the
  search box's hints; bodies are kept only for the conflict flow. Every
  list, agenda and search reads its collections whole, every time. Building
  S-4.2 as written is now a task in the plan, and it pays off on every
  screen, not only on search.
- **A password cannot be cleared through the connection form.** An empty
  field means «leave it as it is», so an empty password cannot be said —
  and Xandikos asks for none. Harmless there, since it takes anything, but
  it is a gap in what the form can express, and it belongs to the frame.
  Recorded in the plan.
- **On a server without MOVE a moved record is held to «nothing lost», not
  «nothing changed».** It goes over as a copy, and a copy is written: it
  carries a stamp of its own and the writer's order of properties. The
  suite finds out which standard applies by asking the server with one
  probe MOVE of a scratch card.
- The probe was written once and not called at all: the first edit script
  failed its own anchor check and wrote nothing, the second added only the
  function, and `go vet` does not flag an unused package-level function. The
  live run caught it by failing on the old message.

## v0.2.27 — 2026-09-23

### Added

- **Stalwart runs, and its seven live suites are green** — the fifth engine
  in the lineup and the only one on Rust, sharing no ancestry with sabre/dav,
  Radicale or SoGo. It stands in the ordinary row of `linux-gates.sh` beside
  Radicale now: no flag, and a failure fails the gates like any other.

### Fixed

- **Discovery walked into the address `.well-known` had already given it**
  (docs/servers.md, SV-45). The probe carries no credentials — RFC 6764 §6
  asks it for the Location, not for the resource — but the client followed
  the redirect to the end and judged by the final status. Stalwart sends
  `/.well-known/caldav` to `/dav/cal`, which answers 401 to anyone who has
  not signed in, so a perfectly good 307 became a failed step; and since
  Stalwart's root is its web interface, answering 200 to PROPFIND, the
  fallback to the entered address did not save it. All seven suites failed
  at discovery. The redirect is now read, not walked into.
- The refusal of an HTTPS-to-HTTP redirect used to be the client's, and its
  reason reached the connection trace that way. The probe no longer lets the
  client follow, so the probe makes that refusal itself — and still puts the
  reason in the step's detail, which is what SV-10's test was written to
  protect. The test was rewritten around a real downgrade rather than a
  simulated one.

### Added, later the same day

- **`docs/servers.md` now carries a table of what each server offers and
  what the live suites actually run against it.** It exists because a claim
  of mine — that Stalwart was the only server in the lineup serving all
  three protocols — was wrong three times over: Nextcloud and Davis serve
  files as well, and Davis's were switched off by a flag of ours. Without
  the table «probably covered» cannot be told from «covered», and a gap is
  found in conversation rather than on purpose.
- **The gap it found on its first day: files were run against no server that
  also serves calendars and contacts.** `TestSFTPGoFiles` was written for
  SFTPGo alone, and SFTPGo and OpenCloud serve nothing else. The checks are
  now `runFiles`, which takes a server, and Nextcloud and Stalwart are run
  against their files — both green. Nextcloud's is worth more than the
  coverage: that account's own path holds a space, so every address arrives
  percent-encoded, which is the shape that breaks a file layer building
  addresses by hand. Davis's WebDAV is switched on and waits for the account
  it needs by hand, like its other suites.

### Notes

- **A second test whose margin was the thing that failed.** The per-ask
  budget check ran four asks' worth of work against a budget 25% larger than
  the work; on a loaded machine a sleep overruns that, the budget ends the
  run, and the test waits three seconds for a source that has already given
  up. The work now sits at 40% of the budget and the two budgeting rules
  still tell each other apart. Like the partial-screen fixture earlier
  today, it passed alone and failed in a full run — which is the only place
  either of them was ever going to fail.
- **What the stand cost, and why it is written down** (SV-46 and
  `testdata/stalwart/README.md`): recovery mode runs the management listener
  and **nothing else** — an ordinary account authenticates on it and is then
  refused, and even the administrator's DAV homes answer 404. Every 403 and
  404 read as a missing permission for an hour. The stand now comes up in
  recovery mode to be provisioned and again without it to serve DAV.
- The shape of `Account.credentials` is not what the documentation or the
  schema says: `describe` calls it a list, a live account returns a map, and
  the element type cannot be described at all. Eight guesses failed. What
  worked was making the server write the value itself —
  `update Account <id> --field credentials/0/secret=…` — and reading back
  what it produced. The key is the element's index as a string.
- Also invented by me and not by Stalwart: `NetworkListener.useTls` does not
  exist, and `SystemSettings` requires a `defaultHostname` the stand has no
  use for. Both are out of the plan.

## v0.2.26 — 2026-09-23

### Changed

- **The catalog is a cache of the server's answer, not the person's data,
  and S-12.2 now says so.** The address, the name, the colour, the component
  set and the kinds derived from it belong to the server; a person may *ask*
  for them when creating a collection, and whether the ask was taken is known
  only from the server's reply. So each is read back after a create: where the
  difference can be mended it is sent again on its own (the name, SV-17; the
  colour, SV-41), and where it cannot, the catalog takes what the server holds
  (the component set, SV-43). Written from the form instead, the difference is
  visible to nobody but Carrel — other clients show their own, and a record
  that lands in a collection outside our kinds appears on no screen at all.
  The ticks, the duplicate decisions, the attachment folder and the
  credentials are the other kind of thing: the server does not hold them, and
  they never give way to it.

### Fixed

- **A search died on any address book holding one accented name**
  (docs/servers.md, SV-42). Xandikos answers 500 to any text-match over a
  book with a value outside ASCII — its own UnicodeEncodeError, whatever the
  text asked for — while the same book without a filter hands every card
  over. A filter that breaks the server is now a third reason to read a book
  the round-about way, beside "there is no such report" (SV-23) and "the
  report is empty though the book is full" (SV-26): the book is read without
  a filter, the sifting is ours, and the screen says so. Narrow on purpose:
  only a 500 to a request that carried a filter.
- **Creating a calendar recorded the kinds the form asked for, not the ones
  the server reports** (docs/servers.md, SV-43). On Xandikos every calendar
  holds every component and cannot be told otherwise — the set is dropped at
  MKCALENDAR and a PROPPATCH of it is refused 409 — so a calendar made for
  events also takes tasks. With the form's narrower set in the catalog, a
  task written there by another client would have shown nowhere in Carrel.
  Discovery has always derived the kinds from the server; creation was the
  one place that did not.
- **A record could not be moved at all on a server without MOVE**
  (docs/servers.md, SV-44). Xandikos answers 405 to every MOVE, across
  collections and for a rename alike, and offers no COPY either. A 405 or 501
  now means "no such method here", and the record goes over the way it goes
  to another server: the copy is written and read back, the loss check runs,
  and only a copy that is really there costs the original. A refusal about
  *this* record — a precondition, a right, a taken name — still reaches the
  person as a refusal.

### Notes

- The live suite stopped testing the server where it meant to test Carrel.
  Whether a server checks passwords at all is now asked of the server rather
  than assumed: Xandikos checks none, and the half of the connection check
  that needs a refusal is skipped there with that said aloud. The component
  set is likewise no longer required to be exactly what was asked for; what
  is required is that the catalog agrees with the server.
- **The unit suite ran nowhere but a developer's own machine, and had been
  red for three weeks.** `linux-gates.sh` ran one crypt test, the image
  build and the live suites — never `go test ./...`. So nothing caught that
  `view`'s fixture check has failed since v0.2.1: fixtures for sixty screens
  went in without the entry the test asks for in the same commit, and two
  screens of sixty-two were being checked. The suite is now the gates' first
  step, and the map holds every screen on disk. Nothing was actually adrift
  — every fixture decodes into its view model with no unknown fields and
  round-trips — but for three weeks nobody could have known that. One screen
  turned out to be a fragment rather than a page (`app-note-edit` is a
  `NoteDoc`), and now says so.
- **A fixture comparison could take an empty screen for a partial one.**
  The combined agenda's partial state waited for a steady point — nobody
  queued, one source held — which is also true the instant the poll starts,
  before anything has answered. Alone the test passed; in a full run under
  load it took that instant and the fixture rightly refused it. The point
  now also requires a source that has answered, which is what «partial»
  means.
- **The gates could be brought down by a stand behind a flag.** The Radicale
  step deliberately runs the whole integration package — remote suites skip
  themselves when their credentials are unset, and that is how a new one
  never gets forgotten — but the local stands skip by *reachability*. A
  Xandikos left up by hand therefore ran its suites inside that step, and
  set -e ended the run before Stalwart and Davis had their turn, although
  each has its own isolated step. The gates now start with nothing of ours
  running.

## v0.2.25 — 2026-09-23

### Fixed

- **A contact could not be opened on Xandikos, and the list showed it the
  whole time** (docs/servers.md, SV-40). The list goes out without a filter;
  `FindUID` went out with `match-type="equals"`, and Xandikos answers that
  with 207 and no responses at all — the same answer it gives for a card
  that is not there. Nothing on the wire says the filter was the problem, so
  there was nothing to fall back from: the card page was a 404, and so was
  the edit. `FindUID` now asks for the default match, a substring, and
  checks the exact UID here, as it already did. Asking a server for a
  narrower match than we need was gratuitous; `dav.CardMatch.Equals` stays,
  unused, with the silence it can buy written at the field.
- **A colour given at creation could be in the catalog and nowhere else**
  (docs/servers.md, SV-41). Xandikos answers MKCALENDAR 201 and names
  `calendar-color` in a 404 propstat of the `mkcalendar-response`: the
  calendar is made, the colour is dropped. Creating a collection put the
  colour the person typed straight into the catalog without reading it back,
  so the swatch would have been one no other client draws — while C-10 went
  on calling it the colour the server sent. The colour is now settled the
  way the name (SV-17) and the address (SV-19) already were: read back, sent
  once more on its own, read again, and what the catalog keeps is what the
  server holds.

### Notes

- Both were found the first time the pinned Xandikos stand came up and the
  live suite ran against it, and both are ours, not its. The lesson is one: **a server answering 207 with
  nothing is indistinguishable from a server that has nothing.** RFC 6352
  §8.6.1 asks for 403 with `supported-filter` instead; Xandikos does not
  give it. The only defence is not to ask for what is optional when the
  answer can be had without it.
- The stand can now play both quirks (`davtest.Server.DeafToMatchType`,
  `davtest.Server.CreateDropsColour`), and each has a sentinel that fails
  when the fix is reverted.
- Housekeeping from the first live run of the batch: Stalwart's setup is two
  `stalwart-cli` calls rather than one `sh -c` (the image has no shell), its
  admin port outside is 8089 because 8080 belongs to SelfPost, each
  unverified stand's step in `linux-gates.sh` fails alone instead of taking
  the gates with it, and Xandikos is pinned to `v0.2.12` with `user: root`.

## v0.2.24 — 2026-09-22

### Added

- **Davis**, sabre/dav with a Symfony admin over it — the same engine as
  Baikal and Nextcloud, so it joins the lineup for completeness rather
  than for news. Seven live suites, and one service: the standalone image
  carries Caddy, so there is none of the nginx-plus-FPM pairing the
  project's own compose files use. A one-shot runs the migrations, which
  Davis does not run for itself — until they have, every screen is a 500.
- Its DAV account is made **by hand, once**, and that is a finding rather
  than a shortcut. Davis has no way to create one without its admin
  screens: the API reads users and never writes them, the three console
  commands do not include one, and the row cannot honestly be written
  straight into the database — the password is a PHP `password_hash`
  despite the column being called `digesta1`, and making a user writes
  four entities at once. Reproducing that in SQL would be reimplementing
  the application, to rot at its next release. So the container comes up
  on its own and the suite takes the account from `CARREL_DAVIS_USER` and
  `CARREL_DAVIS_PASS`, skipping with that explanation when they are unset.

### Notes

- Like Stalwart and Xandikos, the stand has not been run — no Docker on
  the machine it was written on — and its step in `linux-gates.sh` is
  behind `CARREL_DAVIS=1`.
- **DAViCal stays unstarted, checked a second time.** Its tree does hold
  a command line, `scripts/davical-cli`, but it is a third-party Perl
  script with FreeBSD paths and another company's per-domain config
  convention baked in: no use to a stand. With no official image either,
  it would need a Containerfile of our own over the Debian package, plus
  PostgreSQL, on top of the same by-hand account Davis needs. What to
  decide at the machine is in the plan.
- Four links in `docs/` pointed at `docs/spec/typecase.md`, which left
  this tree on 10.09.2026 when Typecase moved to its own repository.
  They now say where the spec lives instead of pointing at nothing. No
  requirement text was touched.

## v0.2.23 — 2026-09-22

### Fixed

- **A search budget that measured the wrong thing, and a ceiling that
  counted the wrong thing.** A source had ten seconds; the work it does
  is one far request per kind the collection holds. On a server that
  answers every request in two to four seconds — one was measured that
  day — a calendar of events, tasks and notes could not be searched at
  all while an address book beside it could. The screen said «No answer
  in time», which named the server for a shortage that was ours.

  Two changes, and either alone makes things worse:

  - `fanout.Asks` — a source says how many far requests it must make and
    is given its share for each (`budgetFor`).
  - `fanout.Slot` — the ceiling of `FanoutParallelism` now counts
    requests in flight rather than sources, and a slot is taken around
    one request and given straight back. The kinds of a collection go
    side by side under the same ceiling, so the server is asked no
    harder than before and the person waits for the slowest kind instead
    of all of them in a row.

  The budget alone was the worse half: a slow three-kind source kept its
  slot three times longer and blocked others, which the fixture of a
  half-answered poll caught at once — four collections loaded became
  two.

  What is no longer promised, and should be known: the ceiling used to be
  held by the fanout whatever a source did. It is now cooperative — a
  source must take a slot for each ask, and one that asks without a slot
  escapes it. Both of Carrel's sources ask through one path that takes
  it. The sentinel is renamed for what it now guards,
  `TestTheCeilingCountsRequestsInFlight`.
- **A panic in a part of a source no longer ends the process.** `ask`
  guards a source with `recover`, and a panic does not cross a goroutine:
  asking the kinds side by side would have carried the parsing of a far
  server's answer outside the only guard there was (S-21).
  `fanout.AskingKind` gives every kind its own.

### Notes

- The concurrency is checked by reading, not by the race detector: it
  wants cgo and there is no `gcc` on this machine. What was read, and
  the run to make on a machine that can, are written down in
  `docs/plan/server.md`.

## v0.2.22 — 2026-09-22

### Added

- **Xandikos**, the second light engine beside Radicale and sharing no
  code with it: Python, every collection a git repository. Seven live
  suites wired to the same `run*` helpers as every other server, and a
  service in `docker-compose.linux.yml` that needs four environment
  variables and nothing else — no config file, no provisioning, no
  console commands.
- It is worth having for one thing in particular: **Xandikos asks for no
  credentials at all**. Carrel's connection form wants a login, because
  an address without one is a subscription rather than an account, but
  it takes an empty password — so this is the only server in the lineup
  that exercises the case where nothing is asked of us. The other eight
  all demand a password.

### Notes

- Like Stalwart, the stand has not been run: there is no Docker on the
  machine it was written on. The step in `scripts/linux-gates.sh` is
  behind `CARREL_XANDIKOS=1` for that reason. Unlike Stalwart there was
  almost nothing to guess — the whole configuration is four environment
  variables out of the project's own README.
- **DAViCal is deliberately not started.** Stalwart and Xandikos have
  official images configured by environment; DAViCal has no official
  image, only community builds that differ widely, and bringing it up
  wants PostgreSQL, `create-database.sh`, an administrator and a user
  made through a web interface. That would have been a third pile of
  guesses in a row, on an image nobody stands behind. Half an hour with
  the machine in front of you beats half a day without it; what to
  decide there is written in the plan.

## v0.2.21 — 2026-09-22

### Changed

- **The pool belongs to the guard, not to each client.** A transport was
  built for every `dav.Client`, so every client dialled its own
  connection, made about three requests on it and threw it away with its
  pool — twenty-four clients and twenty-four connections over one live
  run. `Guard.transports` now caches one transport per pinned
  certificate and every client of that guard shares its connections.

  | | Clients | Connections before | After | Run before | After |
  |---|---|---|---|---|---|
  | stand | 9 | 8 | **1** | — | — |
  | SoGo | 24 | 24 | **1** | 46.3 s | **25.0 s** |
  | Yandex | 16 | 16 | **1** | 44.5 s | **31.9 s** |

  The times are single runs on a noisy network, but twenty-three TLS
  handshakes fewer account for them without a stretch.

  Nothing about a transport belongs to one person: the dialler is the
  guard's own and SSRF-checked, and credentials ride on each request
  rather than on the connection. The one thing that does differ is a
  pinned certificate (F-13), so the cache is keyed by the pin and a
  pinned client never shares a socket with an unpinned one to the same
  host.

  The limits are choices, not findings: `MaxIdleConnsPerHost` 10, as the
  owner decided, and `MaxIdleConns` 100 — ten hosts at ten — because
  leaving the old process-wide ten in place would have made the per-host
  limit meaningless. Neither has been tested under load, because there
  is no load here to test them under, and the code says so.

## v0.2.20 — 2026-09-22

Три решения владельца, и один замер, который перевернул мой довод.

### Added

- `dav.Tally` counts what the outbound guard does — clients built,
  connections dialled, requests sent, of those the writes, and of those
  the ones off a connection the pool had kept — behind `Guard.Counts` and
  `web.Server.DAVCounts`. Four atomic adds per request; nothing in a
  screen reads them. It exists so that a question about the pool is
  settled by numbers.

### Notes

- **The pool, measured.** Over a live forms run: SoGo 24 clients and 24
  connections for 79 requests, Yandex 16 and 16 for 53. One connection
  per client, and no concurrency at all: every connection exists because
  a handler built a fresh `dav.Client`, made about three requests on it
  and threw it away with its pool. The argument I had made against
  sharing — that each client would lose its ten idle connections — was
  wrong, because there is no pool today to lose. Every live write already
  goes out on an idle connection (7/7, 8/8, 3/3), so sharing would not
  improve the resend rule at all; what it would save is some twenty TLS
  handshakes per visit. The decision and the two questions left (do it
  now, and what to set `MaxIdleConnsPerHost` to) are in
  `docs/plan/server.md`.
- **Files in a timeline keep matching by name**, and S-15.4 now says why
  that is not an oversight: an event has a strict reference and a guess
  beside it is harmful, while a file has no reference at all, so the
  choice is not «strictly or by guess» but «by name or not at all». The
  row names its ground — «name in the file name».
- **Contacts stay out of the timeline**, `MEMBER` and `RELATED` with
  them. Looking for a person in other people's cards is a search of
  mentions, not a record of time; and «these cards are one person» is
  already what linking duplicates is for (S-16). After three turns the
  owner placed them where they belong: a «mentions» view entered from a
  card — a function that has no route, no view model and no frame yet,
  so there is nothing for the server to build until one is drawn.

## v0.2.19 — 2026-09-22

### Fixed

- **Every component of a record is read, not only the main one**
  (S-15.4). A series is one record with several components, and an
  occurrence that was overridden carries an `ORGANIZER` and `ATTENDEE`
  lines of its own. Someone added to a single Thursday of a weekly
  meeting is named nowhere else in that record, so their timeline was
  empty over a meeting they were invited to. `cal.People` now gathers
  everyone from the main component and from every `RECURRENCE-ID`
  sibling. A record with no document behind it — one that was built
  rather than fetched — falls back to its main component: less than the
  whole truth, never a lie.

### Notes

- This closes the last of the three timeline gaps that were mine to
  close. The two left open are the owner's to decide: whether contact
  records belong in a timeline at all (a vCard `RELATED`, a group's
  `MEMBER` — they point at a person but carry no date, and the timeline
  is ordered by time), and whether a file matching the person's name is
  enough, given that the same reasoning is refused for events.

## v0.2.18 — 2026-09-22

Stalwart: половина работы, и сказано, какая.

### Added

- Seven live suites for Stalwart — records, forms, collections, move,
  search, timeline and collection delete — wired to the same `run*`
  helpers every other server uses, with not one new line inside the
  checks themselves. On a machine without the stand they skip, naming the
  address and the reason.
- A stand for it: a `stalwart` service and a one-shot `stalwart-setup` in
  `docker-compose.linux.yml`, with `testdata/stalwart/config.json` and a
  `plan.ndjson` that provisions a domain, an account with a password, the
  DAV listener and the default domain.

### Notes

- **The stand has never been run.** It is written from Stalwart's
  documentation on a machine with no Docker; every decision in it names
  the page it came from, and `testdata/stalwart/README.md` lists the six
  places most likely to need a correction on the day someone brings it
  up. The step in `scripts/linux-gates.sh` is behind `CARREL_STALWART=1`
  for exactly that reason: an unverified service must not be able to fail
  anyone else's gates. Stalwart is recorded in the plan as started, not
  as done.
- Stalwart serves calendars at `/dav/cal`, address books at `/dav/card`,
  files at `/dav/file` and principals at `/dav/pal`, and answers
  `/.well-known/caldav` and `/.well-known/carddav`. The suite hands
  discovery the root and lets it walk, the way it would meet a server
  nobody prepared for it.

## v0.2.17 — 2026-09-22

Два вопроса владельца, и оба оказались работой.

### Fixed

- **A person is looked for in every line that names them** (S-15.4). The
  timeline read ORGANIZER, ATTENDEE and RELATED-TO, and that is not the
  whole list. `SENT-BY` is who really sent a record — the assistant who put
  the meeting in for someone else — and `DELEGATED-FROM` names the one who
  handed participation over, who is often struck from the attendee list in
  the same breath. Both of them had an empty timeline over a record that
  names them. All three parameters are read now, and the row says which:
  «sent for the organiser», «delegated this to an attendee», «given this by
  an attendee». `CONTACT` is still not read and the reason is written into
  S-15.4: it is free text, and reading it would be looking for a name in
  the text, which this screen refuses on purpose.
- **Two more clocks that were not the server's.** The gates went red
  overnight — from the date changing, not from a change in the code — and
  three fixtures came apart. The publish sheet counted its expiry from
  `time.Now()`; it now counts from the server's clock, and «30 days, until
  9 September 2026» matches the frame word for word. And `user.Service`
  keeps a clock of its own that the screens' clock cannot reach: the date
  an account was made is written into the record, not drawn. It has a
  `SetClock` beside the server's now, and the fixture run makes the account
  on the day the frames give it — 11 February 2026, deliberately in the
  past. The admin table on the frames says `2026-02-11` too.

### Notes

- Three fixtures are rewritten with values that no longer depend on the day
  the run happens: the publish expiry, the account's creation date, and a
  last login that reads «2 min ago» instead of an artefact of an unfrozen
  clock.
- The landscape of servers is written down in `docs/plan/server.md` by
  engine rather than by name: Stalwart, Xandikos and DAViCal are the ones
  worth a local container, Davis is sabre/dav again and goes last, and
  Cyrus, Open-Xchange, Zimbra and Synology are named as out of reach of a
  container. `docs/servers.md` now says plainly that **Exchange, Microsoft
  365 and Outlook.com speak no CalDAV at all** — so nobody waits for them.
- Left open and written down: the timeline skips contact records
  altogether, so a vCard `RELATED` or a group's `MEMBER` never points at
  anyone; ORGANIZER is read only from the main component; and files are
  still matched by the person's name, which is the very thing events are
  not matched by.

## v0.2.16 — 2026-09-21

Полный живой прогон: 54 зелёных, 2248 секунд, и две настоящие ошибки.

### Fixed

- **A write that was sent twice is not a write that failed** (SV-39). A
  connection can die after the server has read the request and before the
  answer comes back. The guard then sends the write again, and the server,
  which already holds the thing, refuses the second copy: 412 to a
  conditional PUT, 405 to an MKCOL. The data was always safe — the
  precondition is what makes the retry safe at all — but the refusal
  reached the person as it came. They made a calendar and read «The server
  would not make it» about a calendar that was already standing there.
  The guard now marks a second answer as its own, and the places that
  create something read «it is already there» as their own write arriving
  twice: a record, a collection, a folder, and the copy a move makes,
  where a second write would otherwise have left two records where one was
  asked for. A write on a connection dialled for it is still never resent,
  and that message is unchanged.
- **The timeline on Yandex** is tested and understood. The note in v0.2.15
  said it could not run there because Yandex makes no address book. That
  reason was wrong: the book cannot be *made* (SV-20) but one is already
  there, and Carrel has written to it since 16.09.2026. The real reason is
  SV-38 below.

### Changed

- The timeline runs on Yandex (`TestYandexTimeline`) and asks both halves
  of S-15.4 on one event: the card of an attendee finds nothing, because
  the server kept no attendee, and the card carrying the address the
  server itself wrote into ORGANIZER finds the event and says
  «organiser» — the only live proof of that branch there is.
- `davtest.Server.HangUpOnce` makes a stand take a request and then lose
  the connection before the answer, so the fault above has a sentinel that
  goes red without it.
- A live assertion had outlived its screen for the third time: the week of
  an agenda is seven days with the empty ones shown, and the move run
  still wanted one. It cost two red tests on two servers.

### Notes

- **SV-38**: Yandex keeps no `ATTENDEE` on an event written over CalDAV and
  puts its own `ORGANIZER` — the mailbox — in its place.
- **SV-39**: the dropped-connection fault above, caught three times in one
  run on one Nextcloud.
- Two questions that were waiting for a full run are answered.
  `TestBaikalRecords` and `TestBaikalAttachments` are green and have not
  failed since 17.09.2026. `TestSoGoSearch` is green too, and the timeout
  it was watched for turned up on `TestNextcloudSearch` instead — so it
  was never about SoGo. That one now repeats: three runs, always one
  source of two, on the same shared-hosting Nextcloud that dropped three
  connections and swallowed an MKCALENDAR. A source has ten seconds and
  the collection is one this run made; ten seconds should not be there to
  spend. Read as the state of that machine rather than a fault of
  Carrel's, whose screen does the right thing — it names the source that
  did not answer and offers Retry. Written down with its numbers.
- The pool is one `dav.Client`'s, not the process's: `Guard.httpClient`
  builds a transport per client, while the guard's own comment claimed
  they shared one. The comment is corrected; whether to share the pool is
  written down in `docs/plan/server.md` and not decided in passing.

## v0.2.15 — 2026-09-21

Живые прогоны на Google, iCloud и Mail.ru. Решение владельца от 20.09.2026
закрыто: новые функции проверены на всех живых серверах, какие позволяют
учётные записи.

### Fixed

- **The bookkeeping a server writes into a record is not a loss** (SV-36).
  Mail.ru adds `X-MOZ-LASTACK` — when an alarm was last acknowledged — on
  one write and drops it on the next. Carrel keeps every line it does not
  edit, so it sent the line back, saw it gone, and told the person a
  property had been lost that they had never written. `X-MOZ-LASTACK`,
  `X-MOZ-GENERATION` and `X-MOZ-SNOOZE-TIME` are now server stamps beside
  `DTSTAMP` and `SEQUENCE`. A property of the person's that begins with
  `X-` is still theirs and is still reported.

### Changed

- Contact fields run on Google and iCloud in the one address book each
  account has, without making a collection: `TestGoogleContactFields`,
  `TestICloudContactFields`.
- Mail.ru has a test of its own for the first time —
  `TestMailruEventFields`: the fields of an event, the rule of a series
  and the zone, in the single calendar its link points at (SV-22).
- The timeline runs on SoGo (`TestSoGoTimeline`). A record the server will
  not hold at all is now logged and its expectation dropped, instead of
  failing the run: a run that demanded a note of SoGo would be testing
  SoGo's component set, not Carrel's timeline.

### Notes

- **SV-36**: Mail.ru rewrites almost everything. `EXDATE`,
  `RECURRENCE-ID` and `X-CUSTOM` are not taken even when written past
  Carrel; `CATEGORIES` is not kept; the rule comes back reordered and
  without `WKST`; changing the zone rewrites `DTSTART` and `DTEND`.
  Everything but the reordering is a loss by S-7.1 and is reported.
- **SV-37**: SoGo keeps no notes. The calendar is made with `VJOURNAL` in
  its component set and `MKCALENDAR` succeeds, but a `PUT` of a note
  answers 404 — not 403, not 415.
- The timeline is not run on Yandex, and cannot be: it is built around a
  person's card, and Yandex makes no address book (SV-19).

## v0.2.14 — 2026-09-21

Живые прогоны на SoGo и Yandex.

### Fixed

- **A rule said in another order is the same rule** (SV-35). SoGo keeps
  `FREQ=WEEKLY;COUNT=6;BYDAY=MO;WKST=MO` where Carrel sent
  `FREQ=WEEKLY;BYDAY=MO;WKST=MO;COUNT=6`, and the comparison after a write
  read the value as text: every edit of a repeating event there told the
  person the server had not kept RRULE. The series was whole and the
  message was false. `RRULE` and `EXRULE` are now compared part by part —
  a part missing or changed is still a loss.

### Changed

- The forms, Mark done, the rule of a series and Disconnect run on two
  more live servers: `TestSoGoForms` and `TestYandexForms`, five in all.
  A part a server will not have — Yandex makes no address book and keeps
  no task — says so and steps aside instead of failing: a server's limit
  is not a fault of the screen.

### Notes

- **SV-34**: Yandex rewrites an event. `COUNT` becomes `UNTIL`,
  `INTERVAL=1` appears, `WKST` goes, the time moves to its own zone, and
  `STATUS`, `CATEGORIES` and `X-CUSTOM` are not kept at all. Each is a
  loss by S-7.1 and Carrel reports it; there is no workaround and a
  workaround would be a lie. One consequence is worth knowing: once the
  server has turned `COUNT` into `UNTIL`, setting «Times» without
  clearing «Until» is refused on the field, which is right.
- Two live assertions had outlived their screens — a conflict names
  properties as the record does, and a week is seven days with the empty
  ones shown. Both were changes already made and agreed; only the live
  suite, which compiles under its own tag, had not heard.

## v0.2.13 — 2026-09-21

fixtures-tail.md: потеря на формах и незаконченный опрос календаря.

### Added

- `Server.SetPatience` lifts the rate buckets for a test, beside SetClock
  and SetBuild. A fixture run does in a second what a person does in an
  hour, and the limits of architecture.md are not meant for it; an
  instance keeps them.

### Changed

- Three states: what a server dropped from an event and from a task, and
  the combined calendar while one of its stands is held.

### Notes

- The partial state of the tasks and of the notes cannot be taken on this
  set: their collections live on one stand, so holding it holds
  everything, and the screen then shows a moment rather than a state. The
  state is covered by the contacts and by the calendar, whose collections
  stand on both.

## v0.2.12 — 2026-09-21

fixtures-tail.md: состояния остальных родов.

### Changed

- Four states: a week of an agenda that holds nothing, a stand refusing to
  a list of tasks and to a notebook, and the conflict of a note. The
  conflict now has all four kinds behind it.

### Notes

- The edit of a note answers with the document rather than the page
  around it (C-7), and its save stands in the document's own bar. That is
  right — a note is edited in place — and it sets the note apart from the
  other kinds, which a fixture now shows.

## v0.2.11 — 2026-09-21

fixtures-tail.md: предпросмотр фотографии.

### Changed

- `app-photo-preview/normal.json`. The screen exists only while a photo is
  being cropped, so it is taken right after one is added: the upload
  answers with a redirect and the screen is the page it leads to.

## v0.2.10 — 2026-09-21

fixtures-tail.md: сброс по ссылке и расхождения остальных родов.

### Changed

- Three pairs: `/forgot/{token}` and the conflict of an event and of a
  task. The conflict is one screen for every kind (C-2.1) and answers
  alike on all three: the property named as the record names it, both
  versions, the same four ways out.
- The token of a reset joins the things the comparison holds steady.

### Notes

- `s-conflict-1` is declared for four routes and drawn with the rail of
  the contacts. An event's rail is the calendar's and cannot be anything
  else, so that is one frame serving four screens rather than a fault.

## v0.2.9 — 2026-09-21

fixtures-tail.md: слияние и письмо.

### Added

- **The rail of the merge** (s-dup-2). By C-5 the merge is a page, not a
  sheet, so a rail belongs to it: the collections compared, of the kind
  being merged — the same ground as the screen it was reached from.

### Changed

- Two pairs: `/app/duplicates/merge` and `/app/mail/compose`. The merge is
  taken over an undecided pair, because a linked group is a decision
  already made and there is nothing in it to merge.
- The digest that names a group of duplicates is held steady like an
  issued identifier: it is made from the records in the group, and those
  carry the identifier of a connection.

### Notes

- A letter needs two things, not one: a relay, and letters from people
  allowed. A relay alone is for the instance's own letters — invitations
  and confirmations — and without the second `/app/mail/compose` is 404.

## v0.2.8 — 2026-09-21

fixtures-tail.md: первый запуск и открытая регистрация.

### Changed

- Two pairs on an instance nobody has set up yet: `/setup` and
  `/register`. They need an empty instance, so they are taken outside the
  mockup set.

### Notes

- `/register` does not exist until a relay is configured: without one the
  letter that confirms an address never goes, and a person who registered
  could never sign in (S-26). The route answers 404 rather than offering a
  form that leads nowhere — right, and now written down.

## v0.2.7 — 2026-09-21

fixtures-tail.md: файлы и приглашение.

### Changed

- Four pairs: `/app/files`, `/app/files/published`, `/app/files/picker`
  and the link an invited person opens. The three screens of the files
  stand in their empty state, because the mockup set holds no folder of
  files at all — that is the truth of this set, and the gap is written
  into plan/server.md beside the collections the frames draw and the set
  does not have.
- The secret of an invitation is held steady like an issued identifier: it
  stands inside an address, where the rule for the CSRF does not reach.

## v0.2.6 — 2026-09-21

fixtures-tail.md: пустые сведённые виды и отчёт о потере.

### Changed

- Four states: the combined calendar, task list and notebook with nothing
  ticked, and `app-contact-edit/loss.json` — a save the server accepted
  without keeping three properties. The report comes back on the form
  where the change was made, not on a screen the person has to find (C-9).
- `check.py` keys a known frame fault by the screen, not by one of its
  states. The same short rail in `loss` counted as a new fault, and
  repeating the entry per state would hide a real one among copies.

## v0.2.5 — 2026-09-21

fixtures-tail.md: состояния сведённого вида.

### Changed

- `app-contacts/empty.json` and `app-contacts/unavailable.json`: nothing
  ticked, where the screen explains the emptiness instead of showing an
  empty list; and one stand answering 503, where the poll names the book
  that is missing from the answer.

### Notes

- A 503 is an answer, so the source stands as `error` («ответил
  отказом») and the summary says «refused»; `unavailable` is for a
  server that does not answer at all. The two agree, and C-8 means what
  it says.

## v0.2.4 — 2026-09-21

fixtures-tail.md: состояния списка одной коллекции.

### Changed

- Three states of `/app/contacts/{account}/{col}` beside its normal one
  (C-14.3): `empty` — a book that holds nothing; `readonly` — a book this
  account may only read, where «New contact» is refused with its reason
  and the import is not offered at all; `source-error` — the stand answers
  503 and the screen says the book is unavailable instead of showing an
  empty list.

### Notes

- Nothing was wrong in the three: the states of C-8 answer as they should.
  That is a result too — the first of the pairs to find no hole.

## v0.2.3 — 2026-09-21

fixtures-tail.md: поиск говорит, где искал, и шесть пар.

### Added

- **The rail of the search**: every collection it asks, of every kind, in
  one list. A search that finds nothing has to say what it searched — an
  unticked collection is not asked, and that is the whole reason a record
  in one «is not there».
- **The screen of the password carries the rail of the settings**, with
  the account marked. It stands inside them in the frame and answered
  without one.
- Six pairs: the search, the password, and the sheets of renaming, moving
  and deleting a collection and of publishing a link.

## v0.2.2 — 2026-09-21

fixtures-tail.md: экран дубликатов говорит, где искал.

### Added

- **The rail of the duplicates** (s-dup-1): the collections the comparison
  had to work with, how many records of each this session holds, and which
  of them it has actually read — «Sources 2 / 3». Duplicates are found
  among what is loaded and never by asking a server (S-16), so a screen
  that answers «no duplicates» has to say how little it looked at.
  Collections of the kind being shown, and only ticked ones: an unticked
  collection is never polled, and naming it a source would promise a
  comparison that cannot happen.
- The pair `app-duplicates/normal.json`.

## v0.2.1 — 2026-09-21

fixtures-tail.md: одиннадцать пар — вне оболочки и вокруг записи.

### Changed

- Eleven pairs: `/login`, `/forgot`, `/about`, `/admin/install`, the
  timeline of a person, the new-record form, the import wizard, and the
  sheets of moving, of a quick note, of a new collection and of the
  unlock.
- `check.py` does not compare the rail of a sheet. A sheet is drawn over a
  screen and the rail in its frame is that screen's; there is nothing to
  compare, and saying so is not an exception but the kind of the screen.

### Notes

- Two frames declare one screen and draw another's rail: `s-notes-8`
  carries the address of a new contact over a calendar rail, and
  `s-contacts-7`, the timeline of a person, likewise. Listed for Typecase.
- The screen of the duplicates has no rail at all, and `s-dup-1` draws one
  — the collections being compared. Its pair waits for that; written into
  fixtures-tail.md.

## v0.2.0 — 2026-09-21

fixtures-tail.md: у настроек и администрирования появился рейл, и с ним четырнадцать пар.

### Added

- **The settings and the administration have their rail** (C-6.3, the
  third kind). Fourteen screens answered with nothing in it but the line
  about the build: a person who opened the settings had no way back to
  the application except the browser's own button, and no way from one
  setting to the next. Both rails are now what the frames draw — «Back to
  app» first, then the screens with their counts, the open one marked.
- Fourteen pairs: every screen of `/app/settings` and of `/admin`.

### Changed

- The comparison holds steady what the run itself makes: the stands' port,
  the identifiers of a person and a task, and the stamps of what the run
  did — when this session began, when this account was made, when a line
  went into the journal. A session begun on the day of the frames would be
  an expired session today, so those times cannot be frozen; they are of
  the same kind as an issued identifier and are replaced by the day of the
  frames, which still teaches the shape of a date.
- `check.py` reads the rail's rows by the rows themselves. The rail of the
  settings is drawn in two blocks, and the check saw one row of nine.

## v0.1.11 — 2026-09-20

fixtures-tail.md: шесть пар — карточки задачи и заметки и четыре формы правки.

### Changed

- **The form of a contact asks in the order of the frame** (s-contacts-3):
  who the person is, then where they work and when they were born, then
  how to reach them, and last what is written about them. It used to ask
  for the telephones before the organisation.
- `app-task`, `app-note`, `app-contact-edit`, `app-event-edit`,
  `app-task-edit` and `app-note-edit` are pairs nine to fourteen.
- The mockup harness follows the frames further: «Дневник» stands in the
  catalog with its tick off, and the lists are opened before a record is,
  which is where the counts in its rail come from.

### Notes

- A place is «Where» on a card and «Location» on a form, and both are the
  frames' (s-agenda-2, s-agenda-3): a form names fields, a card speaks.
- Five more discrepancies that are the frames': three frames name the way
  back with three different words, and two forms draw the rail a row
  short. Listed in check.py, written into plan/server.md.

## v0.1.10 — 2026-09-20

fixtures-tail.md: ещё три пары — карточка события, список задач, тетрадь.

### Changed

- **A place is «Where»**, on an event and on a task, as the frames write
  it (s-agenda-2). «Location» is the name of the property; a card is read
  by a person.
- `app-event/normal.json`, `app-tasks/normal.json` and
  `app-notes/normal.json` are the sixth, seventh and eighth pairs.

### Notes

- Four discrepancies where the frames, not the answers, are wrong are
  listed in `check.py` and written into plan/server.md: three collections
  the frames draw that the mockup set does not hold («Праздники РФ»
  and «Расписание лиги» among the calendars and the task lists, a
  notebook «Архив»), and two back rails drawn short.

## v0.1.9 — 2026-09-20

fixtures-tail.md: пятая пара — агенда одного календаря.

### Fixed

- **The mockup set lost its time zones.** The loader cut each calendar
  file at the first `BEGIN:V`, which is the first `VTIMEZONE`, so every
  event arrived with a `TZID` nothing defined and no agenda could place
  it. A week of the set's own calendar came back empty. The zones travel
  with their events now.
- **A fixture is matched to its frame exactly first.**
  `/app/calendar/{account}/{col}` and `/app/contacts/{account}/{col}`
  fold to one kind template, and the agenda was being compared with the
  frame of the contacts.

### Changed

- `docs/spec/fixtures/app-agenda/normal.json` is the fifth pair.

## v0.1.8 — 2026-09-20

fixtures-tail.md: четвёртая пара — карточка контакта.

### Changed

- **The actions of a record stand in the order of the frame**: Edit first,
  then moving and copying, then publishing a secret link, and the deletion
  last and marked. Publishing stood first, and it is not what a person
  came to the card for.
- `docs/spec/fixtures/app-contact/normal.json` is the fourth pair.
- `docs/spec/check.py` keeps a named list of discrepancies where the frame
  is wrong and the fixture right (`FRAME_TODO`). They are not red — frames
  are not fixed here — but they are printed on every run and written into
  plan/server.md as the owner's Typecase tasks. A silent exception is the
  same discrepancy, only invisible.

## v0.1.7 — 2026-09-20

fixtures-tail.md: карточка контакта по кадру `s-contacts-6`.

### Added

- **`ContactPage.Subtitle`** (C-6): the line under the name — where the
  person works and as what. A linked card takes each half from the member
  the person chose (S-16).

### Changed

- **Organisation and title leave the field list of a card** and stand in
  its head, as the frame draws them. The panel beside a list has no such
  head and keeps them as fields, also as the frame draws it.
- **The fields of a card stand in the order of the frame**: the telephones,
  then the addresses, then who the person is, then what is written about
  them.
- **CATEGORIES is a field of the card**, beside the tags of the rail: a
  card read on its own should say what it is filed under.
- **«Phone · mobile», not «Phone · cell».** `TYPE=CELL` is the format's
  word (C-10).

## v0.1.6 — 2026-09-20

fixtures-tail.md: стенды держат день кадров.

### Changed

- `davtest.Server` takes a clock (`Clock`), and the fixture harness freezes
  it on the day of the set. «Changed on the server» is the stand's word,
  and a wall clock there moved what a fixture holds — the third thing in
  an answer that walked on its own, after the screen's «now» and the end
  of a poll.
- The fixture harness compares a pair in one call (`mockupApp.compare`),
  so a new screen is a test of four lines.

## v0.1.5 — 2026-09-20

fixtures-tail.md: третья пара — экран расхождения, и три находки по кадру.

### Fixed

- **The rail of a record screen stays in its account** (C-6.3). A way back
  carried every account's collections, so a person in the middle of a
  conflict was offered another server's books. Every frame that draws
  «Back to list» shows exactly one account rubric — s-contacts-3,
  s-notes-1, s-conflict-1, all of s-io — and now so does the answer.
- **«Read at» comes from the clock**, like every other time a screen shows.
  It was the wall clock of the machine, which no fixture could hold.

### Changed

- **A conflict names the properties as the record does**: `FN`, `TEL`,
  `NOTE`, not «Display name» or «Phone» (s-conflict-1). Two versions are
  compared against the file, and a property Carrel does not model has no
  label to invent.
- **The conflict screen says what happened** before it asks anything:
  «Another client changed this contact while you were editing it. Nothing
  has been overwritten.» Two columns of values do not tell a person that
  their save was refused.
- `docs/spec/fixtures/app-conflict/normal.json` is the set's Анна
  Ковалёва with a real UID, after a real 412.

## v0.1.4 — 2026-09-20

fixtures-tail.md: вторая пара — тот же экран с незаконченным опросом.

### Changed

- `docs/spec/fixtures/app-contacts/partial.json` is the combined address
  book while one book is still being asked (C-8): two sources done, one
  querying, the counts of what nobody finished counting shown as dashes,
  the refresh button disabled with a reason, and the folded row standing
  on the member that did arrive — «Команда» has not answered, so the pair
  shows its copy in «Контакты», without the badge. The stand is held open
  with `davtest.Stall`, so the state is made, not waited for.

## v0.1.3 — 2026-09-20

fixtures-tail.md: полоса опроса говорит словами кадра.

### Changed

- **The bar over a list says «Polled 5 of 6 sources · 14 events»**, as the
  frames say it, running or not. It used to reword itself under the same
  numbers — «Asking 3 sources · 1 answered · 3 records» while it ran and
  «3 of 3 sources answered · 3 records» when it stopped — which reads as
  two different answers; the meter and the Stop button already say whether
  it is still going. A search keeps its own words: it is asked once and
  answers once.
- **Each kind counts its own.** A contact is a card, an event an event, a
  task a task, a note a note. «Record» is the name of a type in the
  server, not a word for the person reading a list.

## v0.1.2 — 2026-09-20

fixtures-tail.md: первая пара «вью-модель равна фикстуре» зелёная, и она
сразу нашла ошибку.

### Fixed

- **A folded row was a race.** Two runs of the same combined list gave a
  different record for the same linked group: `foldLinked` kept whichever
  member arrived first, and the first to arrive is the one whose server
  answered first. The row's address, stamp, colour and photo changed from
  run to run. The row is now the group's **first member** (S-16), with the
  others behind it in the group's order. No screen and no frame changed —
  only which of two records the row is, and now it is a decision instead
  of a race.

### Changed

- `docs/spec/fixtures/app-contacts/normal.json` is written from the frame
  against the mockup set: the set's UIDs and counts, the order segments,
  the combined import and export entries, source stamps in full, no
  `details` and no `list.columns`.
- The comparison is no longer behind `CARREL_FIXTURE_DIFF`:
  `web.TestFixtureCombinedContacts` is a gate, and a screen that drifts
  from its fixture fails the build (C-14.2, the first of them).
- `docs/spec/check.py`, check 8, compares what a screen shows and only
  that: which collections the rail holds, their ticks, and the dash («—»
  in the frame is `null` in the fixture, never zero). It no longer
  compares their order — that order is the DAV server's and Carrel keeps
  it, while the set's comes from `davtest`, which sorts by path — nor the
  frame's illustration numbers.

### Notes

- Three things the frame draws and the view model deliberately does not
  carry, because the client has everything to derive them (C-3.1): the
  count beside a list rubric, the subtitle «3 of 4 books · 2 accounts ·
  312 cards», and the Print button — what to print already stands in
  `Shell.Print`.

## v0.1.1 — 2026-09-20

fixtures-tail.md: меню учётной записи — то, что показывает кадр.

### Added

- **The account menu says who is signed in and where** (`TopBar.Signed`,
  C-6): the login and the instance's name, as the frame's first menu row
  shows them. Several instances of Carrel — home, work, the one on this
  laptop — are alike on every other part of the screen.
- **Administration stands in the menu** for an administrator, pointing at
  `/admin/`. It was in the frame and in no answer.

### Changed

- The menu's first action is **Settings** (`user.settings`), not
  «Account»: it opens the settings rail, whose first page is the account.
  Sign out is `user.logout` and carries the line before it, as the frame
  draws it. The old ids `account.settings` and `logout` are gone.

### Notes

- **Theme** and **Keyboard shortcuts** stand in the same menu in the frame
  and are deliberately not in the view model: the theme is a setting of
  this browser (U-11) and the keyboard sheet is shell chrome with no route
  (U-7). The client draws them among the server's items and remembers the
  choice itself. Written into U beside the zones of the shell.

## v0.1.0 — 2026-09-20

Подъём версии по решению владельца: четыре хвоста закрыты.

The tail plans of connections, forms, the timeline and the views are closed:
`connections-tail.md`, `forms-tail.md`, `timeline-tail.md` and
`views-tail.md` have no open items left. What they brought, in one place:

- **Connections** (`v0.0.14x`): Test, Edit and Disconnect on a connection
  card; editing keeps the ID and re-seals each live link with the new
  credentials; Disconnect asks on a sheet that names what goes with it —
  collections and ticks, published links, backup jobs, davloom devices,
  linked duplicate groups — and the journal records the counts.
- **Forms** (`v0.0.15x`): the field kinds a record really has — lists,
  a birthday with or without a year, an address, a choice, a zone, a repeat
  rule, a relation, an attendee — each with the labels the frames show,
  and "Show N empty fields" instead of a wall of blanks.
- **The timeline** (`S-15.4`): everything tied to one person — events,
  tasks and notes from every connected server, matched by organiser,
  attendee or `RELATED-TO`, with the column that says why it matched. It is
  collected when the timeline is opened, never by opening a card.
- **The views** (`v0.0.158`): the orders, groups, windows and switches of
  the lists; the journal; the published calendar; the combined import.

What is not closed and is not claimed: `fixtures-tail.md`. The comparison
"the view model equals the fixture" exists and runs (C-14.2), and the first
pair is still red — about a hundred fixtures are yet to be written from the
frames. S-30 is not closed, and this version does not say it is.

### Changed

- `VERSION` — `0.1.0`.

## v0.0.162 — 2026-09-20

fixtures-tail.md: один источник времени для того, что экран называет «сейчас».

### Changed

- **The screens take "now" from one clock.** The stamp under a printed
  page, «2 min ago», «just now», the day an agenda opens on and the next
  occurrences of a rule all came from `time.Now()` at the call site. They
  now come from `timeFmt.now`, set once per answer from the server's
  clock, and the end of a poll (`poll.answeredAt`) comes from the same
  clock through `fanout.Options.Now`. What is not shown as "now" keeps
  the wall clock: rate limits, session expiry, invite and link expiry,
  request timing and the journal. A record's own dates (REV, `changed`,
  DTSTAMP) were never in question — they are the record's.
- **The rail names the build** (C-13): `rail.about` is «Carrel 0.0.1 ·
  786949a» when the binary was built with a version and a commit, and the
  name alone when it was not — a version nobody set is not shown as one.

### Notes

- `SetClock` and `SetBuild` on `web.Server` exist for tests: a fixture has
  to be the same tomorrow. An instance reads the wall clock and the build
  it was compiled with.

## v0.0.161 — 2026-09-20

fixtures-tail.md: the first findings of the comparison, closed.

### Added

- **The details panel answers** (C-7.2): the zone `details` had no handler
  at all. A record's address, asked for as a fragment with that target,
  comes back as `Details` — the name, the fields of the card, where it
  came from, and its actions with "Open the …" first. The panel beside a
  list is a row chosen there, not a part of the list's own answer.

### Changed

- The badge of a linked row says «linked ×2», as the frame does
  (`s-contacts-1`); the sources stand beside it, so naming them twice said
  nothing. The chosen order of a list is marked `On`.
- The fixture harness loads the mockup set as the frames show it:
  «Команда» read-only, «Рассылки» in the catalog but unticked, the two
  Артём Белов linked by a decision of S-16; and it normalises the issued
  identifiers into the names the fixtures use (`a-nc-home`, `kontakty`),
  which is what makes those short addresses legitimate.

## v0.0.160 — 2026-09-20

**Contract change (C-6.1).** Owner's decision 20.09.2026.

### Changed

- **An empty list goes out as `[]`, never as `null`.** In Go a list nobody
  appended to is nil and marshals to null, so the same field came out `[]`
  on one screen and `null` on another by accident — and every client would
  have had to check both. The rule lives at the boundary
  (`web.withEmptyLists`), not in each handler, so it holds for screens
  written before it; the answer is copied first, because a view model may
  share a slice with the session's cache. `null` keeps one meaning: a
  pointer that is not there — no range on the bar, no details panel,
  nothing to retry. Two guards: every page type answers no null list, and
  neither does a real list screen.

## v0.0.159 — 2026-09-20

fixtures-tail.md, step 1.

### Included

- **The comparison "the view model equals the fixture"** (C-14.2), the
  criterion that had nothing behind it at all. The mockup set of
  `docs/visual/fixtures` is loaded into two `davtest` servers labelled as
  the frames label them, the collections of its README are made with their
  colours, and the answer of a screen is compared with
  `docs/spec/fixtures` by `cmp.Diff` after the identifiers Carrel issues
  (`a-…`, `t-…`) and the session's CSRF are normalised away.
- The first pair — the combined address book against
  `app-contacts/normal.json` — is red, as the plan asks it to be shown
  once, and runs under `CARREL_FIXTURE_DIFF=1` until the fixtures are
  rewritten against the set. What it found is written down in
  fixtures-tail.md: the fixture invented its UIDs and collection
  addresses, its rail counts are the mockup's illustration, the server
  fills no details panel on a list at all, and empty lists answer `null`
  where the fixture has `[]`.

## v0.0.158 — 2026-09-20

views-tail.md: the orders, the groups and the windows of the lists.
**Contract change (C-6).**

### Added

- **The order of a list is the server's** (C-3.1): contacts A–Z or
  Recently changed, notes Newest · Oldest · Title, tasks By due · By
  priority · Recently changed, in `ListBar.Segments` — in one collection
  and in the combined view alike. A–Z keeps its letter rubrics; Recently
  changed is one list.
- **Tasks are grouped by their due date** (`s-tasks-1`): Overdue · This
  week · Later · No due date. "This week" is the seven days ahead, not the
  calendar week, or on a Sunday evening tomorrow would be "later". The
  other orders are one list.
- **The agenda has its windows** (`s-agenda-1`): Day, Week, Month and a
  range of one's own ("From → Show"); the window on the screen is the
  window asked of the servers. Switching the window keeps where the person
  is — today when today is in it. A range that ends before it starts is
  the week: the address is never half-obeyed.
- **Every day of the window stands**, the empty ones included: a day with
  nothing in it says so instead of being skipped.
- **Tasks due and notes on the line of days**, each behind its own switch
  (`agenda.tasks`, `agenda.notes`); events are always there. They are read
  whole and placed here: `DUE` and a journal's `DTSTART` are not what a
  server's time-range filter answers for.

- **`/setup` takes an address** (`s-auth-4`): the first administrator may
  leave one, optional, so getting back in later is possible at all. It is
  not confirmed here — nothing was sent to it — and until the account page
  sends the letter it recovers nothing (S-10.5).
- **A relation to another collection has a name** (`s-notes-2`,
  `s-agenda-2`): `RELATED-TO` whose target is not in this collection was a
  bare UID; it now shows the target's title when some list this session
  read holds it, saying "in another collection". The link still stays
  inside the collection: a record is opened at its own address (S-11).

- **The journal is read with filters** (`s-admin-7`): All · Security ·
  Failures with their counts, and "Export CSV" — the filter of the screen
  is the filter of the file, oldest first, times in RFC 3339.
- **A published calendar is asked how much of it to show** (`s-publish-2`):
  the days of the link, whether its `.ics` may be downloaded and subscribed
  to, and whether the text of its events goes with the times. A link made
  with days shows those days and no way to step out of them; one made
  without the file answers 404 to `/download` and offers nothing; one made
  without descriptions hands over an `.ics` with none. A publish that
  never saw the form is what it was: the whole calendar, downloadable,
  with its text.
- **Import and Export in the combined view** (`s-contacts-1`):
  `/app/{kind}/import` opens the wizard with its "Into" picker,
  `/app/{kind}/export` a sheet of the collections to hand over. Neither
  guesses the collection (S-14).

### Changed

- C-6: `Action.On` — an open section of the rail, a chosen segment, a
  switch that is on; `NavRow.On` moved into it. `AgendaRow.Kind` — a task
  or a note on the line is visible as one. `Range.Form` — the range of
  one's own. The three fixtures lose the `"on": false` the rail rows had.
- C-5.4: `/app/{kind}/import` and `/app/{kind}/export`; C-5.6:
  `/admin/audit` also hands over a file (`?format=csv`). C-6:
  `PublishSheet.Form` and the new `ExportSheet`.
- `user.SetupFirstAdminWith` beside `SetupFirstAdmin`;
  `store.PublishedLink` and `user.PublishTarget` carry the days of a
  published calendar, `NoDownload` and `HideDescriptions`.

## v0.0.157 — 2026-09-20

Live runs of the timeline and Disconnect; the gates get the time they need.

### Included

- `integration`: `TestBaikalTimeline`, `TestNextcloudTimeline`,
  `TestRadicaleTimeline` — an event with the card's address in `ATTENDEE`
  is found as "attendee", a note with `RELATED-TO` the card as "related to
  this person", an event that only says the name in its text is not found.
  `runForms` ends with `disconnect`: the sheet names what goes, the
  connection leaves, its link dies, connecting again finds the collections
  anew and unticked, and the server keeps everything.
- Green on Baikal and Radicale. Nextcloud answered `dav: request timed out`
  while the fixtures were being written, twice in a row: the instance, not
  the code.

### Changed

- `scripts/gates.sh` and `scripts/linux-gates.sh` run `go test` with
  `-timeout 30m`: `internal/web` alone is 264 tests, each deriving a login
  key with Argon2id as the product does — eleven minutes on the development
  machine, past `go test`'s own ten. The parameters are the product's and
  are not lowered for tests.

## v0.0.156 — 2026-09-19

connections-tail.md, step 2. **Contract change (C-5.7, C-6, U-6).**

### Added

- **Disconnect on the connection card** (`s-profile-1`): a connection could
  be made and never taken out. `/app/settings/connections/disconnect` is the
  sheet of what exactly goes — the brother of the collection's delete sheet
  (`s-profile-9`, owner's decision 19.09.2026), without an export or an
  archive: nothing is deleted on the server, so there is nothing to save
  first. It names the collections and the ticks that leave the rail, the
  published links that stop opening with their passwords, the copy jobs that
  lose a source or their destination, the davloom devices that stop seeing
  it, and the linked groups of duplicates that lose members. Confirmed, the
  connection leaves Carrel whole (`user.RemoveConnection`, `v0.0.140`), the
  session's cache of it is dropped, and `connection.remove` goes in the
  journal with the counts.
- `ConnectionRemoveSheet` (C-6), the route row (C-5.7); the comment of
  `ConnectionCard.Actions` says Disconnect, not Delete.

## v0.0.155 — 2026-09-19

timeline-tail.md. S-15.4.

### Added

- **The person's timeline** (`/app/contacts/{account}/{col}/{uid}/timeline`,
  `s-contacts-7`): it answered 404. A search for one person over the ticked
  collections: events of the window, tasks and notes whole, matched by the
  card's addresses in `ATTENDEE` ("attendee · accepted") and `ORGANIZER`
  ("organiser") and by `RELATED-TO` the card's UID ("related to this
  person"); files by the person's name in their name. Nothing is matched by
  a name in the text. From 12 months back to 3 ahead; "Widen the range"
  moves the start back 3 months at a time; the footer says since when it
  looked. Newest first, by month, the undated last; segments All · Events ·
  Tasks · Notes · Files with counts. The poll bar, Retry of one source,
  Stop and the stream are the search's (`/app/search/progress/{task}`).
- **The card's inset** (`ContactPage.Timeline`) shows the newest rows of
  the timeline this session opened for the person; opening the card gathers
  nothing. `More` — "Open the timeline" — is always there.
- **Write a note, New event, New task** from the timeline: the sheets of the
  kind with the person in them — a note and a task relate to the card's UID
  (`RELATED-TO`), an event invites its first address (`ATTENDEE`). The way
  in from the shell (`/app/{kind}/quick`) and the full new form carry
  `related` and `attendee` from the address.

## v0.0.154 — 2026-09-19

Live runs of forms-tail.md and connections-tail.md.

### Fixed

- **A birthday without a year showed the year 1604 on Baikal and
  Nextcloud.** sabre keeps `--MMDD` as Apple writes it,
  `BDAY;X-APPLE-OMIT-YEAR=1604:1604-04-12`. The card and the form show it
  as `--04-12`; a day without a year is written back in that form, and a
  full date takes `X-APPLE-OMIT-YEAR` off, or the year typed would be
  hidden (found live, `TestAppleBirthdayWithoutAYear`).

### Included

- `integration`: `TestBaikalForms`, `TestNextcloudForms`, `TestRadicaleForms`
  — the new fields of every form, a series' rule with its exceptions, the
  time zone, Mark done, and a connection whose password went wrong: Test,
  Test again, Edit, Test passes.

## v0.0.153 — 2026-09-19

views-tail.md, owner's decision 19.09.2026. **Contract change (C-5.3, C-6,
C-14.3, U-6).**

### Removed

- **`/app/notes/quick` and `QuickNoteSheet`** from C-5.3, C-6 and `view`:
  a sheet of one kind broke the equality of collections (C-2.1). The note
  has the common sheet (`/app/{kind}/quick`, `NewSheet`) since `v0.0.86`.

### Added

- **"Nowhere to write" on the common sheet** (`s-notes-6`): when no
  collection of the kind takes a record, `/app/{kind}/quick` answers a
  `NewSheet` without a target or fields, with Connect account and Cancel —
  it answered 404.

### Changed

- C-14.3: `narrow` is the client's (U-9); the server has no such fixture.

## v0.0.152 — 2026-09-19

forms-tail.md, step 6. **Contract change (C-6).**

### Changed

- **`NoteDoc.Fields []FormRow`** (C-6, U: the note's edit screen): the
  fields of the note's edit that are not the text — Date and Time (the
  journal's `DTSTART`), Tags, Related to. `NoteDoc` carried the title and
  the body only, and the frame `s-notes-4` has more beside the text. Empty
  when reading. They go with the same Save as the title and the body.

### Added

- **A note's date, time, tags and relation are edited** (`s-notes-4`). A
  date without a time is a day (`VALUE=DATE`); there is no all-day box.
  Tags are one comma field, `CATEGORIES`. Related to is picked from the
  records the session has loaded, or typed as a UID. A refused value comes
  back on its field.

## v0.0.151 — 2026-09-19

connections-tail.md, step 6.

### Added

- **A connection has a label** (`s-profile-2`, C-4: the account is a label,
  not an identifier). The add and the edit form have a Label field, first
  as on the frame; one line, at most 60 characters. The label names the
  connection card and every "account · collection" stamp — lists, rails,
  search, backups, devices, the attach sheet, move and restore; left empty,
  the host, as before. It lives in the sealed blob with the connection; a
  blob written before it has none and reads as the host.

## v0.0.150 — 2026-09-19

forms-tail.md, step 9.

### Added

- **"Show N empty fields" on the contact form** (`s-contacts-3`): the
  optional fields a card leaves empty — nickname, organisation, title,
  birthday, categories, note — come in a field set of their own whose
  legend says how many; the client folds it. A filled field stays in the
  main set, a field with an error is never hidden, and a new contact shows
  every field. No contract change: it is a `FieldSet` with a `Legend`.

## v0.0.149 — 2026-09-19

forms-tail.md, step 8.

### Added

- **Send on the contact card** (`s-contacts-6`) when people may write
  letters on the instance: the card goes into the letter through the attach
  sheet's own POST, and the letter opens (S-26.2).
- **New task on the event card** (`s-agenda-2`) when the calendar holds
  tasks: the quick sheet of a task in the same calendar, with `RELATED-TO`
  the event carried as a hidden row; "Open the card" carries it to the full
  form, where Related to shows it.
- **Export Markdown on the note** (`s-notes-2`): `GET …/export?uid=…` of the
  notebook hands over that one note in the notebook export's format (S-19),
  named after its title. An unknown UID is 404.

## v0.0.148 — 2026-09-19

forms-tail.md, step 7.

### Added

- **Mark done** beside Edit on the task card (`s-tasks-2`) and in the menu
  of every task row (`s-tasks-1`): Mark not done when the task is done. No
  address of its own: a POST to the record's edit with `op=done`, `done`
  and the version; the rest of the record is taken as the server holds it,
  and only `STATUS`, `COMPLETED` (and nothing else) change, as with the
  Done box. A version that moved is the conflict screen. From a row it
  comes back to the list the row was on, single or combined.

### Fixed

- **A POST to a record's edit that carried only some fields wiped the
  rest.** The edit reads the whole form; a body with `op` in it — an
  action's query posted as its body — was taken for a form whose other
  fields were emptied. Such a body is now Mark done when `op=done` on a
  task, and 404 for any other `op`; `op` in the address (attachments) is
  unchanged.

## v0.0.147 — 2026-09-19

forms-tail.md, step 5.

### Added

- **The task form has Starts, Status, Priority, Location, Categories and
  Related to** (`s-tasks-3`). Status is a choice of RFC 5545's four; set to
  Completed it adds `COMPLETED`, set away from it takes `COMPLETED` off, as
  the Done box does, and when the Done box changed in the same save the box
  decides. Priority is High (1), Medium (5), Low (9) or none, and another
  client's number stays a choice. Related to is picked from the records this
  session has loaded for the collection — nothing is asked of the servers —
  or typed as a UID, which wins; it writes the first `RELATED-TO` and keeps
  the others and the parameters of the line.

## v0.0.146 — 2026-09-19

forms-tail.md, step 4.

### Added

- **Repeats on the event form** (`s-agenda-3`, S-11): Frequency, Every,
  On (the seven days), Until, Times and Advanced RRULE, in a field set of
  their own, for a new event too. Only the parts that changed are written
  into the rule as it was, in their places, so what the fields do not show
  (WKST, BYMONTHDAY, a numbered BYDAY) stays. A changed Advanced RRULE is
  written as typed and wins. Until of a timed series is the end of that day
  in the instance zone, in UTC (RFC 5545 §3.3.10). Until and Times together,
  a count or interval below 1 and a rule that does not parse are refused.
  "Does not repeat" takes RRULE away. EXDATE, RDATE and moved instances are
  never touched: the series is edited as a whole.
- `RecurrenceBlock.Fields` of the edit form describe the rule (Frequency,
  Every, On, Until or Times) before the next occurrences.

## v0.0.145 — 2026-09-19

forms-tail.md, step 3.

### Added

- **The event form has Status, Categories and Time zone** (`s-agenda-3`).
  Status is a choice of no status, Tentative, Confirmed, Cancelled, and a
  value another client wrote stays a choice; anything else is refused.
  Categories are one comma field, each item written as a TEXT item of one
  `CATEGORIES` line.
- **Time zone** shows the zone the event came in, marked "as sent", then
  UTC, the instance zone and a list of common zones. Saves that do not
  change it keep `DTSTART` and `DTEND` as they were. Another zone rewrites
  both at the same instants in that zone and adds its `VTIMEZONE` when the
  document has none (built from the server's zone database for the event's
  year, with the yearly rule of its changes); UTC writes `Z` and no `TZID`.
  The form's own times stay in the instance zone (C-10). An all-day event
  has no zone and no such field.

### Changed

- `cal.EnsureTimezone`, `cal.Timezone`.

## v0.0.144 — 2026-09-19

forms-tail.md, step 2.

### Added

- **A phone or an email row has its label** (`TYPE`, `s-contacts-3`): a
  choice of home, work, cell, voice, fax, other or none, and the label the
  row already has when it is none of these — a label another client wrote
  is kept by leaving it. Changing a label writes that row's `TYPE` and
  nothing else; the conflict screen shows a label changed alone
  ("+44 20 1 · cell") and writes it.
- **Addresses are edited**: each `ADR` is five inputs — street, city,
  region, postcode, country — in a field set of their own, and an empty
  one to add. The PO box and extended parts no input offers stay as they
  were; a semicolon typed into a part is dropped. An address emptied of
  every part goes. The row keeps its parameters.

### Fixed

- **The conflict screen offered a name nobody typed.** When the display
  name was changed elsewhere and the person changed something else, the
  display name and N filled each other in against the server's version:
  the draft carried an N, the screen listed it, and "Use only my edit"
  wrote it. The draft now measures the names against the version the form
  was read at.

## v0.0.143 — 2026-09-19

forms-tail.md, step 1.

### Added

- **The contact form has Nickname, Birthday and Categories**
  (`s-contacts-3`). Categories are one field, commas between them, written
  as one `CATEGORIES` line. Birthday takes YYYY-MM-DD, or --MM-DD for a
  day without a year, and is written in the form the stored one has
  (basic `18151210`, extended `1815-12-10`); without one, vCard 3 gets
  extended and vCard 4 basic. A value that is not a date is refused and
  nothing is written. Changing one field writes that line, its parameters
  kept (S-7).

## v0.0.142 — 2026-09-19

connections-tail.md, steps 4 and 5.

### Added

- **Test and Test again on a saved connection** (C-6 `ConnectionCard`):
  `POST /app/settings/connections?op=test&account=…` runs discovery with the
  stored credentials and writes nothing. The trace stays on the card for the
  session (`ConnectionCard.Trace`, C-9.1), as does the trace of Look for
  changes; a card whose last check failed is `unavailable` and its button
  says Test again.
- **Edit on a connection card**: `GET …?op=edit&account=…` gives the add form
  filled with the address, login and fingerprint, the password empty ("Leave
  it empty to keep the stored one"), Save and Cancel. Save runs discovery
  with the new values and writes only when it answers; the connection keeps
  its ID, ticks and links, and what the server holds now is compared with
  the catalog and offered as Look for changes offers it (S-6.1). A failed
  save changes nothing and gives the form back with the trace. What the
  session has cached of the connection is dropped.

### Changed

- `session.Cache.InvalidateConnection`, `Session.NoteCheck`, `CheckOf`.

## v0.0.141 — 2026-09-19

connections-tail.md, step 3.

### Added

- **`user.UpdateConnection`**: a connection takes a new address, login,
  password or accepted fingerprint and keeps its ID (C-4.1), with its
  catalog and ticks; an empty password keeps the stored one. Every
  published link of the connection has its envelope sealed again with the
  new credentials in the same edit — a link made before a password change
  opens after it. The journal line `connection.edit` names the connection
  and the number of links. No screen reaches it yet.

## v0.0.140 — 2026-09-19

connections-tail.md, step 1.

### Added

- **`user.RemoveConnection`**: a connection leaves Carrel whole in one edit
  of the blob and the state — its catalog, ticks, last-used places, linked
  duplicates (a group left with one member dissolves, a preference naming a
  member gone is asked again), device exclusion, attachment and upload
  folders, the sources of copy jobs, its published links with their
  envelopes, and the collections davloom devices saw of it. Nothing on the
  server is touched. A copy job is never deleted: one whose destination was
  the connection says the destination is no longer in the catalog. The
  journal line `connection.remove` carries the counts, not the address.
  No screen reaches it yet.

## v0.0.135 — 2026-09-19

Fable audit №3 (Ф3-4).

### Security

- **`/setup` is rate limited like sign-in**: five attempts a minute for one
  login, twenty for one address (S-29.3: every route without a session has a
  bucket). Its POST derives a key, and a fresh instance could be kept busy
  by anybody who found it before its administrator did (Ф3-4).

## v0.0.136 — 2026-09-19

Fable audit №3 (Ф3-5).

### Security

- **davloom reads `CARREL_TRUSTED_PROXIES` the way the web does.** It split
  the list on commas only, so a list separated by spaces or semicolons —
  which the web accepts — trusted nobody: behind such a proxy every device
  shared the proxy's address, and twenty wrong passwords from one stranger
  locked every device out for a minute (Ф2-11 over again, Ф3-5). One parser
  serves both listeners.

### Changed

- `conf.ParseTrustedProxies`, `conf.TrustedAddr`.

## v0.0.137 — 2026-09-19

Fable audit №3 (Ф3-6).

### Security

- **A wipe revokes everything that read the connections without the
  person's password**: the published links (their envelopes hold the
  connections' own credentials under `server.key`), the external backup
  triggers (theirs hold the old data key) and the davloom devices. Until
  now the links and the triggers outlived the wipe: a link made by whoever
  held the account went on reading after the owner wiped it to lock them
  out, and the state kept the trigger's secret with an envelope of the old
  key (Ф3-6).

### Changed

- `store.WipeAccount(st, u)` is the wipe on both paths (`/forgot/{token}`
  and a repeated self-registration); `store.WipeCredentials` stays for the
  credentials alone.

## v0.0.139 — 2026-09-19

Fable audit №3 (Ф3-8).

### Security

- **davloom forwards only the methods it advertises** (the `Allow` of
  OPTIONS): TRACE, CONNECT or anything a client invents is answered 405 on
  the listener and never reaches the collection's server with the person's
  credentials. It is a DAV proxy, not a general one (S-27.3, Ф3-8).

### Included

- A guard that a published calendar names no server: the agenda rows of
  the app carry "host · collection" and the sources of a linked group, the
  public page must not. It held; the test stays.

## v0.0.138 — 2026-09-19

Fable audit №3 (Ф3-7).

### For consumers

- **Sign-ins are in the journal** (S-10.6, frame s-admin-7): `login.ok` for
  a successful one, `login.fail` for a wrong password on an account that
  exists. An unknown login writes nothing — the journal keeps no typed text
  — and neither does a disabled account, whose answer to the client is the
  same. No address is recorded: the journal is kept for good, and the
  sessions list shows where a live session came from. This decides the
  note the pre-S-30 review left in views-tail.md (Ф3-7).

## v0.0.134 — 2026-09-19

Fable audit №3 (Ф3-2, Ф3-3).

### Security

- **An archive is restored under the ceilings of an import** (S-29.4,
  S-24.3): a member is not read into memory past `CARREL_IMPORT_MAX_BYTES`,
  and a collection of more than `CARREL_IMPORT_MAX_RECORDS` is refused
  whole, on the preview and on the run — as the import wizard refuses such
  a file. `backup.OpenAt` refuses a manifest past 1 MiB and an archive of
  more than 50 000 members. Until now every member was read with
  `io.ReadAll`: a small zip that inflates to gigabytes took the process down
  on the preview, on every view of the Backup screen while it stayed open
  (Ф3-2).
- **Nothing is written by a path taken from the archive** (S-29.4): the
  collection's name in the manifest became the folder restored files went
  under, so a manifest saying `../../x` wrote outside the chosen storage.
  A name that is not one segment is now made one (`backup.Slug`) (Ф3-3).

### Changed

- `backup.MaxManifestBytes`, `backup.MaxEntries`, `backup.Slug`.

## v0.0.133 — 2026-09-19

Fable audit №3 (S-29 final, Ф3-1 in [reviews.md](../../docs/plan/reviews.md)).

### Security

- **Every answer of the web listener now carries the security headers**
  S-29.1 and C-11 promise: `Content-Security-Policy` without `unsafe-inline`
  (`default-src 'self'`, `img-src 'self' data:` for the QR code,
  `object-src 'none'`, `frame-ancestors 'none'`, `base-uri 'self'`,
  `form-action 'self'`), `X-Content-Type-Options: nosniff`,
  `X-Frame-Options: DENY`, `Referrer-Policy: same-origin` and
  `Cache-Control: no-store`. Until now no page had any of them: the JSON
  renderer set only `Content-Type`, and a client rendering on this origin
  had the policy in the spec, not in the headers (Ф3-1). A download keeps
  its own sandbox and caching, a photo its ETag, a path with a secret its
  `no-referrer`; the mux's own 404 carries the defaults too.
- davloom answers with `X-Content-Type-Options: nosniff` on every path.

## v0.0.132 — 2026-09-19

### Fixed

- **"Overwrite this one" on a folder no longer deletes the folder before its
  replacement exists.** The old folder was deleted first and then the new one
  was copied in. A copy that failed half way (the server full, the connection
  dropped) left neither. The copy is now written beside the old folder under
  a name of its own. The old folder goes only when the copy is whole, and the
  copy then takes its name. A failed copy is removed and leaves the
  destination as it was.

## v0.0.131 — 2026-09-19

### Fixed

- **An address book or a calendar bigger than one answer now opens.** The
  whole book used to come in one REPORT, and past
  `CARREL_DAV_MAX_RESPONSE_BYTES` (10 MiB) Carrel said the collection was
  unavailable. Three hundred contacts with a photo each already reach that.
  Such a collection is now read in parts, under the same ceiling:
  - a book: its listing, then `addressbook-multiget` of up to 100 cards, and a
    batch whose answer is too large is split in halves;
  - a calendar read whole (tasks, notes, export, backup): its listing, then
    `calendar-multiget` the same way.
  
  A single record larger than the ceiling is left out: a card is logged by
  path, a calendar object is named as unreadable.
- A book read by multiget, whether too large for one answer or on a server
  without `addressbook-query` (Yandex, SV-23), is now asked for in vCard 4.0,
  as the query is. Before, a converting server answered in 3.0, and an edit
  wrote 3.0 back (SV-1).

### Changed

- `dav.CalFetcher.FetchCalendar`; `dav.cardReports.MultiGetCards` asks in
  batches.

## v0.0.130 — 2026-09-19

Nothing in the running server changed.

### For operators

- `CARREL_DAVLOOM_PORT` is listed in C-12 with the rest of the variables, so
  the README points to one list instead of two.

## v0.0.129 — 2026-09-19

### For consumers

- **Every form that posts now carries its button** in `Form.Actions`, with the
  words from the frames. Until now a client had to invent the label.
  - Sign in, and Create an account while registration is open.
  - Create administrator, Create account (register and invite), Continue
    (forgot).
  - Erase and set a new password, then Set the password and sign in (the two
    steps of the wipe link).
  - Change password, and Show on the unlock sheet.
  - Connect and Test only on the add-connection form.
  - Save (open registration), Create link (invitation), Save (backup job).
  - Save, Change, Recover and Turn on key escrow on the admin escrow screen.
  - The two restore buttons now belong to the restore form (`Source.Actions`),
    not to the whole block.
- **"Create an account" on the sign-in page is a button of the form.** It
  used to be a text field named `register` with no label and the link hung
  on it.

### Fixed

- A failed sign-in linked "Forgotten your password?" and "About" to the root
  of the host. An instance under `CARREL_BASE_PATH` sent people nowhere.
- `GET /app/settings/unlock` without the fragment header now answers with the
  sheet as a page, as the other sheets do. It answered 404, so "Show password"
  led nowhere without scripts.

## v0.0.128 — 2026-09-19

### Fixed

- **What the session has read stays read after a minute.** The index of
  records read by a list or a combined view lived only as long as
  `CARREL_CACHE_COLLECTION_TTL` (60 s). After that the duplicates screen
  said "Nothing is loaded yet", the numbers in the rail turned into dashes,
  the search suggestions lost their records, and links between records fell
  back to bare UIDs. The TTL now says only when a list is worth asking the
  server again. What was read goes only when the cache budgets of S-4.2 need
  the room, or when the session ends (`session.Cache.PeekETagMap`). Folder
  listings still use the TTL: there it decides whether a folder is read
  again.

## v0.0.127 — 2026-09-19

### Fixed

- **A file upload no longer fails at random with "The server could not be
  reached".** A streamed body — a file, an attachment in the folder, a copy
  between servers, a davloom write — cannot be sent a second time. When it
  went out on a pooled connection that the server had just closed (Apache
  and SFTPGo close idle connections within seconds), the upload was lost.
  Such a request now always opens a connection of its own
  (docs/servers.md, SV-14). A body that can be sent again still uses the
  pool and its one retry.

## v0.0.126 — 2026-09-19

### For consumers

- **New input kind `hidden` (C-8, `view.InputHidden`).** A value the form
  carries and nobody fills in now comes as a hidden row with no label: `_csrf`,
  the version a record form was read at (`etag`, `readat`), and the step a
  form stands for (`op`, `action`, `id`, and the collection of the calendar
  range in the mail attachment sheet). Before, these came as text rows with no
  label, and a client had no way to tell them from fields. Render them as
  `<input type="hidden">`.
- The record edit form no longer shows "Version" and "Read at". They were
  read-only text rows holding an ETag and a Unix time.

## v0.0.125 — 2026-09-19

### Fixed

- **A source that fails badly fails alone.** A panic while one source of a
  search or combined view is asked now settles that source as an error, with
  the reason. The other sources answer as before. It used to end the process
  and every session in it. Background thumbnails and backup runs are guarded
  the same way: a thumbnail is lost, or the run is recorded as failed.

### Plans

- `docs/plan/providers-tail.md` is removed: its last open item, parameters
  with several values, was closed in `v0.0.124`. The review before S-30 keeps
  its findings in `docs/plan/review-server.md`.

## v0.0.124 — 2026-09-19

An edit changes what was edited, and nothing else. A broken record is one
record, not a broken calendar or a stopped server.

### Fixed

- **Editing a contact no longer rewrites the rest of the card.** Every save
  used to write the whole card afresh through go-vcard. Cards from Apple
  Contacts lost the link between a phone or address and its label
  (`item1.EMAIL` / `item1.X-ABLabel`), all but the first `TYPE` value (`HOME`,
  `pref`, `VOICE`), and a quoted parameter was cut at its comma. Categories
  `work,friends` became one category, `work\,friends`. Other clients saw all
  this; Carrel itself did not, because it read the card back the same way.
  Now any value nobody edited goes back in the line it arrived in, byte for
  byte. An edited value is written with all of its parameter values.
  (docs/servers.md, SV-32)
- **Editing an event keeps every value of a parameter**: the second address
  of `DELEGATED-TO` and `MEMBER`, and the values of unknown parameters.
- **The loss report no longer reports reordered or recased `TYPE` values**
  as lost. A value that is really missing is still reported.
- **One malformed calendar record no longer hides the whole calendar**, and
  can no longer stop the process. go-ical panics on some malformed lines
  (a parameter with no colon after it, text after a closing quote). The
  record could come from a far server, a subscription or an imported file,
  and in a combined view or a search the panic ended the server for every
  user. The record is now named as unreadable, and the rest of the calendar
  shows as before (docs/servers.md, SV-33).
- A contact's type label reads `Email · internet, home, pref` rather than
  showing only the first type.

### Changed

- `model.RawValue` carries `Group` and `Wire`, the line the value came in.
  A patch that changes the value clears its `Wire`. `SetValues` keeps
  whatever it does not change: a value whose text stays, in order.
- `dav.CardObject`: a card as the REPORT returned it, with its text.
  `CardQuerier` and `CardFetcher` return it.
- `calReports.QueryCalendar` is Carrel's own `calendar-query` (`dav/calquery.go`).

## v0.0.123 — 2026-09-19

### For operators

- **README**: what Carrel is, how to run it, the variables that matter on
  day one, a reverse-proxy setup for nginx and Apache (davloom under `/dav/`
  included), the checks of the repository, and what a person still checks
  by hand (S-30.4).

## v0.0.122 — 2026-09-19

The acceptance checks of S-30 that a machine can run.

### For operators

- A request for a path that leaves its collection (`..` or a separator
  inside a segment) is logged as a warning, with the path only. It used to
  be refused silently.

### Changed

- New tests check, for the process as a whole, that:
  - leaving the page during a poll leaves nothing running, and Stop closes
    the far connection at once;
  - one connection of three down breaks no screen;
  - the shutdown signal lets a request under way finish;
  - the debug log of a whole flow holds no password, token or record
    content.
- The race detector runs over sessions, fanout, the proxy, accounts and the
  web layer.

## v0.0.121 — 2026-09-19

A live check behind a real nginx, and what it taught about the proxy's
settings.

### For operators

- **davloom behind nginx**: forward `/dav/` without a path after the
  address, `proxy_pass http://127.0.0.1:8081;`. With one
  (`…:8081/`), nginx decodes the path first; the `%2F` in a collection's
  address turns into `/`, and requests miss the collection. davloom strips
  the `/dav` prefix itself. With Apache, set `AllowEncodedSlashes NoDecode`
  and `ProxyPass … nocanon`.
- `scripts/proxy-check/` runs Carrel, nginx and Radicale in Docker on a
  Linux host and checks both the installation check and davloom through
  nginx: once with nginx left at its defaults, once as S-28.1 asks.

## v0.0.120 — 2026-09-19

The installation check: Carrel calls itself at its public address and says
what a browser or a sync client will meet.

### For operators

- **Admin → Installation** takes the public address and saves it to
  `settings.env` as `CARREL_PUBLIC_URL`. A variable in the process
  environment still wins at the next start, and the page says so.
- **Run the check** reports eleven rows, each with what happened and what to
  do: reachable from outside, certificate, base address, scheme behind the
  proxy, client address, event stream buffering, upload size, long request
  timeout (not measurable from outside, shown grey), cookie, clock, and data
  folder.
- The check changes nothing but the address. Each run goes to the audit log.

## v0.0.119 — 2026-09-19

### Fixed

- **davloom and Nextcloud**: a device's REPORT (calendar query, multiget)
  reached Nextcloud chunked and got a 500. The rewritten request body now
  goes with its length, up to 10 MiB. Checked live through Baikal and
  Nextcloud.

## v0.0.118 — 2026-09-19

davloom works: a device connects to one address and sees the collections you
gave it, from every server.

### For operators

- With `CARREL_DAVLOOM_PORT` set, a second listener serves the DAV proxy on
  that port, bound to `CARREL_BIND`. Put it behind your reverse proxy at
  `/dav/` of `CARREL_PUBLIC_URL`, the proxy stripping `/dav/`.
- The proxy has its own rate buckets and logs with `listener=davloom`. It
  sets no cookies.
- SIGINT and SIGTERM stop both listeners, give running requests up to
  `CARREL_PROGRESS_TOTAL_TIMEOUT`, then wipe every session and device key.

### For consumers

- **A CalDAV, CardDAV or WebDAV client** signs in with your Carrel login and
  the device's password. It sees one principal with calendars, address books
  and folders — exactly the collections the device may see.
- Collection addresses stay the same across reconnections and restarts.
- A server that does not answer leaves its collections listed; the others
  work.

## v0.0.117 — 2026-09-19

davloom devices: a password per device, shown once, with what it may see.
The proxy that uses them comes in the next versions.

### For consumers

- **Settings → DAV proxy** issues a device password after you confirm your
  own. It is shown once. It unlocks your credentials for that device only,
  and changing your Carrel password does not disconnect it; revoking it
  does.
- What a device sees — kinds and collections — is chosen when it is made,
  and changed later without a new password.
- **Deleting a collection** now names what points at it: published links,
  backup jobs, and devices that see it.

## v0.0.116 — 2026-09-19

A Nextcloud or ownCloud share link can be subscribed to.

### For consumers

- **Subscribe to a folder** takes the link a Nextcloud or ownCloud user
  sends, `https://host/s/…`. Carrel reads it as that share's WebDAV. If the
  link has a password, it goes in the link's password field. The folder is
  named "Shared folder · host" unless you name it.

### Fixed

- Subscribing to a Nextcloud share link used to fail: the link is a page
  for people, not WebDAV.

## v0.0.115 — 2026-09-19

Test beside the connection form, and the administrator's DAV validator, show
what each step of discovery asked and what the server answered.

### For consumers

- **Test** on the connection form runs discovery and saves nothing. The
  trace names the step that failed: .well-known, principal, home set,
  collections.

### For operators

- **Admin → DAV validator** runs the same discovery for any server. Its
  second stage creates a test event, task, contact and file, reads them back
  and deletes them. It runs only with the box ticked and is written to the
  audit log. An object the server would not delete is named by its path.

## v0.0.114 — 2026-09-19

"Upload file" in the create menu asks where the files go.

### For consumers

- **Upload file** opens a sheet with the tree of folders: the attachment
  folder first, and after that the folder you chose there last. "Upload here"
  sends the files to it the usual way; read-only storage is shown and closed.

## v0.0.113 — 2026-09-19

The account page lists your sessions and can delete the account.

### For consumers

- **Sessions**: every live session with its browser, address, start and last
  activity; End one, or End every other session.
- **Delete account** asks for your password first, on the same sheet that
  reveals saved passwords; then it deletes the account and its saved
  connections. Data on the DAV servers stays. The last administrator cannot
  delete their own account.

## v0.0.112 — 2026-09-19

Published links can be copied and mailed at any time; a calendar goes into a
letter for the days you choose.

### For consumers

- **Published links** shows each link made from now on in full, with Copy
  and, when people's mail is on, "Send this link". The link is kept in your
  encrypted account data, under your password like your connections; the
  instance itself still keeps only its hash. Links made before this version
  cannot be shown: nobody has them.
- **Calendar for a range** in a letter has From and To fields; both days are
  in the file. It starts at 30 days from today.

### Changed

- `MailAttachSheet` has `Range`, a form with the two days (C-6).

## v0.0.111 — 2026-09-19

The administrator sets the instance's name, the lifetimes of sessions and
invitations, and people's mail on Admin → Settings.

### For operators

- **Instance name**: on the sign-in screen and in the letters the instance
  sends. Empty means "Carrel".
- **Lifetimes**: idle timeout (1–168 h, applies to open sessions at once),
  session life (1–365 days, for sessions started afterwards), invitation
  lifetime (1–720 h).
- **Letters from users** and their four ceilings are set on the same page.
  `CARREL_MAIL_*` are the defaults until the administrator sets a value.

### Changed

- Default session life is 30 days (was 7) and default invitation lifetime is
  72 hours (was 7 days), as on the settings screen design.

## v0.0.110 — 2026-09-19

People send letters: with records and files attached, or with links to
them, from the instance's domain. Off unless the operator turns it on.

### For operators

- With `CARREL_MAIL_ENABLED=true` and a relay configured, "Letter" appears
  last in the create menu. The ceilings are `CARREL_MAIL_MAX_*`; letters a
  day are counted per person per UTC day.
- The journal records each letter as `mail.send`: to whom, and the kinds
  attached. Never the subject or the text.

### For consumers

- **Account** now has a display name — the sender's name and the signature —
  and an address with confirmation. A new address waits until its link is
  opened; the one before stays in use until then.
- **Letter**: recipients are suggested from the ticked address books, one row
  per address; books that did not answer are named. Attach a contact, a
  book, an event, a calendar for a period, a task, a note (Markdown or the
  source .ics), a file, or a folder link.
- Whether a thing goes as a file or as a link is shown when you attach it: a
  folder always goes as a link, and so does anything over the ceilings.
  Links are made when the letter goes, last 30 days, and are listed in
  Published links.
- A link you have just published can be sent by letter from its sheet.
- Replies go to your confirmed address. Without one, the letter says at its
  foot that it cannot be answered.

### Changed

- An invitation confirms the invitee's address only when it was mailed to
  that address. A link handed over no longer confirms the address typed
  beside it: that address waits on the account page for its confirmation
  link, and sign-in is not held up by it.

## v0.0.109 — 2026-09-18

The administrator chooses the sender of people's letters.

### For operators

- **Sender address** on Admin → Settings → Mail: people's letters go either
  from the instance's address ("Ann (via Carrel) <noreply@…>") or from
  their login on the instance's domain ("Ann <ann@…>"). The envelope sender
  stays the instance's address either way, so SPF is unaffected; the relay
  decides which of the two it lets out. The instance's own letters always go
  from its address.

### For consumers

- Every letter a person sends carries Reply-To with their address when it
  is confirmed, whichever sender form the instance uses.

## v0.0.108 — 2026-09-18

The instance sends mail: invitations, address confirmations, reset links and
recovery notices go by letter through a relay the administrator sets.

### For operators

- **Admin → Settings → Mail** takes the relay: host, port, STARTTLS / TLS /
  none, login, password and the instance's From address. The password is
  never shown back; leave the field empty to keep it. "Send test" shows the
  relay's whole answer. An empty host turns mail off.
- STARTTLS, when chosen, is required: a relay that does not offer it is an
  error, never a fall back to the clear.
- Links in letters start with `CARREL_PUBLIC_URL`; set it when Carrel sits
  behind a proxy, or links take the address each request came to.
- Letters from people stay off (`CARREL_MAIL_ENABLED`); the page shows that
  and the limits, read-only.

### For consumers

- **Invitations** with an address are mailed; the list says "sent",
  "failed" or "SMTP not configured". The link is shown either way.
- **Forgotten password** mails the reset link to the confirmed address, and
  nowhere else. An administrator's reset does too; if the letter fails, the
  administrator gets the link to hand over.
- **Key-escrow recovery** mails the person a notice, always. The temporary
  password is not in it: it goes hand to hand.
- Every letter goes from the instance's address, with no Reply-To.

### Changed

- Open registration now needs a configured relay (it used to need
  `CARREL_MAIL_ENABLED`): without a confirmation letter nobody who registers
  could sign in.

## v0.0.107 — 2026-09-18

What a subscription is, and what a connection is, is written down, and the
screens follow it.

### For consumers

- **A subscription is a collection reached by a link** — of any kind, read
  only, whoever it belongs to. A link may have a password of its own; the
  subscribe form now names its fields "Login of the link" and "Password of
  the link".
- **A connection is an account**: an address with its login and password,
  and everything written to goes through one.
- **"Connect account" without a login no longer makes a connection.** The
  page says the address is a link and shows the subscribe form with the same
  address — as a folder, or as a feed for `webcal://`, `.ics` and `.vcf`.

### Changed

- Breaking for anyone who connected a password-less WebDAV server as an
  account: that is now refused and offered as a subscription, read-only.

## v0.0.106 — 2026-09-18

A backup can be restored — into new collections, never over what is there
(S-24.3).

### For consumers

- **Restore** on `/app/settings/backup` (`BackupPage.Restore`): the archive
  comes from a job's history, from a file on a WebDAV storage, or from this
  computer (`op=restore-open`, the file streamed with the form). A wrong
  password or a file that is not a Carrel archive is said, and nothing is
  kept.
- **Preview before anything is written**: every collection of the archive
  with its first records or files and its count, and where it can go
  (`Targets`). Nothing is created until `op=restore-run` names targets as
  `target.<n>=`.
- **Restoring creates**: records go into a new collection "… (restored
  DATE)" on the chosen server, files into a new folder of that name; nothing
  that was there before is touched. `op=restore-cancel` closes the archive.
- The archive waits between steps in a temporary file of the session, never
  in memory; it is deleted when the restore ends, when it is closed, or with
  the session.

## v0.0.105 — 2026-09-18

Somebody's folder can be subscribed to, like a feed (S-25.2, owner's
decision).

### For consumers

- **"Subscribe to a folder"** in the Files rail's foot opens the subscribe
  form in folder mode (`/app/settings/connections?op=feed&kind=folder`): a
  WebDAV address — another Carrel's secret link is read at its `/dav/` —
  an optional login, a name. Checking lists the folder and saves nothing.
- **A subscribed folder** is one read-only file collection under
  "Subscriptions" in the Files rail, read live on every visit. It is never
  written, whatever its server would allow, is not published onwards and is
  not copied by backup jobs; its files can be downloaded and copied to your
  own storage. Unsubscribing leaves the folder as it is.
- Yesterday's note that files are not subscribed to is withdrawn: all five
  kinds can be subscribed to (C-2.1).

## v0.0.104 — 2026-09-18

A backup job can be started from outside (S-24.4).

### For consumers

- **External trigger per job**, turned on in the job's card inside the unlock
  window (`op=trigger`; `op=trigger-off` turns it off). The secret — `cbk_…` —
  belongs to the job, not the account, and is shown once. What the card hands
  over is what to paste: a crontab line for a job that writes to WebDAV, a
  line with `-o` for one that downloads, a short script for Both.
- **`/api/backup/…`** answers a script, not a browser: the secret in
  `Authorization: Bearer`, plain text back, no cookie, no redirect.
  `POST /api/backup/run` → 202 and the run's id; `GET /api/backup/status?id=`
  → one word (`running`, `done`, `partial`, `failed`), `?job=` → the last run
  and the last whole copy; `GET /api/backup/{id}/download` → the archive from
  its destination, 409 while the run runs; `GET /api/backup/download` → a
  fresh archive streamed in the same request. A wrong secret is the same 404
  as any secret that never existed.

### Security

- **Turning the trigger on stores the person's data key for that job**, sealed
  under `server.key` and bound to the job. That is what lets the job read
  without anyone signed in, and the card says so: a stolen volume together
  with `server.key` reads what the job reads. An unencrypted job with a
  trigger is warned about twice. Every use of the secret is journaled;
  turning the trigger off deletes the secret's hash and the envelope.

## v0.0.103 — 2026-09-18

Backup jobs: an archive of chosen collections, written, read back and kept
in rotation (S-24, first part).

### For consumers

- **`/app/settings/backup`** lists the jobs — what, where, how many kept,
  sealed or not, how the last run ended — with their history, and holds the
  editor (`?job=` to edit; `op=save`, `op=delete`, `op=run`).
- **A job** copies collections of every kind — a file collection as files —
  to a folder of a WebDAV file collection (made on first use), to the
  browser (`op=download`, nothing written to a server), or both.
- **The archive** is one zip: a folder per collection and `manifest.json`
  with where each came from, its kind, how many records or files, when and by
  which version, and the error of any collection that did not go in. It is
  sealed with age under the job's own password by default; an unsealed one
  opens in any archiver, and the editor says what that means.
- **A run is verified or it is not**: the archive is read back from where it
  was written and its SHA-256 compared. Copies past the job's number are
  deleted only after that, and only when the new copy is whole — a partial
  copy never pushes out a whole one. A copy that is not whole is red on the
  screen.
- A job can run at sign-in when its last whole copy is older than a number of
  days. Carrel keeps no clock of its own; the external trigger comes next.
- An archive in the history downloads from where it lies, never from
  Carrel's volume.

### Included

- **New dependencies: `filippo.io/age` v1.3.2** (pinned in
  `docs/plan/architecture.md`) and, through it, `filippo.io/hpke` v0.4.0.

## v0.0.102 — 2026-09-18

Somebody else's calendar or address book can be subscribed to (S-25.2).

### For consumers

- **"Subscribe to a feed"** on the connections page and in the rail foot of
  every record section (`/app/settings/connections?op=feed`). A feed is one
  `.ics` or `.vcf` at an address; `webcal://` is HTTPS, and another Carrel's
  secret link is read at its `/download`. `op=feed.check` shows what the
  address answered — status, size, time, type, what is inside, the name —
  and saves nothing; `op=feed.subscribe` reads it once more and keeps the
  subscription. A feed with no records, or one that refuses the password, is
  not subscribed to.
- **Nothing of the feed is kept**: it is read through the guard on every
  visit, under `CARREL_DAV_MAX_RESPONSE_BYTES`. The sealed blob keeps the
  address, the local name and colour, the login and password when the feed
  asks, and the kinds it carries.
- **A feed reads like a read-only collection**, at
  `/app/{kind}/f-…/feed`: lists, records, export, copying a record out, the
  rail's "Subscriptions" section, ticks, the combined views, search (sifted
  by Carrel) and duplicates. Editing, creating, importing, deleting and
  moving are 404, as for a foreign address; a feed is not published onwards.
- **`SubscriptionCard`** on the connections page: Check (reads again and
  keeps the kinds it carries now), Edit (the rename sheet — the name and
  colour stay in Carrel), Unsubscribe (the subscription and its ticks, never
  anybody's data).

### Changed

- **Discovery reads rights for file collections too.** A storage that says it
  can only be read is read-only from the start. A folder published by
  another Carrel now says so: its `/dav/` answers `current-user-privilege-set`
  with `read` only, so connecting it as an ordinary connection — the way a
  folder is followed, since files are not feeds — gives a read-only
  collection. Other WebDAV servers that send no rights are unchanged.

## v0.0.101 — 2026-09-18

A file or a folder can be published, and a published folder mounts as a
read-only drive (S-25.1, second part).

### For consumers

- **"Publish…"** in every file row's menu and in a folder's bar. The sheet is
  the same (`k=files`, `f=` the path, `dir=1` a folder); a path that is not on
  the server publishes nothing.
- **A folder's page** lists it (`PublishedPage.Folder`) and goes down into its
  folders with `?in=`, never up or sideways. `/p/{token}/file/{path…}`
  streams a file in a sandbox; `/p/{token}/download` is the file, or the
  folder — or `?in=` a folder in it — as a zip written while it is read.
- **`/p/{token}/dav/`** mounts the folder as a drive that can only be read:
  OPTIONS, GET, HEAD and PROPFIND answer; PUT, DELETE, MKCOL, MOVE, COPY,
  PROPPATCH, LOCK and POST are 405 and never reach the storage. The listing
  is Carrel's own: no address, name or property of the storage server.

### Changed

- **Rate limit of `/p/`** (owner's decision): only a miss spends the
  address's bucket of 30 a minute, and an empty bucket is 429 before any
  check. A link whose secret matched has a bucket of its own, 120 a minute,
  the size of a session's — a mounted drive would not live on 30.

## v0.0.100 — 2026-09-18

A collection or a record can be published as a secret read-only link
(S-25.1, first part).

### For consumers

- **"Publish…"** on every record page and in every list's bar, read-only
  collections included: publishing only reads. `GET /app/publish?k=&a=&c=`
  (and `uid=` for one record) is the sheet — what is published and until
  when, before anything is made; its actions POST the same address for 30
  days or, explicitly, without an expiry. The answer carries the link once.
- **`/p/{token}` is read live, without a session**: an address book, one week
  of a calendar with its range, a task list, a notebook, or one record. The
  page is read-only and leads nowhere into the owner's Carrel: no app
  addresses, no server, no login. A server that does not answer is a notice,
  not a 500. `/p/{token}/download` hands the collection or the record over as
  a file.
- **Revoked, expired and never-existing links are one 404**, in one time.
  So is the link of a disabled account.
- **`/app/files/published`** lists the live links — what, created, until,
  last opened — with Revoke. The link itself is not shown again: only its
  hash is kept.
- `PublishedPage` gains `Tasks`, `Notes`, `Fields` and `Body`.

### Security

- The state keeps each link's hash, target and expiry, and an envelope with
  the address and credentials of that link's one connection, sealed again
  under `server.key` and bound to the link. A stolen volume together with
  `server.key` opens the passwords of connections that have a live link —
  the price of reading without a session. Revoking a link deletes its
  envelope.

## v0.0.99 — 2026-09-18

The Android tab has a code that opens DAVx5 (S-23).

### For consumers

- **`DevicesPage.QR` is filled on the Android tab**: a `data:image/png`
  image of the DAVx5 link of one connection — `davx5://login@host/path/` for
  HTTPS, `caldav://` for plain HTTP, as the DAVx5 manual gives it. It carries
  the address and the login, **never the password**.
- The same link is an action, "Open in DAVx5 · host", for a page already on
  the phone; "Code for host" switches the code to another connection
  (`?account=`). A connection left out of devices has no code.

### Included

- **New dependency: `rsc.io/qr` v0.2.0** (BSD-3, one package, no
  dependencies of its own), pinned in `docs/plan/architecture.md`.

## v0.0.98 — 2026-09-18

A new device is set up from what Carrel already knows (S-23).

### For consumers

- **`/app/settings/devices` is live**, with four tabs (`?tab=apple`,
  `android`, `thunderbird`, `other`): every collection's address whole, its
  type (CalDAV, CardDAV, WebDAV) and a Copy action. File collections are only
  under "anything else".
- **The Apple profile** — CalDAV and CardDAV accounts with their passwords —
  is issued only inside the unlock window: outside it the button opens the
  password sheet, and `POST ?op=profile` issues nothing. Inside, it answers a
  one-time link that lives 5 minutes (`conf.DeviceProfileLinkTTL`).
- **The link opens once, for the same person signed in to Carrel.** Someone
  else's session, a used, expired or revoked link all answer 404. The
  profile is built from that session's data when the link is followed, in
  memory; it is never written to disk and is sent with `Cache-Control:
  no-store`. `op=revoke` burns the links not yet followed.
- **A connection can be left out** (`op=exclude` / `op=include` with
  `account=`): it goes into no profile, and its rows say so on every tab.
- Issue and download are in the audit log with the connections that went in,
  never the passwords.
- The QR code stays empty: no QR library is pinned.

## v0.0.97 — 2026-09-18

The card of a linked record is the card of the entity.

### For consumers

- **Opening a record of a linked group shows the group**: the name,
  organisation and photo (contacts) or title and place (events, tasks, notes)
  chosen when linking; every member's fields without exact repeats; and a
  single-valued field only another member holds, taken from it.
- **One row per other member**: `ContactPage.Books`, and the new
  `EventPage.Linked`, `TaskPage.Linked`, `NoteMeta.Linked` — `Label` is where
  it lives, `Href` its own card, `Value` what it holds differently
  ("Phone, email differ", "Holds the same") or that its server did not
  answer.
- **The card reads the other members from their servers** when it is opened.
  A server that does not answer is named in its row and the group stays; a
  member its server no longer holds leaves the group silently.
- Time, repetition, reminders and editing stay the opened record's own: an
  edit on the card writes that record only.

## v0.0.96 — 2026-09-18

Linking asks once where the records disagree, and remembers.

### For consumers

- **`DuplicateGroup.Choices`**: one `MergeRow` per single-valued field the
  loaded members hold differently — for contacts FN, ORG and PHOTO, for
  events SUMMARY and LOCATION, for tasks and notes SUMMARY. `Property` is the
  property name; a photo's options are named by where they come from.
- **The answer goes with Link**: `choice.<PROP>=<option>` beside `group` and
  `do=link`. It is kept with the linked group; `do=choose` changes it later
  (action `duplicates.choose`, "Keep these values"). Still nothing is written
  to a server.
- The folded row of a linked group shows the chosen value, and a merge of
  that group starts from the same answers.
- The values come from the loaded lists, not from a new request: a member
  whose list this session has not read offers nothing to choose.

## v0.0.95 — 2026-09-18

The points of duplicates are the person's to set, and one strong key is
enough by default.

### For consumers

- **Every signal's points can be set**, beside the threshold on the
  duplicates screen: `ThresholdControl.Weights` lists them (`Signal`,
  `Label`, `Value`, `Default`, `Own`). `POST /app/duplicates/decide` with
  `do=weights` and `weight.<signal>=N` saves the person's own; an empty value
  is the instance's weight again, 0 turns the signal off, a number outside
  0…1000 is not saved. Signals: `uid`, `email`, `phone`, `name`, `birthday`,
  `title-date`.
- **The instance's defaults are environment variables**:
  `CARREL_DUPLICATES_WEIGHT_UID` (80), `_EMAIL` (60), `_PHONE` (60), `_NAME`
  (20), `_BIRTHDAY` (10), `_TITLE_DATE` (70).
- **Breaking for anyone who counted on the old numbers:** email and phone
  score 60, so one shared address or number is a candidate on its own at the
  default threshold of 60. A shared family or office address is now offered;
  "Not duplicates" or a lower weight takes it away.
- A phone number of any length is a key; a short one used to be ignored.
- The reason formerly `title-when` is `title-date`.
- `TaskRow` and `NoteRow` carry `Badges` and `Sources`: a linked task or note
  says "2 sources" and where from, like a contact or an event.

## v0.0.94 — 2026-09-18

Duplicates are found, linked and merged (S-16).

### For consumers

- **`/app/duplicates` is live.** Groups are found only among records this
  session has already read — the screen asks no server, and says so when
  nothing is loaded. Each group carries its score and the reasons beside it
  ("same email, same name"); segments split them by kind; the threshold is
  the person's own once they set it, the instance's (`CARREL_DUPLICATES_THRESHOLD`)
  until then.
- **Link** and **Not duplicates** write the sealed blob and nothing else.
  `POST /app/duplicates/decide` takes `group` and `do` (`link`, `unlink`,
  `ignore`, `again`) and answers the group as a fragment, or `do=threshold`
  with `value` (empty: the instance's number again) and answers 303.
- **Linked records are one row** in every list: `State: "linked"`, and on
  contacts and the agenda a "2 sources" badge and `Sources`. A linked contact
  row fills an empty phone, email or organisation from the others.
- **Merge on the server** (`/app/duplicates/merge`): GET names the kept
  record, what is deleted and every value the kept record gains; a question
  only for a single-valued property whose values differ, answered as
  `choice.<PROP>=<index>`. POST writes the kept record with If-Match, reads it
  back, and deletes the others one by one with their own If-Match only when
  the server kept everything; the first refusal stops the rest and nothing is
  rolled back. A record that changed since the page was shown is not merged
  over. A read-only member makes the group unmergeable, said on the screen.
- The Duplicates row of the rail has a count: groups still to decide.
- A linked member leaves its group silently when a list read whole no longer
  holds it; an unavailable server takes nobody out.

### Included

- `internal/dupes`: `KeysOf`, `Find` (buckets by strong key, never every
  pair), `Score`, `Merge`, `Prune`, `Link`; names folded after NFD with
  `golang.org/x/text`, not `strings.EqualFold`.
- `sealed.Blob.Duplicates` (threshold, linked groups, ignored signatures) —
  an older blob reads as empty; `user.EditDuplicates`.
- The list index keeps each record's keys and a detail line.

## v0.0.93 — 2026-09-18

Long lines are folded on the way out, as both formats ask.

### For consumers

- **Every line Carrel writes is at most 75 octets.** An inline attachment or a
  photo used to go out as one line of hundreds of kilobytes, which RFC 5545
  §3.1 and RFC 6350 §3.2 both ask not to do: a parser with a line limit had
  every right to refuse it, and we would have heard about it from whoever
  could not open the record.
- **Neither library will do it, and both said so.** go-ical#18 was closed
  "I would rather not. This is just a SHOULD", and a pull request for it was
  refused outright; go-vcard closed two of the same. So the rule lives here,
  once for both formats (`docs/servers.md`, SV-31).
- A fold never falls inside a character, and folding an already folded
  document changes nothing.
- **Breaking for anything that reads a written record byte for byte.** What
  goes on the wire and into an export is folded now, so a search for a long
  value has to unfold first — a CRLF followed by one space or tab is not
  there. Properties, parsing and comparison are unchanged: unfolded, the
  record is exactly what it was.

### Included

- `model.FoldLines`, used by `cal.Encode` and `card.Encode`; documents built
  inside the service stay unfolded.
- **go-vcard raised to v0.1.0**, and that is part of this change rather than
  housekeeping: in the version pinned before, unfolding a long line was
  string concatenation in a loop, and a folded 500 KiB photo is some 7000
  lines — about 1.7 GB of copying to read one card. Upstream fixed it with a
  `strings.Builder` (their issue #41).
- `TestFoldLines` and neighbours in `internal/model`; `TestEncodeFoldsLongLines`
  in both providers, with 60 KiB of attachment and 40 KiB of photo going out
  in lines of 75 and coming back whole. Test helpers that read a raw body
  unfold it: an assertion about content should not know where a line broke.

## v0.0.92 — 2026-09-18

The live set stops shouting about a server that is not there, and a search
that ends wrong says why.

### For consumers

- Nothing in the running server changed: this is the live set.

### Included

- `radicaleEndpoint` skips the Radicale set when nothing is listening at the
  address. Radicale runs in docker-compose beside the gates of the Linux
  machine and nowhere else, so on any other machine fourteen tests failed with
  nothing to say — and a real failure is easy to lose among them. Only a
  refused connection is a skip, never a wrong answer from a server that is up.
- `scripts/gates.sh`: the gates of the plan in one command, with `--fix` for
  formatting. Its gofmt check compares gofmt’s output with the file ignoring
  carriage returns, so it tells the truth whatever the working copy’s line
  endings are — no more 106 false hits with a real one hidden among them — and
  it names a file gofmt cannot parse as exactly that. The working tree was
  rewritten to LF as well: 41 files predated `.gitattributes`, and no file’s
  content changed, the index having always held LF.
- The guard now says which rule it applied to a connection that ended before
  the answer: not sent again because the body cannot rewind, or because it is
  a write on a connection dialled for it, or sent again and ended the same
  way. A failure that does not repeat is read out of a log months later, and
  "EOF" alone leaves the reader guessing.
- `integration.sourceStates`: a live search that ends in a bad state, or is
  still running after 60 s, prints every source with its state and message
  rather than the first bad one. The one unexplained failure of
  `TestSoGoSearch` was lost because the run’s output was filtered to status
  lines, not because the test said nothing.

## v0.0.91 — 2026-09-18

A folder uploads as a folder (S-17.1).

### For consumers

- **A folder can be uploaded.** The browser sends the chosen folder as a set of
  files, each named by its path inside it; the service creates the folders that
  path needs before it writes the file, once each per upload, and writes every
  file where it belongs.
- **Every file is its own row in the queue, named by that path.** Two files
  called the same from different folders are told apart, and a taken name is
  the same question as ever — Skip, Keep both, Overwrite this one — asked about
  the file that clashed, with "Keep both" looking for a free name in the folder
  that file was going to. Nothing is overwritten (S-17.3), and a refusal on one
  file is not a refusal of the rest.
- **Not every name is a path.** A part named after a place on the person’s own
  machine — `C:\fakepath\photo.jpg`, or a name starting at the root — means the
  file name alone. A path that would lead out of the chosen folder is refused
  segment by segment and says so in its own row, rather than being quietly
  corrected (S-17.4).
- The two ways in, "Files" and "Folder", are the client’s two menu rows
  (U-6.5): one address, one method, one parse.

### Included

- `web.partName` reads the name from the part’s own Content-Disposition:
  `Part.FileName` strips the directory, as RFC 7578 §4.2 tells a server to, and
  a folder upload is exactly that directory.
- `web.uploadPath`, `web.makeFolders`, and `transfer.rel` carried through
  `place` and `freeName`, so an answer to a question lands in the right folder.
- `TestFolderUploadKeepsThePathsInside`, `TestFolderUploadRefusesAWayOut`, and
  live `TestSFTPGoFolderUpload` through the pages of a real file server.

## v0.0.90 — 2026-09-18

A collection with no name gets one, and a write is sent again when the server
had closed the connection before it.

### For consumers

- **A collection the server does not name is named by what it is.** iCloud
  sends no `displayname` at all for the only address book it has, and the last
  segment of that address reads "card" to the person whose contacts are in it.
  It is now called "Address book" — and where a home holds several nameless
  ones of a kind, each keeps that segment beside the name, "Address book
  (card)", because two identical rows are worse than a technical word
  (`docs/servers.md`, SV-29).
- **A write that died on a connection the pool had kept idle is sent again.**
  On shared hosting a server closes idle connections fast, and the request
  after one of those ends before the answer: `MKCALENDAR` on Nextcloud failed
  that way in a full live run, with nothing having reached the server. The
  guard now sends such a write once more — but only when the connection came
  out of the pool idle, because that is the case where the server had closed
  its side before the request and never saw it (S-5.1, SV-14).
- **A connection dialled for the request is not retried, and neither is a
  streamed body.** There the failure says nothing about what the server did,
  and a file on its way to another server cannot be produced twice.
- Safe by a second measure: every write Carrel sends is conditional (S-8) or
  idempotent, so one that did land answers the retry with a failed
  precondition instead of doubling.

### Included

- `discover.named` and `kindNoun`, with
  `TestACollectionWithoutANameIsNamedByItsKind`; the name is given where the
  whole home is in view, so `web.collectionLabel` is untouched and its fallback
  to the address stays for file collections.
- `dav.limitTransport` reads `httptrace.GotConnInfo` for `Reused` and
  `WasIdle`; `replayable` split into `readOnly` and `rewindable`. Four tests in
  `internal/dav/retry_test.go` hold the four cases.

## v0.0.89 — 2026-09-18

Nextcloud and iCloud are in the live set.

### For consumers

- **Nextcloud takes the whole harness.** Records, collections, search, a move,
  import and export, a collection deleted by name, a vCard 4.0 edit and a
  photo, all on Nextcloud 28.0.3 behind nginx on shared hosting. sabre/dav
  underneath, like Baikal, so the interest is the deployment: this account's
  principal path holds spaces ("MIX_1 for tests"), so every address in it
  arrives percent-encoded — the shape that breaks a client which builds
  addresses by hand. Nothing needed changing.
- **iCloud contacts work.** A card is created from the page, opens at the
  address the page gives, takes an edit of one field with the others untouched,
  and deletes. iCloud keeps a card exactly as it was sent — version, UID, name
  parts, a foreign property — and answers `addressbook-query` with and without
  a filter. No workaround was needed for any of it.
- **A calendar deleted on Nextcloud stays deleted.** Nextcloud puts it in its
  own trash bin, where it keeps its address, keeps its place among the children
  of the calendar home, and calls itself a calendar to any request that does not
  ask about the deletion. So the person deleted a calendar and the next
  discovery offered it back, alive. Discovery now asks about the deletion — one
  more property in the PROPFIND it already makes — and leaves out what the
  server says is deleted, saying on the trace how many (`docs/servers.md`,
  SV-30).
- **iCloud calendars do not work, and not because of Carrel.** The calendar
  host refuses the same Apple ID and app password that the contacts host
  accepts: 401 on every address and method (`docs/servers.md`, SV-28). The owner
  confirmed it: the account is a limited one, and Apple wants a sign-in from an
  Apple device before it will turn the calendar on. The connection says the
  refusal plainly and is not created.
- **iCloud's only address book has no name.** The server sends no
  `displayname` at all, so the book is labelled by the last segment of its
  address — "card" (SV-29). That is the address, not an invention; a friendlier
  fallback is an open question for the owner.

### Included

- `integration.nextcloudConfig` with the eight harness entry points, and
  `integration.icloudCardConfig` with `TestICloudContactLifecycle`.
- `integration.liveHasRecord`: the address and body of the first record that
  matches, or nothing — a live test may be proving absence, and absence is not
  a broken test.
- Both servers in `liveServers` for the discovery report, both in the
  credentials mapping, and Google, Nextcloud and iCloud now in the shapes
  `TestCredentialsFileIsRead` holds.
- `discover.propDeletedAt` and `davtest.Trash`, with
  `TestDiscoverySkipsWhatTheServerPutInItsTrashBin`. S-6 states the rule: a
  collection the server considers deleted is not a collection.
- The live check after a collection delete no longer demands a 404: the
  collection either goes, or stays and stops being a collection of its kind.
  That covers all three behaviours — Baikal and SoGo delete outright, Nextcloud
  hides in a trash bin.

## v0.0.88 — 2026-09-17

A name written shorter is the same name, and no longer reported as a loss.

### For consumers

- **Editing a contact on SoGo no longer claims the server dropped the name.**
  SoGo hands a structured value back with its trailing empty components gone —
  `N:Lovelace;Ada;;;` comes back `N:Lovelace;Ada` — which is the same name
  (RFC 6350 §6.2.2). The loss check compared the text, so every edit ended with
  "the server did not keep: N" over an intact name (`docs/servers.md`, SV-27).
- **A component that really changed still counts.** A dropped given name is a
  loss; a semicolon in a plain text property is still data.
- The false report appeared with the name fields of v0.0.84 — before them
  Carrel never wrote N — and the live suite caught it, not a fixture.

### Included

- `model.isStructured` (N, ADR, ORG, GENDER) and `withoutTrailingEmpty` in the
  loss comparison, with `TestCompareIgnoresTrailingEmptyComponents`.
- `TestSoGoTrimsEmptyNameComponents` holds the server's side of it.

## v0.0.87 — 2026-09-17

Contacts work on Google: the book reads, and a new card keeps the identity the
server gave it.

### For consumers

- **Google address books read.** Contacts on a Google account were listed as
  empty, a card would not open by the address its own list pointed at, and a
  search over the book found nothing and said nothing. All three were one
  server answering `addressbook-query` with an empty multistatus whatever the
  book holds, and one non-conformant element in Carrel's own request
  (`docs/servers.md`, SV-26).
- **A report that says the book is empty is now checked.** An unfiltered query
  for versions alone, against the book's own listing: two answers that carry no
  cards. Proven hollow, the book is read by listing and multiget, and a search
  over it says the sifting was Carrel's — the same note SV-23 already shows for
  Yandex. A server whose report works pays nothing extra.
- **No behaviour change for servers that answer.** Baikal, SoGo, Yandex and
  Radicale read and search as before; what changed on the wire is one element
  fewer.

### Included

- `dav.cardQueryBody` and the new `dav.cardMultiGetBody` ask for the whole card
  with an empty `address-data`, which is how RFC 6352 §10.4 asks for it.
  go-webdav's own multiget put `<D:allprop/>` inside it, and Google answers 400
  `INVALID_ARGUMENT` to that element in either namespace.
- `dav.CardFetcher` gained `ListCards`, `MultiGetCards` and `QueryEtags`;
  `FetchCards` is now the first two together.
- `card.confirmEmpty` behind `card.query`: an empty answer, filtered or not, is
  taken as the truth only when the book agrees.
- `davtest.HollowCardQuery` and `davtest.RefuseAllPropInAddressData`, with
  `TestHollowQueryIsCheckedAgainstTheBook`, `TestAnEmptyBookIsTakenAsEmpty` and
  `TestSearchLooksAgainWhenAQueryAnswersNothing`.
- `integration.pickAddress`: the credentials file names a server's protocols
  under keys of their own now, and a bare address among others is still read.

### Also: a record created under another identity

- **A new contact on Google no longer opens a 404.** Google names the resource
  itself and writes its own UID over the one it was sent, so the address the
  page sent the person to did not exist — the card was whole, and unreachable
  (`docs/servers.md`, SV-25).
- **A created record is now read back at the address it was written to** when
  nothing answers under the UID it was given, and the identity found there is
  the one it has in Carrel (S-7.1). That UID was Carrel's to give, so it is not
  counted as a property the server lost.
- **A record that is nowhere says so.** Neither under the identity nor at the
  address: an alert and a way into the collection, the way a move already
  answers (SV-18), instead of a redirect into nothing.
- Included: `web.finishCreate`, `web.reportWrite` split out of
  `finishWriteWith`, `model.CompareAfterCreate`, `web.collectionCtx.fetch`;
  `davtest.CreateRewritesUID` and `davtest.CreateHidesTheRecord` with
  `TestCreateWhereTheServerRewritesTheUID` and
  `TestCreateWhereTheServerShowsNothing`; live —
  `TestGoogleContactInTheOnlyBook`, the wrapper for what this account allows
  (neither a calendar nor a second address book can be made on it).

## v0.0.86 — 2026-09-17

The create menu works, and a new record starts where the last one was made.

### For consumers

- **The create menu in the top bar is no longer empty.** Contact, Event, Task
  and Note each open the quick sheet. A file and mail belong there too and are
  not built yet, so they are absent rather than shown dead.
- **The sheet opens where the last record of that kind was created.** Not a
  guess: it is the collection the person chose themselves last time, marked in
  the list of places to put it. The first time — or when that collection is
  gone, or has become read-only — the sheet asks where and shows no fields,
  because filling in a form with nowhere to put it helps nobody.
- **Only a record that was actually created counts.** Opening a sheet and
  walking away changes nothing, and none of this reaches the journal: a hint
  for a form is not an event of the account.

### Included

- `sealed.Blob.LastUsed`, one entry per kind, and `user.SetLastUsed` writing
  through `editBlob` without a journal entry. The field is optional, so a blob
  written before it reads as empty — there is nothing to migrate.
- `web.newMenu` and `web.getKindQuickEntry` at `/app/{kind}/quick`, with the
  route for the quick note folded into it: `/app/notes/quick` was a stub, and
  notes now go the same way as every other kind. The stub itself stays under an
  honest name, `sheetStub`, for the screens that still need one.
- `TestQuickSheetFromTheShellRemembersWhereRecordsGo` follows the whole way in:
  the menu points at the entry, the entry asks where while nothing is
  remembered, and goes straight to the collection once a record has been made.

## v0.0.85 — 2026-09-17

A record can be started from a sheet of a few fields, and nothing is written
until Save.

### For consumers

- **The quick sheet is there**, at `/new/quick` for contacts, events, tasks and
  notes: where the record goes, and the few fields without which it does not
  exist. The full creation form keeps `/new` and is not replaced — a record can
  be started either way.
- **"Open the card" saves nothing.** It carries what was typed into the full
  form, and until Save is pressed there is no record: opening the card and then
  changing your mind leaves nothing behind. What was typed travels in the body
  of the request, never in the address — a telephone number has no business in
  the address bar, in browser history or in a proxy's logs, and without
  JavaScript it behaves the same.
- **The sheet's fields are the full form's own**, named rather than copied, so
  the two screens cannot drift apart.

### Included

- `internal/web`: `getKindQuick`, `postKindQuick` and `quickSheet`;
  `recordFormOf` now draws the fields it is given, and `createRecord` is the
  one way a new record is written, used by both screens.
- `view.NewSheet` gains `Form`.
- `TestQuickSheetSavesOnlyOnSave` counts the writes before and after: opening
  the card writes none, Save writes one.

Still missing, and recorded in the plan: the create menu in the shell is `nil`,
so the sheet is reachable by address but not from the shell; and
`/app/notes/quick` is a stub that should fold into this one.

## v0.0.84 — 2026-09-17

The live tests take away what they make, on servers that name collections
themselves.

### For consumers

Nothing in the running server. This is the test suite keeping real accounts
clean.

### Included

- `integration/live_cleanup_test.go`: `makeCollection` creates a collection and
  answers with the address it actually got, comparing the home before and after
  — on a server that assigns its own (`docs/servers.md`, SV-19) the asked-for
  address holds nothing, so a cleanup aimed there took nothing away. The
  cleanup now goes through `removeLater`, which sends the delete again and logs
  "left on the server", instead of `_ = client.Delete(...)`, which swallowed
  the miss and let the leftovers pass unheard.
- Every create site moved over: fourteen mechanically, the two loops in
  `makeMoveCollections` by hand because a value loop had nowhere to put the
  address back, and `photo` and `vcard4`, whose cleanups already spoke up but
  aimed at the wrong address.
- `TestYandexMakeCollectionFindsTheRealAddress` holds it on the server that
  made it necessary: the helper must answer with an address other than the one
  it asked for, and the asked-for one must answer 404.

## v0.0.83 — 2026-09-17

An address book on a server without the search report is read instead of
falling over.

### For consumers

- **A book that could not be read now reads.** Some servers have no
  `addressbook-query` at all: Yandex answers 400 to it, and 403 with the
  supported-report precondition when a filter travels. Carrel read books only
  that way, so the whole source failed and the screen said "Personal is
  unavailable" — while the card sat on the server, written moments earlier by
  the same person. The book is now listed and its cards fetched by multiget,
  which that server does support.
- **A search over such a book is honest about who searched.** No filter travels
  with a fetch, so Carrel sifts the answer itself and the source says "this
  server does not search its address books, so Carrel sifted its answer" — the
  same words, and the same rule, as for a calendar server that ignores
  text-match (`docs/servers.md`, SV-16 and SV-23).
- **Nothing is lost from the list.** Fetching returns every card; only the
  searching moved.

### Included

- `internal/dav`: `FetchCards` beside `QueryCards` — a PROPFIND of depth one
  and an `addressbook-multiget`, behind the `CardFetcher` interface.
- `internal/provider/card`: the fallback fires only after a refusal that
  survived dropping the version, because that is what tells "no such report"
  from "not in that format"; `siftCards` then does the searching, and
  `ErrServerDoesNotQuery` carries the note.
- Watchdog with no network: `davtest.RefuseCardQuery` answers exactly as Yandex
  does, with `TestSearchFetchesWhatAServerCannotQuery`. Live, the Yandex test
  now requires what it used to merely print: the record page shows the name and
  carries no notices.

## v0.0.82 — 2026-09-17

A calendar handed over as a link stays a calendar instead of arriving as a
folder.

### For consumers

- **A link to one collection is taken for what it is.** Some servers give out
  the address of a single calendar rather than a home, and such an address
  names no principal — Mail.ru is one. Discovery used to read the missing
  principal as "this must be a file server" and offered the calendar as a
  folder of files. It now asks the address what it is first: a resource type
  carrying the calendar or address-book mark means the link is that collection,
  taken with its name and its components, and the connection holds it alone.
- **Nothing changes for file servers.** A file collection is one whose resource
  type carries neither mark, which is what S-6 said all along; SFTPGo and
  OpenCloud discover exactly as before.
- **The screen says which it was.** The trace step reads "the entered address
  is itself a calendar" where it used to read "1 file collection(s)", because a
  discovery that cannot be read is a discovery nobody can act on.

### Included

- `internal/discover`: `addressIsCollection` between the principal step and the
  file fallback, with the resource type and the component set read by local
  name — prefixes are declared on the root of the answer and every server picks
  its own. A refusal there changes nothing: the fallback runs as it did.
- Watchdogs: `TestEnteredAddressIsACalendar` and
  `TestEnteredAddressIsAnAddressBook`, both with a non-empty list of folders so
  that a file walk taking over would fail them; the file-server tests stay as
  they were. Live: `TestLiveDiscoveryReport`, where Mail.ru shows one calendar
  named «Основной».
- `docs/spec/carrel-server.md` (S-6) now says the rule outright, and
  `docs/servers.md` gains SV-22.

## v0.0.81 — 2026-09-17

The contact editor has the name fields the frames always drew, and a contact
written to a server that builds the display name from them arrives with a name.

### For consumers

- **Given and Family are in the contact editor.** The frames have held three
  name fields from the start — Display name, Given, Family — and the editor
  offered one: it wrote `FN` and never `N`. Both parts are written now.
- **The two sides fill each other in.** An empty display name is made from
  Given and Family when the card is saved. A display name typed on its own is
  cut left to right — the first word is the given name, the rest the family
  name — and a hint under the field says so, because the order cannot be
  guessed: «Ковалёв Пётр» would come apart the wrong way round. Only a display
  name that was **changed** is cut: editing a phone on a card that carries no
  `N` does not quietly grow one.
- **What no field offers is left alone.** A middle name, a prefix and a suffix
  another client wrote come back exactly as they were.
- **On Yandex a contact now arrives with a name.** That server builds `FN` from
  `N` and throws away the `FN` it was sent, so a card with an empty `N` had no
  name at all there — `FN:` and `N:;;;;` with nothing in them
  (`docs/servers.md`, SV-21).

### Included

- `internal/web/record_edit.go`: a field type that is two inputs and one
  property (`fieldName`), `nameOps` beside `textOps`, and the five loops of the
  editor — read, empty, submitted, changed, drawn.
- Watchdogs with no network: `TestContactNameFieldsWriteN`,
  `TestEditingAnotherFieldDoesNotGrowAName`, and
  `card.TestEncodeKeepsStructuredName`, which covers what nothing covered — `N`
  travels as one flat value and its five components survive the trip. Live:
  `TestYandexContactInTheOnlyBook`, where the card came back holding
  `FN:Carrel Probe …` and `N:Probe …;Carrel;;;`.

## v0.0.80 — 2026-09-17

A new collection goes into the catalog at the address the server actually made
it at, and Yandex joins the live set.

### For consumers

- **A new collection is found where the server put it.** A server may answer a
  create with 201 and make the collection at an address of its own, sending
  nothing to say where: Yandex names a new calendar `events-<id>` whatever
  address it was asked for. Carrel now reads the address back, and when nothing
  is there, reads the home again — the one collection that was not there before
  the create is the new one, and that is what the catalog keeps. It matters
  most when a collection is moved to such a server, because the address of the
  destination is taken before the records are copied into it
  (`docs/servers.md`, SV-19).
- **When there is no telling where it went, Carrel says so** instead of filling
  the catalog in with a guess: the screen says the server made it at an address
  of its own, and that re-discovery on that connection will take it in.

### Included

- `integration/yandex_live_test.go` (S-30.2): the shared harness does not fit
  this server — every wrapper seeds an address book first, and Yandex refuses
  MKCOL of a book with 405 from nginx — so what it does allow is tested
  instead: a calendar made, renamed and deleted through the pages, a refused
  book that arrives as the content of the sheet, and a contact written into the
  one book the account has.
- A watchdog that needs no network: `davtest.CreateAssignsItsOwnAddress` with
  `TestCreateWhereTheServerPicksTheAddress`.
- Two findings recorded: an address book cannot be made on Yandex at all
  (SV-20), and the server builds `FN` from `N` and discards the `FN` it was
  sent — so a contact made through Carrel, whose form writes `FN` and no `N`,
  arrives there without a name (SV-21). That second one is a gap on this side,
  not a quirk to work around: the frames have held Given and Family beside the
  display name from the start, Carrel already reads `N` and keeps it, and only
  the fields to enter it are missing. The spec said nothing about them and now
  does (U-6.1); the editor itself is still to come.

## v0.0.79 — 2026-09-16

SoGo joins the live set, and three things it does its own way are now held by
Carrel instead of by luck.

### For consumers

- **A search on a server that does not search.** SoGo answers a calendar
  text-match with the whole collection, whatever was asked of it. Carrel sifts
  such an answer through the same properties and the same term, so what is
  shown holds the text, and the source says "this server does not search its
  calendars, so Carrel sifted its answer" beside the results. A server that
  filtered for itself loses nothing by this (`docs/servers.md`, SV-16).
- **A new collection keeps the name the server shows.** SoGo names a new
  address book after its address and ignores the name the create carried.
  Carrel reads the name back, sends it once more on its own when it differs,
  and keeps in the catalog what the server shows — because that is the name a
  person sees, and the one the delete sheet asks them to type (SV-17).
- **A move that changes the identity of a record is said out loud.** SoGo
  gives a card moved into another address book a UID of its own. That
  identifier is what duplicates are matched by and what a move promises not to
  change, so instead of a redirect to an address that no longer opens
  anything, the page says the record is there under another identity and
  offers the collection it went into (SV-18).

### Included

- `integration/sogo_live_test.go` (S-30.2): seven wrappers over the same
  harness the other servers run — records, collections, search, a move, import
  and export, a collection deleted by name, and a vCard 4 edit. No new code
  stands behind them, which is the point of a harness parameterised by server.
- Watchdogs that need no network: `davtest.SearchIgnoresFilter` and
  `davtest.MoveRewritesUID`, with `TestSearchSiftsWhatAServerDoesNotFilter`
  and `TestMoveWhereTheServerChangesTheUID`.
- The live tests no longer assume the address, the name or the identifier a
  server gives a record: they look for what is written in it, take both
  endings of a move, and a decode that met a redirect now says where it went.

## v0.0.78 — 2026-09-16

Copies the Typecase mockup through plan 1 step 5: inline `style=` is gone
from frames; one-off layout is `--rN` and `tc-table--fit`. The Р-13
visible list is approved.

### For consumers

Nothing in the running server. Open
[carrel-ui-mockups.html](../../docs/visual/carrel-ui-mockups.html) for the
current mockup; fonts are in `docs/visual/fonts/`.

## v0.0.77 — 2026-09-16

Attachments have run on live servers, and a read that meets a connection the
server has closed is sent again instead of failing.

### For consumers

- **A closed idle connection no longer fails a read.** Servers close
  connections they have kept for a while (Apache does it after 5 seconds), and
  the pool that holds them is shared by every DAV client of the process. Go
  sends again only the methods it knows as safe, and `PROPFIND` and `REPORT`
  are not among them, so such a request failed with `EOF` and the screen said
  the server could not be reached. Carrel now sends it a second time: only a
  method that reads, only when the body can be produced again, and only once.
  **A write is never sent twice** — the server may have taken the first one —
  so a PUT that meets a closed connection still says so.
- This is what made a live test fail now and then with no explanation, and it
  is what stopped an attachment from being written on a live run
  (`docs/servers.md`, SV-14; SV-9 said the pool was per client, which was
  wrong).

### Included

- `integration/attach_live_test.go` (S-20): the calendar on one server and the
  attachment folder on another, which is how two connections of one person
  stand. A 3 KB file goes into the record on Baikal and comes back byte for
  byte; the 200 KB attachment another client left inline is untouched by every
  write that follows; a 2.7 MB file is streamed to the folder on SFTPGo as
  `2026-08-10 Синк-ап … - report.pdf`, the record keeps the link with `SIZE`,
  and `/app/attach` gives all of it back; the same name again becomes
  `- report-2.pdf` with the first file unchanged; removing the link leaves the
  file in the folder. The folder itself is chosen through the settings page.
- `internal/dav`: the retry in `limitTransport`, with `retry_test.go` — a read
  sent again, a write never, a timeout or a refusal never, and one retry only.
- `TestRadicaleAttachments` is the same run against Radicale; it belongs to the
  Linux gates and has not run yet.
- `gofmt`, `go vet`, `go test ./...`, `docs/spec/check.py` green; the live run
  green from Windows against Baikal and SFTPGo (59 s).

## v0.0.76 — 2026-09-16

A file attached to an event, a task or a note travels inside the record when it
fits, and goes to a folder on your file server when it does not (S-20).

### For consumers

- **Where it is.** Every event, task and note carries an `Attachments` block on
  its reading page and on its edit page: `Attach` (`attach.add`) adds one,
  `Remove` (`attach.remove`) takes one off. Both are writes of their own on the
  edit address (`?op=attach`, `?op=detach`) with `If-Match` of the version the
  page showed, the way the steps of a contact photo are — no new address. A
  record that changed elsewhere is not written to, and the screen says so.
- **The boundary is 2 MB, and nothing raises it.** Up to it the file is written
  into the record itself (`ATTACH` with binary data) and travels with it: a
  phone with only the calendar gets the file with the event. The size is
  measured on the finished object before the PUT, so a collection that declares
  a smaller `max-resource-size` lowers the boundary for itself.
- **Larger goes to the attachment folder**, chosen once in Settings →
  Attachments, and the record keeps a link. The folder is made on first use and
  stays an ordinary folder. Without a folder chosen nothing is written at all:
  the notice says where to choose one, and the record is not left pointing at
  something that is not there. A write the server then refuses takes the file
  back out of the folder.
- **The name in the folder** is the date of the record, its title and the name
  the file came with: `2026-08-10 Синк-ап по релизу - report.pdf`. Characters a
  file system refuses become `-`, a long title is cut, the extension is not. A
  name already taken gets a number (`- report-2.pdf`); nothing is overwritten,
  and the row of the attachment shows the name the file actually got.
- **What another client stored stays as it is.** An inline attachment is never
  moved to a folder, whatever its size; a record already over the boundary
  simply sends its next attachment to the folder. Removing a link removes the
  link — the file stays, the label says so — while an inline attachment goes
  with its only copy.
- **Opening one** is `/app/attach/{account}/{col}/…`: an inline attachment as
  `{uid}/{n}` straight from its record, a link only when it points into a file
  collection of your catalog. A link anywhere else is shown and never fetched.
  Attachments are handed over the way downloads are: an attachment in a sandbox,
  `nosniff`, `private, no-cache`.
- **Settings → Attachments** shows the two ways side by side — what stays in the
  record, what goes to the folder, and the boundary each collection declares —
  and none of them is editable. The folder is picked in the same tree as a move
  or a copy, with an optional folder to make inside it. Changing the folder
  moves nothing: records already written keep their links.

### Included

- `internal/provider/cal/attach.go`: `Attachments`, `AttachmentData`,
  `AttachInline`, `AttachLink`, `Detach`. Adding one appends a value; removing
  one writes every other value back with its parameters. A name with a double
  quote, which has no escape in an iCalendar parameter, is written with
  apostrophes instead of breaking the record.
- `internal/web/attach.go`, `internal/web/attach_settings.go`;
  `sealed.AttachmentFolder` in the catalog and `user.SetAttachmentFolder`
  (journal `attachments.folder`). The upload streams: the attachment route
  joins the file upload route in reading `_csrf` in front of the file, so the
  body has no ceiling of Carrel's (C-12).
- `sendFile` and `downloadHeaders` shared by the files section and attachments;
  `folderAddress` shared by a move, a copy and the settings tree.
- Tests: `provider/cal/attach_test.go` and `web/attach_test.go` — the boundary
  both ways, the name and its cleaning, the counter on a taken name, a missing
  folder, a record changed elsewhere, a foreign inline attachment left byte for
  byte, a link removed with its file kept, a read-only collection, and the
  settings page.
- `gofmt`, `go vet`, `go test ./...`, `docs/spec/check.py` green. Not live yet:
  Radicale, Baikal, SFTPGo.

## v0.0.75 — 2026-09-15

`docs/servers.md` is shorter and up to date. Nothing changes in the running
server.

### Included

- Every entry SV-1…SV-13 keeps its number: the server behaviour, what Carrel
  does, the version and the test that notices a change. A comment in code that
  names `SV-n` still leads to it.
- The line-up says that one live suite runs Radicale, Baikal (sabre/dav 4.7.0)
  and SFTPGo on the Linux machine.
- SV-12 names the live tests that hold what `davtest` imitates, moving and
  deleting a collection included.

## v0.0.74 — 2026-09-15

Import, export and deleting a collection have run on live servers. Nothing
changes in the running server.

### Included

- `integration/import_live_test.go` (S-19): on Radicale and Baikal, a `.ics`
  whose series takes an identity the calendar holds goes in under a new one,
  its instance too, and the series that was there is untouched; a task the
  calendar of events does not take is skipped; a taken card is created under
  a new identity. The book goes out as `.vcf`, the calendar with `whole=1` as
  one `.ics` envelope holding the task once. Markdown files in an SFTPGo
  folder become notes in a Baikal notebook, which goes out as `.md`.
- `integration/collection_delete_live_test.go` (S-12.4): on Radicale and
  Baikal, the sheet counts what the server holds, its file carries the series,
  the instance and the task, a name in another case sends nothing, and the
  name shown deletes the collection on the server and out of the catalog.
- Green with the whole live suite on the Linux machine (59 s): Radicale in
  Docker, Baikal and SFTPGo over the internet.

## v0.0.73 — 2026-09-15

A calendar or an address book is deleted on the server, once its name is
typed (S-12.4).

### For consumers

- **Where it opens.** Every collection row of the rail and of the connections
  page ends with `Delete…` (`collection.delete`, `danger`, after a separator),
  read-only rows included: the right to delete is not guessed. The sheet is
  `/app/collections/delete?a=&c=` (`CollectionDeleteSheet`). A file collection
  has none: it is the root of a storage, and its folders go through `op`.
- **What the sheet says before anything is sent.** `Warning` names the server
  and how many records go with it — counted by one PROPFIND, left out when the
  server does not answer — and that Carrel keeps no copy. `References` is an
  empty list: published links, backup jobs and davloom devices fill it once
  those exist. `Archive` says that no backup job holds it, and that a restore
  brings records back into a new collection at a new address.
- **The file comes first.** `Export` is `Download .vcf first` or
  `Download .ics first`, the same stream as S-19. A calendar's link carries
  `whole=1`: every component it holds, events, tasks and notes, in one `.ics`
  with no range, so a task in a calendar of events is not lost with it. An
  empty collection has no `Export`, and `ExportNote` says there is nothing in
  it.
- **The confirmation is the name.** `Form` holds `name`; `Hint` is the name as
  shown. It must match in letters and case — spaces around it and the Unicode
  form it was typed in do not count. Anything else answers the sheet again
  with the field's error and the typed value, and nothing goes to the server.
- **After.** The name typed sends `DELETE` on the collection and redirects to
  `/app/settings/connections`; the catalog loses the entry and its ticks, and
  the journal writes `collection.delete` with the connection. A refusal leaves
  the collection, in the catalog too, and the connections page carries the
  notice with method, address, code and body. A server answering 404 no longer
  has it: it leaves the catalog the same way.

### Included

- `internal/web/collection_delete.go`; `user.RemoveCollection`;
  `collectionCtx.allRecords` shared by the export and the move (S-12.3), a
  resource two REPORTs name read once.
- Tests: `web/collection_delete_test.go` — the menu in both places, the sheet
  and its file holding a task and an instance, a book and an empty calendar,
  wrong names that send nothing, a delete that forgets the ticks, a decomposed
  name, a refusal, a server that has it no more.
- Not live yet: Radicale, Baikal.

## v0.0.72 — 2026-09-15

A file is imported into a collection, and a collection is handed over as a
file (S-19).

### For consumers

- **Where it opens.** The list of one collection has `Import…`
  (`collection.import`) and `Export` (`collection.export`) beside `New …`, and
  every collection row of the rail has both. The wizard is
  `/app/{kind}/{account}/{col}/import` (`ImportPage`); the export is
  `/app/{kind}/{account}/{col}/export` and answers with the file itself.
- **The wizard.** `Step` is `source`, then `preview`, then `report`.
  `Source.Kinds` are the formats of the kind: `.vcf` for contacts, `.ics` for
  events, tasks and notes, and `.md` for notes as well. The file goes in the
  `file` part of a multipart POST, with `_csrf` before it or `X-CSRF-Token` on
  the request.
- **The destination is chosen, not assumed.** `Into` holds every collection of
  the kind, read-only ones shown and closed with the reason; an option's `ID`
  is the wizard with that collection chosen (`to`, `into`), and choosing
  another one counts the same file against it again. The list the wizard was
  opened from is only the first value.
- **Notes have a second source** (`Source.Folder`): a folder of a file
  collection of this person, read one level deep through the guard. `.md` and
  `.markdown` become notes; a file with a header block gives back what the
  block names, a file without one is a note whose body is the whole file.
- **What the preview says.** One table of what will be created in the chosen
  collection, each record `New` or marked as already there under this
  identity. Its columns are the kind's own: a contact by its address, an event
  and a task by their time, a note by its date. A second table names what this
  import does not write — a task or a
  note in a calendar of events, a `VFREEBUSY`, an occurrence whose series the
  file does not hold — with the section it belongs in. Half a file is never
  dropped in silence.
- **An import always creates.** Every write is `If-None-Match: *`. A record
  whose identity the collection already holds is given a fresh one before the
  write — in the instances of a series too, so it stays one record — and the
  record that was there is not touched. `ImportReport` counts `Created`,
  `Renamed` and `Skipped`, lists every row and names what the server refused.
  The file waits in `DraftPendingImport` until it is written, dropped, or the
  session ends; a run empties that slot, so a report cannot be run twice.
- **Ceilings.** `CARREL_IMPORT_MAX_BYTES` (16 MiB) on the file, and on a
  folder taken together; `CARREL_IMPORT_MAX_RECORDS` (5000) on the records in
  it. Over either, the whole file is refused: half of it is not written and
  then called an import.
- **The export has no screen.** It is a GET that writes the file as the
  records arrive — no temporary file — always an attachment in a sandbox
  (`Content-Disposition: attachment`, `nosniff`, `sandbox`,
  `Cache-Control: private, no-store`), named after the collection. A book is
  `.vcf`, a calendar and a task list `.ics` under one envelope with each time
  zone written once, a notebook `.md`.
- **A note goes out whole.** The header block names `title`, `date`,
  `categories` and `uid`; every other property of the record keeps its own
  name below them, parameters included, and the body is the description
  unescaped. Nothing of the record is left out of the file.
- **A calendar takes a range where the screen has one:** the agenda's Export
  carries the week it shows (`from`, `to`); without them the collection goes
  whole.
- **A source that does not answer** leaves the list screen with the usual
  notice (S-21), not a broken download.

### Included

- `internal/provider/cal`: `Split`, `Components`, `SetUID`, `NoteMarkdown`,
  `ParseNote`, `ProdID`, and the line-level helpers under them — an imported
  file is cut into records and an exported one copied out without decoding
  either, so what another program wrote travels as it wrote it.
- `internal/provider/card`: `Split`, `SetUID`.
- `internal/web/import.go`, `internal/web/export.go`; the import route's body
  ceiling in `bodyLimit`; `listActions` on the four lists; import and export in
  the rail's collection menu.
- Tests: `cal/split_test.go`, `cal/note_test.go`, `card/split_test.go` —
  series with instances and zones, strays, a file that left out UID or
  DTSTAMP, a note written and read back. `web/import_test.go`,
  `web/export_test.go` — the preview against two collections, an import that
  creates and renames, a second run that writes nothing, a file over the
  ceiling, notes from a folder, one envelope and one zone on the way out, the
  week of an agenda, an unavailable source.
- Not live yet: Radicale, Baikal and SFTPGo.

## v0.0.71 — 2026-09-15

Moving a whole collection has run between two live servers. Nothing changes in
the running server.

### For consumers

- **Checked.** A calendar with a recurring event, its overridden instance and a
  task moved from Radicale to Baikal: it was made there with its name, colour
  and component set, every record arrived whole, and the original was gone from
  the server and the catalog. An address book copied the same way left the
  original where it was.

### Included

- `integration/collection_move_live_test.go`:
  `TestRadicaleBaikalCollectionMove`. Green with the whole live suite on the
  Linux machine.

## v0.0.70 — 2026-09-15

A calendar or an address book moves or is copied to another connection
(S-12.3).

### For consumers

- **Where it opens.** A collection row on the connections page and in the rail
  has `Move to another server…` (`collection.move`) when another connection
  can hold its kind; with none there is no item, and the sheet answers 404.
  Files are not offered: they move as a tree (S-17).
- **The sheet.** `/app/collections/move?a=&c=` (`CollectionMoveSheet`):
  `Subject` is the collection and its server; every connection is an option,
  the current one `Already here`, one without a home for the kind closed with
  the reason. An option's `ID` is the sheet with it chosen (`to`). The confirm
  (`collection.move.confirm` / `collection.copy.confirm`) is closed with a
  reason until a server is chosen; the second action switches between move and
  copy (`copy=1`). `Warning`, before the start, says what happens — for a
  move, that the address changes and the original goes only if every record
  arrived whole. A read-only collection can be copied, not moved.
- **What a move does.** The collection is made there with the name and colour
  the server holds now and the same component set, at the same last segment
  of its address when that is free (else `-2`). Every resource of the original
  goes one at a time: a record as S-18 sends it, anything else — a broken
  record, another component — as its bytes. The new collection is read back
  and each record compared.
- **When the original stays.** A refused record takes the new collection back
  and nothing changes. A loss, a copy that cannot be read back, a change to the
  original while it was copied (its members and versions are compared before
  the delete) or a refused delete leave both collections, with a notice on the
  connections page and a link to the copy.
- **Catalog.** A move carries the ticks of the original to the new collection
  and takes the original out; a copy ticks nothing. The journal has
  `collection.move` / `collection.copy` with both connections, not addresses.
- **Limits.** One request does the whole move: a reverse proxy with a short
  read timeout can cut the answer of a large collection while the move goes
  on. Each whole-collection REPORT is under `CARREL_DAV_MAX_RESPONSE_BYTES`.

### Included

- `internal/web/collection_move.go`; `user.PlaceMovedCollection`;
  `newCollectionCtx` opens a collection that is not in the catalog yet.
- `davtest`: `DELETE` of a collection, and depth-0 PROPFIND of a collection
  with `displayname` and `calendar-color` (`docs/servers.md`, SV-12).
- Tests: `collection_move_test.go` — the menu and the sheet, move with
  override, task, colour and ticks, copy to a taken address, refusal taking
  the new one back, loss keeping both, read-only copies not moves.
- Not live yet: Radicale and Baikal.

## v0.0.69 — 2026-09-15

Moving and copying records has run on live servers. Nothing changes in the
running server.

### For consumers

- **Checked.** On Radicale and on Baikal, a contact moved between two address
  books of one account is one MOVE and arrives byte for byte; the copy back
  keeps the UID and a foreign property. An event with an overridden instance
  moves with its RRULE and RECURRENCE-ID, and the agenda of the target shows
  the override. Between accounts, a contact from Radicale to Baikal and an
  event from Baikal to Radicale arrive whole and the original is gone.

### Included

- `integration/move_live_test.go`: `TestRadicaleMove`, `TestBaikalMove`,
  `TestRadicaleBaikalMove`. Green with the whole live suite on the Linux
  machine; `TestBaikalMove` also from Windows.

## v0.0.68 — 2026-09-15

A contact, an event, a task or a note moves or is copied to another
collection of its kind, on this account or another one (S-18).

### For consumers

- **Where it opens.** The record page has `Move…` (`record.move`) and `Copy…`
  (`record.copy`) beside Edit and Delete; a note has them with Delete. A
  record in a read-only collection keeps `Copy…` only. The sheet is
  `/app/{kind}/{account}/{col}/{uid}/move` (`MoveSheet`), `?copy=1` for the
  copy.
- **The sheet.** Every live collection of the kind, the current one (`Already
  here`) and read-only ones (`Read-only`) shown and closed. An option's `ID`
  is the sheet with it chosen (`to`, `into`); the confirm
  (`record.move.confirm` / `record.copy.confirm`) is closed with a reason
  until a collection that can take the record is chosen. A move to another
  account says before it starts that it is a copy, a check and a delete.
- **On one account a move is one MOVE.** The server moves the resource; no
  body travels and nothing is re-encoded.
- **To another account, and any copy,** the record read whole — time zone and
  overridden instances with the event — is created there with its UID. A move
  reads the copy back and deletes the original, with If-Match of the version
  read, only if nothing was lost. A loss, a copy that cannot be read back, a
  refused or conditional-failed delete leave both records and say so, with a
  link to the copy. A copy with a loss shows the loss report, as an edit does.
- **Names.** The file name stays; a name taken there becomes `{uid}-2.vcf` /
  `.ics` once. A UID the target already holds stops before any write, with a
  link to the record there.
- **Stamps.** A copy is a write through Carrel: `DTSTAMP` and `LAST-MODIFIED`,
  or `REV`, become the time of the copy (as in `v0.0.66`). A MOVE keeps the
  bytes as they were.

### Included

- `internal/web/record_move.go`; `collectionOf` opens a catalog collection
  without a request, for the target.
- `davtest` answers MOVE of a record (go-webdav's CalDAV and CardDAV backends
  answer 501): 412 on a taken name with `Overwrite: F`, 409 outside a
  collection.
- Tests: `record_move_test.go` — the sheet, MOVE on one account, copy and
  delete across accounts, loss and changed original keep both, copy keeps the
  UID, taken name and taken UID, read-only source and target, an event with
  VTIMEZONE and RECURRENCE-ID.
- Not live yet: MOVE between collections on Radicale and Baikal.

## v0.0.67 — 2026-09-15

Three live checks the plan still listed as open have run green. Nothing
changes in the running server.

### For consumers

- **Checked.** Collections (create, rename) and case-insensitive search on
  Radicale pass in the Linux gates; files on SFTPGo — an 11 MiB file whole, a
  taken name, MOVE without overwrite, MKCOL, copying a tree — pass live.

### Included

- `docs/plan/server.md`: `TestRadicaleCollections`, `TestRadicaleSearch` and
  `TestSFTPGoFiles` ticked with the run they passed on.

## v0.0.66 — 2026-09-15

A record written through Carrel carries the time of that write, so clients
show Carrel's edit as the latest change.

### For consumers

- **Events, tasks and notes.** Every write — a new record, an edit, the
  conflict screen, reminders — sets `DTSTAMP` and `LAST-MODIFIED` of the main
  component to the time of the write, in UTC. RFC 5545 §3.8.7.2 has `DTSTAMP`
  of an object without METHOD say when it was last revised. Until now an edit
  left both as they were, and jtx showed the time of change from before it.
- **Contacts.** Every write, the photo included, sets `REV` (RFC 6350 §6.7.4).
  The card page shows `REV` as the server's time of change.
- **Loss report.** Unchanged: these properties were already not counted as
  lost when a server rewrites them.

### Included

- `cal.Provider` and `card.Provider` stamp the object before `Create` and
  `Update`, through a patch.
- Tests: `TestWritesStampTheRevision` in `cal` and `card`; live
  `TestBaikalRecords`, `TestBaikalVCard4Edit`,
  `TestBaikalCardForeignPropertyRoundTrip` green.

## v0.0.65 — 2026-09-15

A note written in a real foreign client was read, edited and read again. Nothing
changes in the running server.

### For consumers

- **Checked.** jtx Board through DAVx5 4.5.19 on Baikal: a note with CLASS,
  COLOR, GEO, LOCATION, STATUS, URL, CATEGORIES, COMMENT and an inline
  attachment of 395 KiB. Carrel read the text with escaped commas,
  semicolons, backslashes and emoji as written; an edit of title and text
  kept every other property and the attachment byte for byte, with no loss
  report; jtx showed the edit and everything else after syncing.
- **Seen on the way, not changed yet.**
  - An edit leaves `LAST-MODIFIED` and `DTSTAMP` as they were, so a client
    shows the old time of change.
  - Carrel writes long lines unfolded (go-ical does not fold) and sorts
    properties and parameters: an inline attachment goes out as one line of
    hundreds of KiB. jtx and Baikal accept it.
  - Edited at the same time in jtx, the note kept jtx's version over an edit
    Carrel had written with the right If-Match; Baikal refuses a stale
    If-Match. How DAVx5 sent its PUT is still to be seen.

## v0.0.64 — 2026-09-15

Every server quirk in [docs/servers.md](../../docs/servers.md) now has a test
that notices when the server changes, and the connection trace says why a
well-known redirect was not followed.

### For consumers

- **Connection trace.** A failed `.well-known` step carries its reason, such as
  "GET /.well-known/caldav: dav: redirect downgrades HTTPS to HTTP". It was
  an empty detail; on the test Baikal both well-known steps looked failed for
  no reason.
- **Checked on the live servers, 15.09.2026.** SFTPGo still overwrites a taken
  name despite If-None-Match (SV-8). The Apache in front of Baikal closes an
  idle connection after 5 s (SV-9). Baikal's well-known answers https with a
  302 to `http://…/dav.php` (SV-10).

### Included

- `dav.ResolveWellKnown` returns a detail for a failed request; test
  `TestResolveWellKnownSaysWhyItFailed`.
- Live watches: `TestSFTPGoIgnoresIfNoneMatch` (in place of the skipped
  `TestSFTPGoUploadOccupiedName`), `TestBaikalIdleConnectionClosed`,
  `TestBaikalWellKnownDowngradesScheme`.
- `docs/servers.md`: SV-8, SV-9 and SV-10 name their watches and what was
  seen.

## v0.0.63 — 2026-09-15

Server quirks found and worked around are kept in one place:
[docs/servers.md](../../docs/servers.md). Nothing changes in the running server.

### For consumers

- **What is there.** Each deviation of a real DAV server or library from the
  standard, or from what Carrel expects: the version it was seen on, what the
  standard says, what the server does, what Carrel does about it, and the test
  that will notice when the server changes. Radicale, sabre/dav (Baikal,
  Nextcloud), SFTPGo, the Apache in front of Baikal, go-webdav and go-vcard;
  thirteen entries, `SV-1` to `SV-13`.
- **Without a watch.** SFTPGo's If-None-Match (SV-8), Apache's keep-alive
  (SV-9) and Baikal's well-known downgrade (SV-10) have no test that would
  notice a change; the file says so.

### Included

- `docs/servers.md`; the plan links to it from «Живые серверы» and
  `providers-tail.md`.
- Comments at the workarounds name their entry (`docs/servers.md, SV-n`).

## v0.0.62 — 2026-09-15

When a server cuts a contact photo, the loss report offers to save it in the
older form.

### For consumers

- **The offer.** Saving a photo into a vCard 4.0 card writes it as RFC 6350
  has it, a `data:` URI. A server that cuts that (Radicale) now answers with
  the loss report for `PHOTO`, a notice "The server did not keep the photo"
  that names the risk, and `loss.actions` with `photo.save.legacy` — POST to
  the same `photo-preview` address, `op=legacy`.
- **The older form.** `PHOTO;ENCODING=b;TYPE=JPEG:` with the base64, in the
  same card version: this server keeps it whole, but apps that follow vCard
  4.0 strictly may not show it. The choice is per card, not a setting.
- **The frame waits.** Until the person answers, the framed photo stays in the
  session draft at the version the first save left; the older form writes with
  that version as If-Match, and the draft goes. Any other outcome of a save
  clears the draft as before. A 3.0 card, which already writes base64, is
  offered nothing.
- **`LossReport`** (C-9) gains `actions`: what can be done about the loss;
  empty when nothing can. Record edits leave it empty.

### Included

- `web`: `finishWriteWith` — `finishWrite` with a say in the loss report;
  `photoSave` writes the older form for `op=legacy`.
- `card`: `PhotoOpsBase64`, `PhotoIsDataURI`.
- `davtest`: `CutPhotoDataURI`, Radicale's cut.
- Spec: C-9 `LossReport.Actions`; S-13 says the version kept is the one the
  server hands over (a converting server hands over 4.0), and how a cut photo
  is offered the older form.
- Tests: `TestPhotoOlderFormAfterACutDataURI`,
  `TestPhotoOpsBase64KeepsTheVersion`; `TestRadicalePhoto` takes the offer and
  checks the photo is kept.

## v0.0.61 — 2026-09-15

The live photo test holds Radicale to the loss it causes. Nothing changes in
the running server.

### For consumers

- **Radicale and contact photos.** Radicale (3.5.4 and 3.8.0 checked) cuts a
  vCard 4.0 photo written as RFC 6350 has it, `PHOTO:data:image/jpeg;base64,…`,
  at its comma: the server keeps `data:image/jpeg\;base64` and the image is
  gone. Saving a photo into a 4.0 card on Radicale therefore answers with the
  loss report for `PHOTO`. A 3.0 card keeps its photo. Radicale's issues #1795
  and #1863 were closed without a fix.
- **Carrel keeps writing RFC 6350.** Baikal and Nextcloud keep the photo
  whole. A way to save the photo in the older form, from the loss report, is
  the next step.

### Included

- `TestRadicalePhoto` expects the loss report for `PHOTO` in a 4.0 card and
  checks the server really cut it; it fails the day Radicale keeps the photo.
  `TestBaikalPhoto` expects the photo kept.

## v0.0.60 — 2026-09-15

vCard 4.0 cards on sabre/dav servers (Baikal, Nextcloud) are no longer
downgraded to 3.0 by an edit, and keep what only 4.0 holds.

### For consumers

- **What was wrong.** Carrel read contacts with a REPORT that named no vCard
  version. sabre/dav answers such a REPORT in 3.0 and drops what 3.0 cannot
  hold — `KIND`, among others. An edit through Carrel then wrote the card back
  as 3.0: on Baikal a 4.0 card with `KIND:individual` lost it for good when
  only `FN` was changed. The loss report did not show it: it happened on read,
  before the patch. Found by the live checks of S-13.
- **Now.** Every contact read (list, card, search, the lookup by UID) asks for
  `text/vcard; version=4.0`. A server that keeps cards as sent (Radicale)
  answers as kept. A converting server answers 4.0; a 3.0 card read that way is
  written back as 4.0 after an edit — the format changes, the data does not. A
  server that refuses 4.0 is asked again without a version.

### Included

- `dav`: `CardQuerier`, `CardQuery`, `CardMatch`, `VCard4` — an
  addressbook-query of its own whose `address-data` names content type and
  version, which go-webdav cannot. It reads cards escaped or in CDATA and the
  modification time. `Transport` is unchanged.
- `card`: `Query`, `Search` and `FindUID` go through it; a refusal of 400, 403,
  409, 412, 415, 422 or 501 is asked again without a version.
- Tests: `TestQueryCardsNamesTheVersion`, `TestCardReadsAskForVCard4`,
  `TestCardReadFallsBackWhenVersionIsRefused`.
- Live (S-30.2): `TestBaikalVCard4Edit`, `TestRadicaleVCard4Edit` — a 4.0 card
  edited through Carrel stays 4.0 with `KIND` and `GENDER`;
  `TestBaikalPhoto`, `TestRadicalePhoto` — a saved photo is one unfolded
  PHOTO line of hundreds of KiB, a data URI in 4.0, and decodes to the framed
  JPEG. Baikal green; Radicale in the Linux gates.
- Live tests seed and clean up with a fresh client: Apache closes an idle
  connection after 5 s, and a PUT down it fails with EOF.

## v0.0.59 — 2026-09-14

A contact search works on Radicale before 3.7.7. It no longer looks in `N`;
`FN` carries the same name.

### For consumers

- **Radicale.** Versions before 3.7.7 (released 2026-07-19; Debian stable
  ships 3.5.3, oldstable 3.1.8) answer 500 to a text-match on a structured
  vCard property. Every contact search on them failed with an error source
  since `v0.0.47`.
- **What is searched.** `FN`, `NICKNAME`, `EMAIL`, `TEL`, `ORG`. A card whose
  family or given name appears only in `N` and not in `FN` is not found by it;
  vCard 3.0 and 4.0 require `FN`.

### Included

- `card`: `N` left `searchProps`.
- Test: `TestSearchSendsNoTextMatchOnN`. `TestRadicaleSearch` fails on
  Radicale 3.5.4 before this version and passes after it.

## v0.0.58 — 2026-09-14

S-22: a record's source block and a collection row on the connection card
say when the collection was last read and when it last changed on the server.
Neither costs a request.

### For consumers

- **`lastRead`.** When this session last read a list of the collection:
  "just now", "2 min ago", later the date and time. Empty until a list of it
  was read in this session. A record page alone is not a read of the
  collection; neither is a search.
- **`changed`.** The newest modification time the server sent
  (`getlastmodified`) among the records of a whole read, as date, year and
  time: "14 September 2026, 11:02". Empty when no whole list was read or the
  server sent no times. The agenda reads one week and fills `lastRead` only,
  the same rule as the rail's count; a task or note list of the same calendar
  fills `changed`.
- **Combined views.** A finished poll counts as a read of each collection
  that answered from the network, at the time the poll ended.
- **Where.** `SourceBlock` on contact, event, task and note pages;
  `CollectionRow` on `/app/settings/connections`. `objects` in both is still
  empty.

### Included

- `model.Object.Modified`: the reply's `getlastmodified`; `cal` and `card`
  fill it from the REPORT. It is never written back.
- `session.Reading`, `NoteReading`, `ReadingOf`: kept in the session, not in
  the list index, whose `CARREL_CACHE_COLLECTION_TTL` would drop it after a
  minute. Reads of several kinds fold: the later read and the newer change
  win. The session's end wipes them.
- Tests: `TestReadingFoldsReads`, `TestCollectionStateFromWhatWasRead`.

## v0.0.57 — 2026-09-14

Search ignores letter case in every script. "ковал" finds "Встреча с
Ковалёвой" in events, tasks and notes, not only in contacts.

### For consumers

- **Calendars.** A search asks the calendar server for `i;unicode-casemap` by
  name. Until now it went with CalDAV's default, `i;ascii-casemap`, which
  ignores case in Latin only: Baikal found Cyrillic in the case it was written
  in. Contacts already went with CardDAV's default, `i;unicode-casemap`.
- **A server that refuses it.** The same search is asked again with the
  server's default. The source answers with the note "the server ignores
  letter case in Latin script only".
- **One collection, several kinds.** A note on one kind (the file ceiling, the
  case note) no longer stops the other kinds of the same collection from being
  searched.

### Included

- `dav`: `CalSearcher` and `CalSearch`, a calendar-query REPORT of its own
  whose text-match names a collation; `UnicodeCaseMap`. It reads bodies escaped
  or in CDATA; a body that does not parse stays a record that could not be
  read. `Transport` is unchanged.
- `cal`: `Search` uses it; a refusal of 400, 403, 412, 415, 422 or 501 falls
  back to go-webdav's query and returns `ErrLatinCaseOnly` with the records.
- `web`: the search source turns `ErrLatinCaseOnly` into the source note.
- `davtest`: honours `i;unicode-casemap` (go-webdav's own text-match counts
  case); `RefuseCollation` answers 400 the way sabre/dav does.
- Tests: `TestSearchCalendarNamesItsCollation`,
  `TestSearchIgnoresCaseInEveryScript`,
  `TestSearchFallsBackWhenCollationIsRefused`, `TestSearchNotesLatinOnlyCase`.
  `TestRadicaleSearch` and `TestBaikalSearch` now require "ковал" to find both;
  not run in this version.
- `TestRequestTimeoutCountsSilenceOnStreams` (`v0.0.56`) has wider steps: under
  a loaded run 80 ms against a 300 ms timeout read as silence.

## v0.0.56 — 2026-09-14

`CARREL_DAV_REQUEST_TIMEOUT` no longer cuts a file that is still moving. On
a download, an upload and a copy it counts silence: the transfer is cut after
that many seconds without a byte either way. A record request keeps one
deadline for all of it, body included.

### For consumers

- **Files.** A download, an upload or a copy between storages lasts as long
  as bytes keep coming; a server that stops answering mid-file is cut after
  `CARREL_DAV_REQUEST_TIMEOUT` (30 s) of silence. The known limit of `v0.0.53`
  is gone.
- **Everything else.** PROPFIND, REPORT, discovery, records and photos: the
  same 30 s for the whole request as before.
- **Failure.** A cut request fails with a timeout, as before: a source in a
  search is `timeout`, not `error`.

### Included

- `dav`: the guarded client has no `http.Client.Timeout`; its transport holds
  a watchdog per request. `Stream(ctx)` marks the requests that carry a file;
  `Open` marks its own. `ErrTimeout` is `context.DeadlineExceeded` to
  `errors.Is` and a `net.Error` with `Timeout()`.
- `file`: `UploadCreate` and `UploadOverwrite` mark their context, so `Copy`
  and `CopyTree` stream both ways.
- Test: `TestRequestTimeoutCountsSilenceOnStreams` — a file moving for longer
  than the timeout passes both ways; a stalled one is cut; a record GET and
  PUT past it are cut.

## v0.0.55 — 2026-09-14

`docs/spec/check.py`, check 8 (a fixture draws into its frame), reads the
rail under the `tc-*` class names. It had been red since `v0.0.51`: it still
looked for `m-src`, `m-meta`, `m-nav`, `m-num` and found an empty rail in
every frame.

### For consumers

Nothing in the running server. `python docs/spec/check.py` is green again; a
red check 8 now means the fixture and the mockup frame really disagree.

## v0.0.54 — 2026-09-14

Copies the Typecase mockup through plan 1 step 4: frames use the same tags
the widgets print (`button`, `input`/`textarea`, `a href`, `kbd`); menus
that sat inside a button become `.tc-drop`.

### For consumers

Nothing in the running server. Open
[carrel-ui-mockups.html](../../docs/visual/carrel-ui-mockups.html) for the
current mockup; fonts are in `docs/visual/fonts/`.

## v0.0.53 — 2026-09-14

S-17 in [server.md](../../docs/plan/server.md): files. The C-5.5 routes that
were stubs answer: the root of the section, a folder, download, upload, new
folder and the operations, with the transfer queue that asks about taken
names.

### For consumers

- **Root.** `GET /app/files` is `FilesRootPage`: the ticked file collections
  as rows (a gone one stays, `state: gone`, without `href`), the shortcuts
  `files.attachments` and `files.published`, and `empty` when nothing shows —
  "No file storage is connected" (with `account.connect`) or "No storage is
  ticked".
- **Rail.** The files rail has `nav.files` on, a first row "Selected
  collections" whose tick is off, part or on, and the file collections by
  connection. Their `href` is the collection root and ends in `/`; no count, no
  menu. `POST /app/sources/files` with `tick` = those hrefs replaces the file
  ticks, as for the record kinds; a fragment answers `rail` alone.
- **Folder.** `GET /app/files/{account}/{col}/{path…}` is `FilesPage`. A
  folder address ends in `/`; each segment is decoded on its own, and an
  empty, `.` or `..` segment is 404.
  - **Rows.** Folders first. A folder's `href` is its address and `type`
    "folder". A file's `href` is its download, `type` its media type, and
    `thumb` its download with `?inline=1` when it is PNG, JPEG, GIF or WebP of
    at most 512 KiB; otherwise empty.
  - **Menu.** `file.download` (files), `file.rename`, `file.move`,
    `file.copy`, `file.delete`. On a read-only storage everything except
    download and copy is disabled with a reason.
  - **Bar.** `file.upload`, `folder.new`, and over a selection `file.download`
    (one file), `file.rename` (one item), `file.move`, `file.copy`,
    `file.delete`. `segments` are `files.view.list`, `files.view.tiles`,
    `files.sort.name`, `files.sort.size`, `files.sort.changed`.
  - **Query.** `filter`, `sort`, `view` and `select` (repeatable) cut the
    listing this session read last; the plain address reads the server.
  - **A file's address** without the trailing slash — what search results link
    to — is its folder with the file selected. A folder's address without the
    slash redirects to it.
  - **Other fields.** `crumbs` lead to the root, the current one without
    `href`. `queue` is the transfer queue, or null. Over
    `CARREL_FILES_MAX_ENTRIES` rows the page says how many it shows.
  - A server that does not answer is a notice with `source.retry`, not 500.
- **Download.** `GET …/download/{path…}` streams the file with no ceiling of
  Carrel's. The header `Range: bytes=a-b` or `a-` is passed on, and a 206
  comes back with `Content-Range`. Every response carries
  `Content-Disposition: attachment` with the file name,
  `Content-Security-Policy: sandbox; default-src 'none'`, `nosniff`,
  `Cache-Control: private, no-cache`, and the server's `ETag`,
  `Last-Modified` and `Content-Length`.
  - Only `?inline=1` on the four raster types is `inline`.
  - A missing file is 404; a refusal is the folder page with a notice.
  - An address after `download/` that ends in `/` is the folder `download`
    at the root of the collection.
- **Upload.** `POST …/upload/{folder}/`, `multipart/form-data`, one or more
  file parts named `file`. The body has no cap.
  - **Token.** `_csrf` must come before the first file (the fields in front of
    it are capped at 1 MiB), or go in `X-CSRF-Token`. A file part before the
    token is 403, and nothing is written.
  - **A free name** goes straight to the server, streamed.
  - **A taken name** is never overwritten, also on servers that ignore
    `If-None-Match`. The row waits with a question, and its file is kept in
    the temporary directory until the answer or the end of the session.
  - **The answer.** A fragment answers target `queue` with `TransferQueue`; a
    plain form redirects to the folder.
- **Queue.** Rows have `state` `done`, `failed`, `cancelled` (skipped) or
  `idle` (asking).
  - **Question.** An asking row carries `question` with `transfer.skip`
    "Skip", `transfer.both` "Keep both" and `transfer.overwrite` "Overwrite
    this one" (danger). Each is a POST to the same upload or op address with
    `transfer` and `answer`.
  - **Answers.** Keep both takes the first free `name-2.ext`, `name-3.ext`, …;
    Overwrite is the only write without a precondition. A row answers once
    and in its own session; otherwise 404.
  - **Shown once.** Asking rows stay; a finished row appears once, in the
    first queue shown after it finished. `note` summarises the rows and says
    why each failed one failed.
- **New folder.** `POST …/mkcol/{folder}/` with the field `name`. Success
  redirects, or answers `main` with the folder. A taken name ("… is already
  here"), a bad name or a read-only storage is the folder with a notice.
- **Operations.** `POST …/op/{folder}/?op=rename|move|copy|delete&item=…`
  answers `FileOpSheet` (`sheet`). Its first action runs the operation, with
  `go=1` in its address; the second, `file.op.cancel`, returns to the folder.
  - **Rename.** Takes one item and the form field `name`.
  - **Move and copy.** `target` is every live file collection as a root,
    read-only ones marked, opened along the chosen folder. A form field or
    query `to` = a node's `id` chooses it; the folder of the address is the
    default.
    - **Refused before it starts.** Moving into the same folder, a folder into
      itself, onto a read-only storage, and moving out of one.
    - **Across connections.** Another connection has `warning` said before it
      starts. The files go GET → PUT and nothing moves on a server; a move
      deletes the original only after the whole copy arrived.
    - **One connection.** Moves and renames are MOVE, without traffic.
  - **Delete.** `warning` says Carrel keeps no index of what points at a file.
    The run sends the versions the sheet read, so a file changed since stays:
    "0 of 1 item deleted".
  - **Reports.** Rename, move and copy report in the queue, a taken name as a
    question. Delete reports in notices. A plain form redirects when there is
    nothing to say.
- **Known limits.** `CARREL_DAV_REQUEST_TIMEOUT` still bounds a whole DAV
  request, body included, so a download or upload that takes longer is cut.
  A folder copy stops at `CARREL_FILES_MAX_ENTRIES` entries and says so.

### Included

- `dav`:
  - `Streamer` and `Client.Open` return a `Download` without the response cap;
    `Get` keeps the cap.
  - `Sized` gives a PUT body its length.
  - `MkCol` and `Move` send their own requests and refuse with a
    `StatusError`, and a 207 from MOVE or DELETE is a refusal. `Move` sends
    `Overwrite: F` or `T`.
- `file`:
  - New `Open`, `Stat`, `NotFound`, `MkCol`, `Move`, `Delete` (with the
    version as If-Match), `UploadOverwrite`, `Copy`, `CopyTree`, `Suffixed`
    and `CheckName`.
  - `Entry.Type`.
  - `List` drops hrefs outside the folder or deeper in it.
- `session`: `Transfer`, `KeepTransfer`, `TakeTransfer`, `ReadTransfers`. The
  end of a session discards every row.
- `conf`: `FileThumbMaxBytes` = 512 KiB, `TransferQueueLen` = 50.
- `user`: `NewTransferID` (`q-`).
- `davtest`:
  - `FilesIgnoreIfNoneMatch` (SFTPGo's PUT), `File` and `SeedFolder`.
  - A DELETE under `FileRoot` is checked by the file server.
- `web`:
  - `files.go`, `files_ops.go`.
  - The rail's section rows come from `navRows`.
  - The upload body is read as a stream in the middleware.
- Tests:
  - `dav`: `TestOpenPassesTheWholeFile`, `TestSizedBodyIsNotChunked`,
    `TestMoveMkColAndDeleteRefusals`.
  - `file`: `TestFileOperations`, `TestListKeepsTheFolderOnly`,
    `TestGetIsNotCapped`.
  - `session`: `TestTransferQueue`.
  - `web`: seven tests in `files_test.go`.
  - `integration/files_live_test.go` (`TestSFTPGoFiles`), not run: the live
    servers did not answer on 14.09.2026.

## v0.0.52 — 2026-09-13

S-13 in [server.md](../../docs/plan/server.md): contact photos. One route
serves them; the edit page and the frame screen upload, frame, save and
remove them. A photo a card only links to is shown through Carrel and is not
edited.

### For consumers

- **Photo route.** `GET /c/{account}/{col}/{uid}/photo` streams the photo the
  card keeps, typed as the card declares it: `TYPE=JPEG`,
  `MEDIATYPE=image/jpeg` and a `data:image/jpeg` URI are all `image/jpeg`;
  likewise PNG, GIF and WebP. Any other declared type, no type, or bytes that
  are not the declared format answer 404: nothing is guessed. Responses carry
  `X-Content-Type-Options: nosniff` and `Cache-Control: private, no-cache`.
  - **`?size=thumb`** is a JPEG square of `CARREL_PHOTO_THUMB_SIDE`, from the
    centre.
  - **`?v=`** is the version (ETag) of the card the link was made from. The
    response's `ETag` is that version: `"<etag>"`, or `"<etag>-thumb"` for the
    thumbnail. `If-None-Match` answers 304.
  - **`?preview=`** is the frame of the draft being chosen (below).
  - **Linked photos.** A photo the card links to (`VALUE=uri`, or any URI in
    vCard 4.0) is fetched through the DAV guard when the route is asked for
    it. Only http and https, only an `image/*` reply in one of the four
    formats, within `CARREL_DAV_MAX_RESPONSE_BYTES` and
    `CARREL_PHOTO_MAX_PIXELS`. A failed fetch is 404.
- **Avatars.** Contact rows (one collection and the combined view) and the
  contact card fill `avatar.href` with the thumbnail address. It stays empty
  when the card has no photo, or one that cannot be shown. The list makes the
  thumbnails from the cards it has already read, so showing them asks the DAV
  server nothing. `photoHref` of the contact card is the photo itself.
- **Edit page.** `photo` of `ContactEditPage`:
  - **No photo:** `editable: true`, `note` "JPEG, PNG, GIF or WebP.", action
    `photo.add` "Add".
  - **A kept photo:** `href`, and actions `photo.replace` "Replace",
    `photo.crop` "Crop", `photo.remove` "Remove" (danger).
  - **A kept photo that cannot be shown:** a `note`, `photo.replace` and
    `photo.remove`.
  - **A linked photo:** `href`, `note` "External photo link; editing is not
    available.", `editable: false`, no actions.

  Every action is a POST to `photo-preview` with `op` and the form's `etag`
  in the query. `photo.add` and `photo.replace` send `multipart/form-data`
  with the file in the field `photo`: `_csrf` goes in the body before the
  file, or in `X-CSRF-Token`.
- **Upload.** The body of
  `POST /app/contacts/{account}/{col}/{uid}/photo-preview` may be
  `CARREL_PHOTO_MAX_UPLOAD_BYTES` plus 1 MiB; other POSTs keep 1 MiB. A body
  over that cap with `_csrf` in it is 413 before the handler.
  - **Nothing kept.** The file goes to a temporary file, removed whatever
    happens. The dimensions are checked before anything is decoded. Nothing is
    written to the card before Save photo.
  - **Refusals.** A refusal is the edit page with a notice: "No photo was
    chosen", "The photo is too large", "The photo has too many pixels", or
    "This file is not a photo Carrel can use".
- **Frame.** `GET .../photo-preview` is `PhotoPreviewPage` for this card's
  draft; without a draft it redirects to the edit page. The title is "Photo"
  and the rail is "Back to the card". `href` is the preview: a JPEG of
  `CARREL_PHOTO_MAX_SIDE`, never enlarged past the crop.
  - **`zoom`**: percent of the largest square that fits, 100 to 400.
  - **`offsetX`, `offsetY`**: percent of the room left on each side, from
    -100 (left, top) to 100; 0 is the centre.
  - **`rotate`**: 0, 90, 180 or 270, clockwise, applied after the EXIF
    orientation.
  - **Actions.** `photo.rotate.0` to `photo.rotate.270` ("0°" to "270°"; the
    one shown is disabled), `photo.save` "Save photo", `photo.cancel`
    "Cancel", and `photo.remove` "Remove photo" when the card keeps a photo.
  - **Fields.** A turn and Save photo read `zoom`, `offsetX`, `offsetY` and
    `rotate` from the form body. A turn's own `rotate` travels in its address
    and wins. Values out of range are clamped.
- **Save photo.** One patch on PHOTO, with `If-Match` of the version the photo
  was chosen for, then a redirect to the edit page (or the edit page with a
  loss report).
  - **What is written.** A JPEG at `CARREL_PHOTO_JPEG_QUALITY`, without any of
    the upload's metadata, in the card's own version:
    `PHOTO:data:image/jpeg;base64,…` in 4.0, `PHOTO;ENCODING=b;TYPE=JPEG:…`
    in 3.0, `ENCODING=BASE64` in 2.1.
  - **A card changed since.** The preview page answers with "Changed
    elsewhere, so the photo was not saved". The draft now holds the new
    version, and Save photo writes over it only when pressed again.
- **Remove.** Deletes the PHOTO property (not an empty value), with `If-Match`
  of the version the page showed. A card changed since is the edit page with
  "Changed elsewhere, so the photo was not removed". Upload, Crop and Remove
  on a linked photo answer 404.
- **Other clients' photos.** A vCard 4.0 `data:` URI in PHOTO, LOGO, SOUND or
  KEY used to go back to the server with its comma escaped (`base64\,`) on
  any edit of the card. It now goes back as it came.

### Included

- `photo`: new package. `Load`, `Open`, `Render`, `Fetch`, `MediaType` and
  `Crop`. The orientation is EXIF tag 0x0112 from APP1, read without an EXIF
  library. Large sources are halved before Catmull-Rom; transparency lands on
  white.
- Dependencies: `golang.org/x/image` v0.46.0 (webp, draw); `golang.org/x/sys`
  v0.48.0 comes with it.
- `conf`: `PhotoMaxZoom` = 400.
- `card`:
  - `PhotoURI` and `PhotoOpen` read vCard 4.0 `data:` URIs and links without
    `VALUE=uri`.
  - `PhotoOpen` returns the declared type as written: empty when there is
    none, no longer `application/octet-stream`.
  - New `PhotoOps`; `Encode` keeps the `data:` URI comma.
- `web`:
  - `photo.go`, and `bodyLimit` in the middleware.
  - `contactGroups` takes the session; `finishWrite` takes where to go when
    nothing was lost.
  - Both photo sizes live in the session's thumbnail cache, keyed by card
    version.
- `davtest`: a PUT is stored as sent unless `Drop` is set. Re-encoding with
  go-vcard had hidden what Carrel really sends.
- Tests: `internal/photo`; `TestPhotoForms`,
  `TestEncodeKeepsTheDataURIComma`, `TestPhotoOpsFollowTheVersion`;
  `TestPhotoRouteServesWhatTheCardKeeps`, `TestPhotoUploadFrameAndSave`,
  `TestPhotoUploadCaps`, `TestPhotoRefusalsLeaveNothing`,
  `TestPhotoSaveOnAMovedCard`, `TestVCard3PhotoCropAndRemove`,
  `TestLinkedPhotoIsProxiedAndNotEdited`.

### Not yet included

- The frame mockup's "Saved as 512 × 512 · JPEG · about 90 KB" line has no
  field in `PhotoPreviewPage`.
- Not checked against Radicale and Baikal: how they keep a 4.0 `data:` URI
  and an unfolded PHOTO line of a hundred kilobytes.
- vCard property groups (`item1.EMAIL`) are dropped when a card is read, so
  any edit loses them. This predates S-13.

## v0.0.51 — 2026-09-13

Copies the Typecase mockup through plan 1 step 3: classes inside frames
are the Typecase dictionary (`tc-*`, states `is-*`); desktop window chrome
is page classes `mk-*`.

### For consumers

Nothing in the running server. Open
[carrel-ui-mockups.html](../../docs/visual/carrel-ui-mockups.html) for the
current mockup; fonts are in `docs/visual/fonts/`.

## v0.0.50 — 2026-09-13

The rest of S-14 in [server.md](../../docs/plan/server.md): the back rail on
record screens, the category filter of lists, and the print header.

### For consumers

- **Back rail.** The `rail` of a record (`/app/{kind}/{account}/{col}/{uid}`),
  its edit, a new record, the import screen and the conflict screen is now the
  back rail of C-6.3. `nav` holds one row with `id` `nav.back` and icon
  `i-back`: "Back to list" to the collection's list ("Back to calendar" for
  events), or "Back to the card" on the edit of an existing record. `sections`
  are the collections of the kind as on the list, with ticks and counts, but
  without the combined row. `tags` and `foot` are `null`. The way back leads
  to the collection's own list, even for a record opened from the combined
  view.
- **Tag filter.** The list of one collection and the combined view fill
  `rail.tags`. `all` comes first, then every category of the records read,
  most used first. Each `href` is the same list with `?tag=`.
  - **Matching.** `?tag=` compares without case and cuts what was already
    read; no server is asked for it.
  - **Counts.** The rail's counts and `bar.total` stay the whole collection,
    and `bar.loaded` counts the rows shown. On a contacts list `bar.loaded`
    used to equal `bar.total`; without a tag it still does.
  - **Unknown or empty.** A tag in the address that no record has stays in
    `tags` with `on: true`, so it can be cleared. With no categories and no
    tag, `tags` is `null`.
  - **Combined view.** Tag links carry `task=`, so a category is chosen from
    the finished poll without polling again. `top.refresh` and `poll.again`
    of a filtered view keep `tag`.
  - **Kept by other links.** The week presets of the agenda and the task
    segments keep `tag`.
  - **Tags elsewhere.** The `tags` of task rows, note rows, the contact card
    and the note's meta now have an `href` to the filter of their collection,
    and `on` for the category shown.
- **Print header.** `print` of a list:
  - **`title`** is the page title.
  - **`range`** joins with " · " what the list shows, in this order: the
    number of collections in a combined view ("2 books"), the agenda week
    ("7 September 2026 – 13 September 2026"), the task segment ("Open"), the
    number of rows ("3 cards"; in a combined view only once the poll is
    over), and the tag.
  - **`taken`** is absolute with the year: when the list was read, or when
    the combined poll finished.
  - **Nothing ticked:** a combined view with no ticked collections has only
    `title`.

  On a record page, `title` is the record's title, `range` is its stamp
  (`account · collection`), and `taken` is when it was read. Edit, new and
  conflict screens leave `print` empty.

### Included

- `web`: `frame.go` holds the back row, `tagFilter`, `categoriesOf`, the
  print time (`timeFmt.taken`) and `printRange`. `kindRail` is split into
  `railSections`, shared by the sections rail and `backRail`. `taskGroups`,
  `noteGroups`, `agendaRange` and `taskSegments` take the filter.
- Tests: `TestBackRailOnRecordScreens`, `TestTagFilterOnContacts`,
  `TestTagFilterOnCalendars`, `TestCombinedTagFilterReadsThePoll`;
  `TestConflictScreenAndCombination` checks the conflict screen's rail.

### Not yet included

- Stop and Retry of a filtered combined view, sent without scripts, return to
  the view without its tag.
- Events have no count in the rail: the agenda reads one week, not the whole
  calendar.
- Records a combined REPORT returned unreadable are left out without a notice;
  the collection's own list names them.
- S-22: `LastRead` and `Changed` of a collection are not filled yet.

## v0.0.49 — 2026-09-13

Two tails of S-14 in [server.md](../../docs/plan/server.md), closed before
S-13: "New" from the combined view without scripts, and the rail's count after
a write.

### For consumers

- **New without scripts.** `GET /app/{kind}/{account}/{col}/new?pick=1`
  without the fragment header now answers the `NewSheet` as the page, the same
  body the sheet fragment has: `target.options` with `chosen` empty and
  `record.new` disabled. Follow an option's `id` to its new-record form. Before,
  that address opened the form of the collection in its path, which the
  combined view only names because the route needs one. Without `pick` the page
  is still the empty form of that collection.
- **Counts after a write.** A save, a new record, a conflict resolved and a
  delete now change the rail's `count` of that collection by what was written,
  instead of turning it into `null` until the list is read again (C-6.3). A
  list that fails to read afterwards still shows that count. Links from
  RELATED-TO to the written record keep its new title.

### Included

- `web`: `collectionCtx.noteWrite` replaces `dropIndex`: the written record
  replaces its entry in the loaded list, a new one is added, a deleted one
  leaves; the other entries are as the last read left them.
- Tests: `TestCountSurvivesWrites`; `TestNewFromCombinedChoosesTheTarget`
  reads the page without the fragment header.

### Not yet included

- The rail on record, edit, import and conflict pages ("Back to list"), the
  tag filter, and `print` are not filled.
- Events have no count in the rail: the agenda reads one week, not the whole
  calendar.
- Records a combined REPORT returned unreadable are left out without a notice;
  the collection's own list names them.
- S-22: `LastRead` and `Changed` of a collection are not filled yet.

## v0.0.48 — 2026-09-13

S-14 in [server.md](../../docs/plan/server.md): the combined view. Each
record kind shows the collections ticked for it in one list, from any number
of servers, and the ticks are kept in the account's sealed settings.

### For consumers

- **Ticks** (`POST /app/sources/{kind}`, `kind` one of `contacts`,
  `calendar`, `tasks`, `notes`). The rail is a form: send the `href` of every
  ticked row as a repeated `tick` field, exactly as the rail gave it. The form
  replaces the ticks of that kind only; the other kinds keep theirs. An empty
  form unticks everything. A row that is not a collection of that kind in the
  catalog is ignored, and so is a gone collection that was not ticked already.
  Without the fragment header the answer redirects to `/app/{kind}`; with it,
  the answer is the `rail` and, out of band, the `poll` of the new selection.
  `files` answers 404 (the files root is S-17).
- **Combined view** (`/app/{kind}`) has the same view model as one collection,
  plus `poll` and, on every row, `stamp` (`host · collection`) and `colour`.
  Rows are merged, not grouped by source: contacts by name, events and tasks
  by time, notes newest first. `href` of a row is the record in its own
  collection, so editing always happens there. The agenda takes `?from=` like
  a single calendar; tasks take `?show=`.
- **Nothing ticked** is an empty screen: `poll.taskID` is empty,
  `poll.summary` says nothing is ticked, and no server is asked.
- **Poll and reload.** The page answers at once and the sources fill in the
  background, as in search: follow `GET /app/search/progress/{task}` (or
  `…/poll`) and read the page again. `?task={id}` reopens that result without
  asking again; a plain `GET /app/{kind}` asks again once the previous poll is
  over, and reuses it while it is still running. Stop and Retry work the same as
  in search; without the fragment header they return to `/app/{kind}?task={id}`.
  `poll.again` is "Refresh", a GET on the view's own address.
- **Refresh** (`top.refresh`): `action` is `poll.again` (GET on the combined
  view, disabled while the poll runs), `answered` is when it ended, `running`
  says whether it is still going. The mockups show Refresh as a POST to
  `/app/sources/{kind}`; the server does not take that.
- **Rail** on the combined view and on a single collection: `nav` has the four
  kinds with `count` (the sum of the ticked collections, `null` while one has
  none), then Files and Duplicates without a count. The first section holds
  one row, the combined view, with `tick` `off`, `on` or `part`. Then one section
  per connection (`account` is the host) with the collections of the kind:
  `href` to the collection, `tick`, `count` (whole collection, only when ticked
  and read in this session), `state` `ok`, `read-only`, `unavailable` (the
  server did not answer this poll) or `gone`. A gone collection stays in
  the rail while ticked, without `href`, and is not asked. `menu` has
  `collection.rename`. `foot`: `rail.apply` (POST `/app/sources/{kind}`),
  `collection.new`, `account.connect`. `note`: "2 of 3 collections ticked".
- **New from the combined view.** `bar.actions` has `record.new` pointing at
  `…/{account}/{col}/new?pick=1`. As a sheet it lists every collection of the
  kind as `target.options` (read-only ones with `readOnly` and `reason`),
  `chosen` empty, and `record.new` disabled until a target is chosen. **An
  option's `id` is now the address of its new-record page**; open it to
  continue. Opened from one collection, that collection is `chosen`.

### Included

- `user.SetSelection` (journal `sources.select`, detail the kind).
- `web`: `combined.go` — the combined view, its `listTask`, the rail,
  `postSources`. List rows are built by the same code for one collection and
  for the combined view (`contactGroups`, `agendaDays`, `taskGroups`,
  `noteGroups`). The poll bar takes the owner of the task, so search and lists
  share the progress routes.
- Tests: `TestCombinedContactsFollowTheTicks`,
  `TestCombinedAgendaTasksAndNotesMerge`, `TestCombinedGoneIsNotUnavailable`,
  `TestNewFromCombinedChoosesTheTarget`.

### Not yet included

- Without scripts, "New" from the combined view opens the form of the first
  writable collection: the choice of target is the sheet, and a sheet needs the
  fragment header.
- Events have no count in the rail: the agenda reads one week, not the whole
  calendar.
- A count goes back to a dash after a write in that collection until the list
  is read again.
- The rail on record, edit, import and conflict pages ("Back to list"), the
  tag filter, and `print` are not filled.
- Records a combined REPORT returned unreadable are left out without a notice;
  the collection's own list names them.
- S-22: `LastRead` and `Changed` of a collection are not filled yet.

## v0.0.47 — 2026-09-13

S-15 in [server.md](../../docs/plan/server.md): search across the ticked
collections, run as a fanout task of the session. Passes against live Baikal
(`TestBaikalSearch`).

### For consumers

- **Search** (`/app/search?q=…`, `SearchPage`) answers at once: `poll` lists
  every source as `waiting` or `querying`, and reading the page again shows
  what has arrived. The sources are the collections ticked in `Selected`, each
  asked for every kind it is ticked for; a gone collection is not asked.
  Nothing ticked: no task, and `poll.summary` says so. An empty `q` searches
  nothing and writes nothing.
- **Results** (`results`, `FindRow`) come in kind order — contacts by name,
  events and tasks by time, notes newest first, files by name — each with
  `stamp` (`host · collection`), `colour`, `href` (the record route; a file's
  is `/app/files/{account}/{col}/{path}`), `detail` and `when`. `note` says
  files are searched by name only whenever a file collection was asked.
- **Segments and filter** (`bar.segments` with ID `search.kind`, `?kind=`,
  `?filter=`) cut what has arrived and ask no server. `bar.total` counts all.
- **Poll bar** (`PollBar`): `taskID`; `summary` ("3 of 3 sources answered ·
  7 found · 1 refused"); `percent`; `running`; `answeredAt` once it is over.
  `sources[]` carry `state` (C-8.1), `count` when answered, and `message` with
  `retry` (`source.retry`, POST) for `error`, `timeout`, `unavailable` and
  `cancelled`. `actions`: `poll.stop` (POST, while running) and `poll.again`
  (GET with `again=1`, once over).
- **Back to a result**: the same `q` in the same session opens its task from
  memory — no server is asked, `answeredAt` says when it was collected;
  `again=1` asks anew. A new query stops a running search; what it found stays.
- **Progress routes.** `GET /app/search/progress/{task}` is SSE: `: ping` at
  once, `event: poll` with `{"target":"poll","body":PollBar}` on every change,
  `event: poll-finished` at the end; closing the stream while the task runs
  cancels it. `…/poll` (fragment) is the fallback; with
  `CARREL_PROGRESS_MODE=poll` the stream is 404. `POST …/retry?source={id}`
  asks that source only, `POST …/cancel` stops; both answer the bar as a
  fragment, or redirect to the result without the fragment header. A task of
  another session is 404.
- **History** (`top.search.history`, `SearchHistoryRow`): up to ten, newest
  first, written when a poll ends. `outcome` is "7 found", "nothing found"
  (a source answered) or "no source answered"; a repeat updates its row;
  `href` reopens the result. **Hints** (`top.search.jump`) on the result page:
  collection names and records of lists this session already loaded, no server
  asked.
- **Too many searches**: past four live fanouts nothing starts and the page
  carries a notice; a retry without a free slot says so in that source's
  `message`.

### For operators

- `CARREL_PROGRESS_MODE`, `CARREL_PROGRESS_SOURCE_TIMEOUT` and
  `CARREL_PROGRESS_TOTAL_TIMEOUT` take effect. A proxy in front must not buffer
  `text/event-stream` (the stream sends `X-Accel-Buffering: no`); otherwise
  set `CARREL_PROGRESS_MODE=poll`.
- The DAV server does the matching. On Baikal, lower-case Cyrillic finds
  contacts but not events, tasks or notes: its CardDAV ignores Unicode case,
  its CalDAV only ASCII case.
- A file collection is walked with one PROPFIND a folder, up to
  `CARREL_FILES_MAX_ENTRIES` entries; past that the source answers with a note.
  No file body is read.

### Included

- `fanout`: `Query`, `Item`, `Source`, `Task` (`Start`, `Snapshot`,
  `Changed`, `Retry`, `Cancel`) over a `golang.org/x/sync` v0.23.0 semaphore.
  Answers are merged in order; a source that ignores its context still ends at
  the ceiling, and its late answer is dropped.
- `session`: kept tasks (`KeepTask`, `Task`, `Tasks`), at most
  `conf.MaxKeptFanouts` (10), cancelled when the session ends;
  `HistoryEntry.Hits` and `None`; `AddHistory(HistoryEntry)`.
- `provider`: `card.Search` (one REPORT, any of FN, N, NICKNAME, EMAIL, TEL,
  ORG), `cal.Search` (a REPORT per property: SUMMARY, LOCATION, DESCRIPTION
  for events; SUMMARY, DESCRIPTION for tasks and notes), `file.List`
  (PROPFIND depth 1, for S-17 too).
- `web`: `search.go`, `Renderer.Event`; `user.NewTaskID` (`t-`).
- `davtest`: `FileRoot` over go-webdav's WebDAV handler, `SeedFile`, `Accept`,
  `Stall`.
- Tests: `TestItemsArriveInPlace`, `TestStatesTellEmptyFromRefusal`,
  `TestSourceTimeout`, `TestParallelismIsACeiling`,
  `TestCancelKeepsWhatArrived`, `TestCeilingEndsASourceThatNeverReturns`,
  `TestRetryAsksOnlyThatSource`, `TestBusyAsksNobody`,
  `TestNoSourcesEndsAtOnce`, `TestHistoryRepeatUpdatesItsRow`,
  `TestKeptTasksAreBoundedAndEndWithTheSession`,
  `TestSearchLooksInShownPropertiesWithOneReport`,
  `TestSearchIsAnyOfShownProperties`, `TestListReadsOneFolder`,
  `TestSearchAcrossTickedSources`, `TestSearchRefusalRetriesOneSource`,
  `TestSearchStopAndCeiling`, `TestSearchStartsOnlyWithTextTicksAndASlot`,
  `TestSearchProgressStream`; live `TestBaikalSearch`, `TestRadicaleSearch`.

### Not yet included

- Ticks: `POST /app/sources/{kind}` and the rail are S-14. Until then nothing
  writes `Selected`, and search says nothing is ticked.
- `TestRadicaleSearch` has not run: Radicale runs in the Linux gates
  (`scripts/linux-gates.sh`), as `TestRadicaleCollections` still does.
- Lower-case Cyrillic in calendars on Baikal (above): go-webdav cannot send a
  collation with text-match; the way out is not chosen yet.
- Hints while typing: C-5 has no route for them; `jump` is filled on the
  result page only.

## v0.0.46 — 2026-09-13

S-12.1–12.2 in [server.md](../../docs/plan/server.md): collections are
created and renamed from Carrel. Passes against live Baikal
(`TestBaikalCollections`).

### For consumers

- **New collection** (`/app/collections/new`, `CollectionNewSheet`). Opened
  with `?kind=contact|event|task|note` (the rail) or `?a={account}` (a
  connection card, whose `collection.new` action is new); without `kind` the
  form has a `kind` row, Calendar or Address book. Form rows: `account` (only
  servers with that home; the others are `target` options with `readOnly` and
  a `reason`, and `note` says why), `name`, `colour` (radio, the eight C-10
  tones — calendars only), `components` (check, `VEVENT` `VTODO` `VJOURNAL` —
  calendars only, the sheet's kind preselected), `address`. POST goes to the
  form's action; success redirects to the list of the new collection.
- **The address** comes from the name when `address` is empty: NFD, the
  closed Cyrillic table, lower case, runs of spaces and hyphens to one hyphen,
  `[a-z0-9-]`, empty becomes `col`; `-2`, `-3` where the server already holds
  it. "Дни рождения" is `dni-rozhdeniya`. A typed address is kept as typed and
  must be one segment of `[A-Za-z0-9._~-]`, not starting with a dot and without
  `..`; a taken one is a row `error`, not a suffix. The address never changes
  afterwards.
- **Rename** (`/app/collections/rename?a={account}&c={address}`,
  `CollectionRenameSheet`). Every non-file `CollectionRow` of a connection
  card has `menu` item `collection.rename`. Rows `name` and, for calendars,
  `colour`; `address` is the full URL, shown and not editable. One PROPPATCH
  of `displayname` (and `calendar-color`), no MOVE; success redirects to the
  connections page. A colour left unchosen stays as it is.
- **A refusal** fills the new `failure` notice of either sheet: title, what did
  not happen, and `details` with method, address, `→ code` and the server's
  body. 403 reads "This account can read here but not create" (or "… not change
  it"). The form keeps what was entered; nothing is created, renamed, added to
  the catalog or journaled. Neither door is hidden or disabled in advance: the
  right to create is not guessed.
- **C-6 change:** `CollectionNewSheet.Failure` and
  `CollectionRenameSheet.Failure` (`*Notice`, `failure`). The failure frame
  (s-profile-6) had no field to carry it.
- Journal: `collection.create` and `collection.rename`, detail the connection
  ID only.

### For operators

- A PROPPATCH answered 207 is read now: a property refused inside it fails the
  request. Before, a 207 counted as success whatever it said.

### Included

- `dav`: `Prop.Value` is the XML content of the property element, checked
  before sending; `TextValue` escapes text for it. `MkColProps("MKCALENDAR")`
  sends a CalDAV `mkcalendar` root. PROPPATCH reports the refused property's
  code (a 424 of the others does not hide it). Closes the `Prop.Value` item of
  [providers-tail.md](../../docs/plan/providers-tail.md).
- `web`: `handlers_collections.go`, `slug.go`; `golang.org/x/text` v0.42.0 for
  NFD.
- `user`: `AddCollection` (an entry marked gone at the same path becomes the
  new one), `RenameCollection`. A catalog that cannot be written after the
  server created the collection deletes it again.
- `davtest`: MKCALENDAR with name, colour and components; PROPPATCH of
  `displayname` and `calendar-color` on collections, atomic; MKCOL on an
  existing collection answers 405; `Refuse`, `Calendar`, `AddressBook`,
  `Colour`.
- Tests: `TestNewCalendarTakesItsAddressFromTheName`,
  `TestNewAddressBookWithAddressByHand`, `TestRefusedCreateLeavesNothing`,
  `TestRenameWritesTheLabelNotTheAddress`, `TestCollectionSlug`,
  `TestUniqueSlug`, `TestAddressProblem`, `TestPropValueIsXMLContent`,
  `TestPropPatchReportsRefusedProperty`; live `TestBaikalCollections`,
  `TestRadicaleCollections`.

### Not yet included

- `TestRadicaleCollections` has not run: Radicale runs in the Linux gates
  (`scripts/linux-gates.sh`).
- The rail's `collection.new` door (S-14); the card door works now.
- Move and delete of a collection (S-12.3 after S-18, S-12.4 after S-19).

## v0.0.45 — 2026-09-13

Closes the tails in front of S-12 in [server.md](../../docs/plan/server.md):
re-discovery on a connection, the connection password behind the unlock
window, and the first day of the week and 12/24 hours from the account.

### For consumers

- **Re-discovery** (S-6.1). Every `ConnectionCard` has the action
  `connection.rediscover` (POST). It answers the connections page with one
  notice: "Nothing appeared or disappeared", or "Collections changed on the
  server" naming what appeared and what is gone, the addresses in `details`,
  and two actions: `connection.rediscover.apply` (POST; its href carries
  `op=apply`, `account` and one `add` or `gone` per address) and
  `connection.rediscover.keep`. Nothing changes before Apply, and Apply writes
  only the addresses in its href that the server still confirms; the names,
  colours, rights and change tags of the other collections are refreshed with
  it. A server that does not answer changes nothing: the page carries "Could
  not connect", and that card has `state: "unavailable"` and its `trace`.
- **Gone collections** stay in the catalog and in the selection. They are no
  longer rows of their card: the page names them in a "Gone from the server"
  notice, and their routes answer 404. One that comes back is offered again
  as appeared and becomes the same entry.
- **Connection password** (S-10.7). Outside the unlock window
  `password.reveal` opens `/app/settings/unlock`, as before. Inside it the
  href is `/app/settings/connections?reveal={account}`; that GET answers
  `password.shown: true` with the value, a "Hide password" action and
  `Cache-Control: no-store`, and writes `connection.password.reveal` to the
  journal. An open window alone puts no password on the page, and another
  session of the same person has its own window.
- **Appearance** (`/app/settings/appearance`). `account` is a form with
  `weekStart` (`monday`, `sunday`, `saturday`) and `clock` (`24`, `12`); POST
  redirects back, and an unknown value comes back with the row's `error`.
  `instance` shows the time zone, read only. `browser` stays empty: theme,
  density and columns belong to the browser. Every time the server assembles
  for that person follows the setting: the agenda week and "This week", agenda
  rows, record fields, reminders, recurrence, and the `When` values of the
  unlock sheet, top bar, escrow and admin pages ("10:00 AM").

### Included

- `user`: `ApplyRediscovery` (journal `connection.rediscover`, detail
  `{account} +added -gone`, no addresses), `RevealConnectionPassword`,
  `SetTimePrefs`. `store.User.WeekStart` and `Clock12`: empty is Monday and
  24 hours, so existing state reads as before. `sealed.Collection.Gone`.
- `web`: one `timeFmt` per page instead of the loose C-10 helpers.
- `davtest.Server.RemoveCollection`.
- Tests: `TestRediscoveryOffersThenApplies`,
  `TestRediscoveryFailureChangesNothing`,
  `TestConnectionPasswordOnlyInsideUnlockWindow`,
  `TestAccountWeekStartAndClock`.

### Not yet included

- A note written by a real foreign client, read, changed and read again
  (S-11). It needs a real client; a machine run does not close it.
- The rail does not list selected collections yet (S-14); a gone one will
  carry `state: "gone"` there.

## v0.0.44 — 2026-09-13

S-11 records pass against live Radicale and Baikal
(`TestRadicaleRecords`, `TestBaikalRecords`). The run found a regression in
`v0.0.43`.

### For consumers

- **Record writes work on Radicale again.** Since `v0.0.43` every record PUT
  went out chunked, without Content-Length, and Radicale answered 400 to
  edits and new records. Baikal and go-webdav accepted it, so the httptest
  suite stayed green.

### Fixed

- `dav.Typed` no longer hides the body length: `Put` and `PutCreate` unwrap
  it before building the request. `TestPutSendsTheBodyType` also checks
  Content-Length and the absence of chunked encoding.

### Included

- `integration/records_live_test.go`: through the real request layer, on each
  live server — connect and check the catalog (colour, change tag, rights,
  kinds); contacts: list, card, edit keeping `X-CUSTOM` and `TYPE`, a write by
  another client leading to the conflict screen and its combination, delete;
  calendar: new event with a reminder, agenda, a series with a RECURRENCE-ID
  override, a new task marked done, delete. It makes its own calendar and
  address book and removes them.

### Not yet included

- A note written by a real foreign client, read, changed and read again (S-11).

## v0.0.43 — 2026-09-13

Step 2 of S-11…S-26 in [server.md](../../docs/plan/server.md): records. The
four record kinds of one collection are listed, read, edited, created and
deleted over C-5.4, and a write against a version that moved on opens the
conflict screen.

### For consumers

- **Lists** (`/app/{kind}/{account}/{col}`). `ContactsListPage` groups by
  initial. `AgendaPage` is one week in the instance zone: `?from=YYYY-MM-DD`
  picks the week and `bar.range.presets` step it; series are expanded, and an
  instance another client rewrote is shown in place of the one it replaces.
  `TasksPage` takes `?show=open|done|all` (default open) with counted
  segments. `NotesPage` is newest first. `poll` stays null: one collection is
  not a fanout.
- **Record pages** (`/{uid}`). Contacts: fields, tags, foreign properties.
  Events and tasks: time, the original time zone when it differs, reminders,
  attendees (read only), related records, recurrence in words with the raw rule
  and "Edited as a whole series". Notes: Markdown rendered as blocks. A related
  record is a link only when it is in a list this session already loaded for
  the collection; otherwise its UID is shown without a link.
- **Edit** (`GET`/`POST /{uid}/edit`). The form carries `etag` and `readat`.
  List fields post as `name.0`, `name.1`, … and `name.new`; times as
  `name.date`, `name.time`, `name.allday`; reminders as `reminder.N` and
  `reminder.new` from the offered options. Only changed fields are written, and
  parameters stay with their value. Success redirects to the record. When the
  server drops properties, the edit page comes back with `loss`. Notes answer
  `NoteDoc`, or `NotePage` when there is a loss or a notice.
- **Conflict** (`/{uid}/conflict`). A write whose version moved redirects
  here. Rows are the fields you changed that the server now holds otherwise.
  Post an action's `href`, adding `choice.N=theirs` to keep the server's value
  in row N; rows without a choice take your edit.
- **New** (`/new`). As a sheet fragment it answers `NewSheet` naming the
  collection; as a page, the empty edit form. The record gets a 32-hex UID and
  the file `{uid}.vcf` or `{uid}.ics`.
- **Delete** (`POST /{uid}/delete`) needs `etag`; the record page's action
  carries it. If the version moved, nothing is deleted and the record page
  answers with a notice.
- A source that refuses is a notice with the server's answer and a
  `source.retry` action, status 200. A read-only collection has no edit,
  delete or new: those routes answer 404.

### Fixed

- Record PUTs sent no `Content-Type`, and CalDAV/CardDAV servers that check it
  refused the write (go-webdav answers 400). Providers now send
  `text/calendar` and `text/vcard` through `dav.Typed`; `Transport` is
  unchanged.
- Descriptions and notes showed iCalendar escapes (`\n`, `\,`).
  `cal.Description` unescapes.
- A TZID the Go zone database does not know (Outlook writes Windows names) no
  longer fails the record: it is read as a wall clock in the instance zone.

### Included

- `provider/card`: `Query`, `FindUID`, `NewObject`, `Delete`.
- `provider/cal`: `Query` (objects that cannot be read are named, not fatal),
  `FindUID`, `NewObject`, `Delete`, `Alarms` and `SetAlarms`, `TimingOf`,
  `DueOf`, `Overrides`, `Attendees`, `RelatedTo`, `Text`, `EscapeText`,
  `TextList`, `TimeOf`.
- `model.NewUID`.
- `davtest`: `Drop` for lossy servers; an address book filter without
  prop-filters matches every card (RFC 6352 §10.5).

### Not yet included

- Live S-30.2 runs for records on Radicale and Baikal, and a note written by a
  real foreign client.
- First day of the week and 12/24 hours from the account: Monday and 24 hours
  until that setting exists.

## v0.0.42 — 2026-09-13

Step 1 of S-11…S-26 in [server.md](../../docs/plan/server.md): the collection
catalog lives in the sealed blob, and connecting a server writes it.

### For consumers

- **POST `/app/settings/connections` connects a server.** Fields `server`,
  `login`, `password`, `certFingerprint`. Discovery runs through the outbound
  guard with the fingerprint. Success redirects to the page (303). Failure
  answers the page again (200) with an alert notice that names the step
  discovery stopped at and carries every step in `details.lines`; nothing is
  saved, and the password is not echoed.
- **GET `/app/settings/connections` lists connections** in `accounts`: server,
  login, and per collection its label, address, colour, kinds and read-only
  flag. The password stays hidden.
- **`{col}` is the collection address encoded into one segment**
  (`/dav/personal/` → `%2Fdav%2Fpersonal%2F`), and it must be in the catalog.
  An address the catalog does not hold answers 404, like a foreign one. Bare
  names such as `/app/contacts/{account}/personal` no longer pass.
- Collection colour is the server's `calendar-color` without alpha, else one of
  eight muted tones derived from the address (C-10).

### Fixed

- Address books came back writable from every server: go-webdav's
  `FindAddressBooks` never asks for `current-user-privilege-set`. Discovery now
  reads rights for calendars and address books in the same home PROPFIND that
  fetches colour and change tag.
- Discovery ignored the accepted certificate fingerprint (F-13).
  `discover.Run` takes it and uses it for every request, well-known included.

### Included

- `sealed.Blob`: `Collections` and `Selected`; `Connection` gains
  `CalendarHome`, `AddressBookHome` and `FileRoot`.
- `discover`: homes on `Result`; `Collection.Colour`, `MaxResourceSize` and
  `ChangeTag`; calendar kinds in a fixed order.
- `user`: `Catalog`; `AddConnection` (journal `connection.add` with the
  connection ID only, `ConnCount`); `OwnsCollection` and `OwnsRecord` check the
  catalog.
- `internal/davtest`: in-memory CalDAV/CardDAV on go-webdav's own server, for
  tests.

### Not yet included

- Re-discovery offering the difference (S-6.1), and revealing a connection
  password after the unlock sheet.

## v0.0.41 — 2026-09-13

Compresses [server.md](../../docs/plan/server.md) from 783 to 655 lines.
Closed layers become one table with the versions that shipped them; the
S-11…S-26 and davloom forks stay word for word.

### For consumers

Nothing in the running server. Read what still binds wiring in «Что держится
при проводке»; closed findings and the decisions that hold live in
[reviews.md](../../docs/plan/reviews.md), «Ограничения на следующие слои».

### Changed

- Closed sections (skeleton, `view`, crypto, store, session, transport, request
  layer, users, providers), the 05.09 tail, and the first S-30.2 run are
  replaced by a version table. Their findings were already in `reviews.md`
  and this changelog.
- «Linux-машина» keeps the `dev/` and SelfPost rules and gains what the
  13.09 run taught: `GOTOOLCHAIN=auto`, Radicale `owner_only` layout, and its
  403 / 400 / 405 refusals.
- The open `gofmt` item now lists the manual gates.
- S-12 links the open `Prop.Value` question; S-30.2 acceptance is ticked for
  layers up to providers.
- `reviews.md` no longer points at the removed «Хвост» section.

## v0.0.40 — 2026-09-13

The Linux gates pass end to end for the first time: F-10, the S-28 container
smoke, and S-30.2 against a live Radicale (`scripts/linux-gates.sh`). Closes
the last run in [providers-tail.md](../../docs/plan/providers-tail.md).

### For consumers

- **The image starts on an empty volume.** `/data` now ships owned by
  `carrel` (uid 1000). A fresh named volume used to come up root-owned, and
  the server exited with `open /data/server.key: permission denied`. A volume
  already created by an older image keeps root ownership: `chown 1000:1000`
  its contents or start from a new volume.
- **PROPPATCH and MKCOL with properties send the whole XML body.** The bytes
  were taken before the encoder flushed; Radicale answered 400
  (`no element found`) to a collection rename.

### Fixed

- `docker-compose.linux.yml`: `kozea/radicale:3.2.4` does not exist in any
  registry. Now `tomsquest/docker-radicale:3.5.4.0`, the layout
  `testdata/radicale` was written for (`/config/config`, `/config/users`,
  `/data`).
- `scripts/linux-gates.sh`: the smoke run no longer sets `CARREL_BIND=127.0.0.1`
  inside the container, where the published port cannot reach it. Each run
  starts on a fresh volume; container logs print when `/healthz` fails.
- Radicale integration tests, which had never passed on a live server:
  - calendars and address books live directly in the home — Radicale's
    default `owner_only` rights allow nothing deeper;
  - paths are relative to the endpoint (`dav.Client` resolves a leading `/`
    from the host root);
  - events carry `DTSTART`; the large item is one event, not several UIDs in
    one resource;
  - collection CRUD on Radicale is create, rename (displayname), delete, and
    checks the collection is gone. No MOVE: the address is immutable
    (S-12.2), and Radicale answers 405;
  - the merge test read a shadowed `err` and seeded the target with an empty
    `X-CUSTOM`.

### Included

- `internal/dav`: `xmlDoc` builds `propPatchBody` and `mkcolBody` and keeps
  the first encoder error. `TestPropPatchBodyIsCompleteXML` and
  `TestMkColBodyIsCompleteXML` fail on the previous code.
- Open for S-12, in [providers-tail.md](../../docs/plan/providers-tail.md):
  `Prop.Value` is written as escaped text, so a raw `<collection/>` in
  `resourcetype` would not arrive as XML.

## v0.0.39 — 2026-09-12

Copies the Typecase mockup through plan 1 step 2: product CSS in
`<style data-tc>`, page chrome in a second `<style>`, and Public Sans,
Literata, and Source Code Pro next to the file.

### For consumers

Nothing in the running server. Open
[carrel-ui-mockups.html](../../docs/visual/carrel-ui-mockups.html) for the
current mockup; fonts are in `docs/visual/fonts/`.

## v0.0.38 — 2026-09-11

Changelog verified against full git history from the initial commit.

### For consumers

Nothing in the running server.

### Fixed

- `v0.0.21` date aligned with commit `2891831` (2026-09-08).
- `v0.0.37` date aligned with commit `4589e0b` (2026-09-10).

## v0.0.37 — 2026-09-10

Typecase now lives in its own repository; this tree is Carrel server and
product docs only.

### For consumers

Nothing in the running server. Check out or depend on `carrel.dev/typecase`
from its standalone repository — the `typecase/` directory is no longer in
this checkout.

## v0.0.36 — 2026-09-09

Adds [limits.md](../../docs/spec/limits.md): one reference for every timeout,
TTL, size cap, count, rate limit, and crypto bound — product constants in
`internal/conf/limits.go`, C-12 defaults, hardcoded transport and store
limits, and planned values still only in the spec.

### For consumers

Nothing in the running server. Use the file when checking or changing limits
instead of hunting through `conf.go`, `limits.go`, and architecture.md.

## v0.0.35 — 2026-09-09

Withdraws the eight-device cap from the S-27 plan. That number was not in
the spec; there is no ceiling until a person sets one.

### For consumers

Nothing in the running server.

## v0.0.34 — 2026-09-08

Closes forks for **davloom and delivery** (S-27, S-28) in
[server.md](../../docs/plan/server.md). No binary change. The image,
`/healthz`, and slog are already shipped; Composer does not rebuild them.

### For consumers

Nothing in the running server. `/app/settings/davloom` and `/admin/install`
stay stubs until that checklist is ticked.

### Included

- Same binary, second listener; device wraps live on `User` with AAD
  `carrel.device.v1`, not the login wrap. Changing the login password does
  not revoke devices; wipe does.
- Collection hrefs are `/c/{account}/{col}`, never a connection-order index.
  XML href prefixes rewrite as a stream; record bodies are not parsed.
- GET `/` on the davloom port is `AboutPage` (C-13). Basic username is the
  Carrel login. At most eight devices. Issue requires unlock.
- Install check is eleven named rows; the probe client is not the DAV guard.
  Image publish and CI stay out of this plan.

## v0.0.33 — 2026-09-08

Closes the provider tail in
[providers-tail.md](../../docs/plan/providers-tail.md) (S-30.2 integration
coverage). Run on Linux: `scripts/linux-gates.sh`; SFTPGo and Baikal need
`CARREL_SFTPGO_*` / `CARREL_BAIKAL_*` from `dev/credentials.txt`.

### For consumers

Nothing in the running server. Integration tests assert live DAV behaviour
the provider layer depends on.

### Included

- `integration/providers_test.go`: large-file heap check, merge write failure
  keeps sources, optional Baikal foreign-property round-trip.
- `integration/providers_collection_test.go`: collection CRUD on Radicale
  (address book + calendar) and SFTPGo folders.
- `scripts/linux-gates.sh`: optional Baikal block when credentials are set.
- Baikal contacts use `/dav.php/addressbooks/...` paths (endpoint is `/dav.php`).
- SFTPGo folder CRUD renames via MOVE; `x/net/webdav` ignores `If-None-Match` on
  PUT, so occupied-name check moves to `TestRadicaleCardUploadOccupiedName`.
- `integration/live_creds.go` reads `dev/credentials.txt` when `CARREL_*` unset.
- `scripts/run-live-integration.sh` / `.ps1` for manual full runs.

## v0.0.32 — 2026-09-08

Closes forks for **S-11…S-26** in [server.md](../../docs/plan/server.md).
No binary change. Composer starts at the collection catalog in the
sealed blob, not at a GET stub.

### For consumers

Nothing in the running server. HTTP routes C-5 stay stubs until the
checklist in that section is ticked.

### Included

- Commit order is not S-number order: fanout before unified view, collection
  move after record move, mail after publish.
- `Transport` is not extended (`Copy` is `Get` + `PutCreate`). Unconditional
  PUT is only file overwrite-on-demand. File `Get` drops the 10 MiB
  `LimitReader`; PROPFIND/REPORT keep it.
- Reminders are nested VALARM via `cal.SetAlarms`, not a new `PatchOp`.
- Unified-view ticks live in the blob; empty selection is empty, not “all”.
- Public `/p` live-reads with a per-link connection-password envelope under
  `server.key`. Duplicate detection uses the session cache only.
- Architecture: `Source` lives in `internal/fanout`; PHOTO URI proxy is
  `internal/photo` through the guard.

## v0.0.31 — 2026-09-08

Provider review. Closes the **Провайдеры** section in
[server.md](../../docs/plan/server.md); leftover:
[providers-tail.md](../../docs/plan/providers-tail.md).

### For consumers

- **Note Markdown keeps emphasis, strong, and links** as C-8 block kinds
  (`emph`, `strong`, `link`), not flattened text. Raw HTML is still an escaped
  `para`. Fenced code uses the node's lines, so the body is not empty.
- **`cal.Expand` uses rrule-go** on the master component: RRULE window minus
  EXDATE plus RDATE. RDATE-only events expand. Times stay UTC.
- **`file.Join` rejects `.`, `..`, empty, slash, and backslash** — not every
  leading-dot name. `.hidden` is a legal segment.
- **`card.Foreign` lists RFC properties that C-6 does not name** (LOGO, FBURL,
  …). The known set is Name / Fields / Tags, not the whole RFC 6350 dump.

### Included

- VALARM and `RECURRENCE-ID` siblings survive encode after a series patch.
- `rrule-go` v1.8.2 is a direct `require`.
- Tests: EXDATE/RDATE, RDATE without RRULE, Markdown inline kinds, code fence,
  Join slash/backslash, LOGO in Foreign.

## v0.0.30 — 2026-09-08

Closes the **Провайдеры** section in [server.md](../../docs/plan/server.md).
HTTP routes C-5 still return stub view models; wiring GET records, photos, and
file download is S-11 / S-13 / S-17. Remaining S-30.2:
[providers-tail.md](../../docs/plan/providers-tail.md).

### For consumers

- **Conditional file create uses `If-None-Match: *`.** New `dav.Creator.PutCreate`
  on `*Client`; `Transport.Put` is unchanged (`If-Match` only). Occupied names
  surface as HTTP 412 — suffix and overwrite are S-17 queue, not this layer.
- **Record providers reject updates without ETag** (S-8). Empty `ifMatch` never
  reaches the wire from `card` or `cal`.
- **`model.NewCalendarObject`** holds a full iCalendar tree; patch applies only
  to the main VEVENT / VTODO / VJOURNAL (no `RECURRENCE-ID`). VTIMEZONE, VALARM,
  and unknown properties survive encode.
- **`card.Foreign`** returns `[]view.Field` for unknown vCard properties; PHOTO,
  UID, and VERSION are excluded. `VALUE=uri` photos are not fetched (SSRF).
- **`cal.Expand`** returns UTC instants in a caller-supplied window; `cal.RawRRULE`
  returns the rule text; human recurrence copy stays in `web` (C-10).
- **`cal.RenderMarkdown`** (goldmark v1.8.6) produces `[]view.Block`; raw HTML
  nodes become escaped `BlockPara`, not nested markup.

### Included

- `internal/dav`: `Creator`, `PutCreate`, `StatusError`.
- `internal/model`: `NewCalendarObject`, calendar patch target.
- `internal/provider/file`: stream `Get`, `UploadCreate`, `Join`.
- `internal/provider/card`: parse/encode vCard, `Foreign`, inline `PhotoOpen`.
- `internal/provider/cal`: parse/encode iCalendar, `Expand`, `RenderMarkdown`.
- Direct deps: `go-ical`, `go-vcard` (pinned), `goldmark` v1.8.6, `rrule-go` v1.8.2.

## v0.0.29 — 2026-09-08

Closes the pre-provider gate. Remaining S-30.2 coverage (foreign properties,
collection CRUD, large-file RSS, occupied names, merge) stays in
[server.md](../../docs/plan/server.md) and waits on provider packages.

### For consumers

- **Admin and escrow POST actions name their target on the href.** Menu
  items on `/admin/` and `/admin/invites`, and deposit/withdraw on
  `/app/settings/escrow`, carry `action` and the account or invite identity
  in the query string (`Action` has no form body in C-6). Posting the href
  from the JSON view is enough; CSRF still travels in the form.
- **The escrow admin “covered” list is only deposited accounts.** Accounts
  that never opted in no longer appear there.
- **Outbound DAV refuses HTTPS→HTTP redirects.** The Guard does not follow
  a scheme downgrade, so Basic credentials are not copied to `http://` on
  the same host. An address entered as `http://` is unchanged (F-13 warning
  on the connections screen).
- **`AdminEscrowPage.Result` encodes absence as `null`**, not a missing
  key (pointer fields in `view` are never `omitempty`).
- **Radicale integration reset uses a typed `MKCOL`** for the parent
  calendar folder. Untyped `MKCOL` is 403 on Radicale.

### Included

- `internal/web`: `postQuery`; admin and escrow action hrefs; covered-list
  filter.
- `internal/dav`: `httpsToHTTP` in `CheckRedirect`.
- `integration/radicale_test.go`: typed parent `MKCOL`.
- Tests: `TestAdminUserMenuHrefDisables`,
  `TestAdminInviteActionsNameTheInvite`,
  `TestGetAdminEscrowCoveredOmitsUncovered`,
  `TestEscrowWithdrawActionCarriesAction`,
  `TestRedirectRefusesHTTPSDowngrade`.

### Not yet included

- Provider packages. Leftover and closed forks:
  [server.md](../../docs/plan/server.md) («Провайдеры»).
  Create is `dav.Creator.PutCreate` (`If-None-Match: *`), not a
  `Transport.Put` argument. Providers do not assemble C-6 pages.
  Calendar records are a component tree; `NewObject` stays for cards.
  PHOTO `VALUE=uri` is not fetched. Order: Creator → file → card → cal.
  Package tests only; live HTTP and the rest of S-30.2 come after.

## v0.0.28 — 2026-09-08

Closes Linux-machine hygiene from the pre-provider gate §5
([server.md](../../docs/plan/server.md), «Linux-машина»).

### For consumers

- **Radicale in `docker-compose.linux.yml` binds to `127.0.0.1:5232` only.**
  The test stack no longer listens on all interfaces on the shared dev host
  (SelfPost on the same machine).
- **Radicale integration tests use CalDAV resources, not generic WebDAV files.**
  `PUT`/`GET` run against valid `.ics` inside a typed calendar collection, as
  Radicale requires. Verify on Linux with `scripts/linux-gates.sh`.

### Included

- `docker-compose.linux.yml`: loopback port publish.
- `integration/radicale_test.go`: `icsEvent`, `largeICS`; PutIfMatch and
  GetStreams use `/cal/personal/*.ics`.

## v0.0.27 — 2026-09-08

Closes discovery well-known defects from the pre-provider gate
([server.md](../../docs/plan/server.md), S-30.2 first run).

### For consumers

- **Well-known redirects are validated before discovery switches away from the
  entered URL.** A redirect target is accepted only when a
  `current-user-principal` is found there; otherwise discovery keeps the URL
  you typed (Radicale-style web UI traps no longer replace a working path).
- **Well-known probes use the server origin** (`/.well-known/...` at the host
  root), not a path relative to the entered URL (RFC 6764).
- **HTTPS→HTTP downgrade during discovery is refused.** Credentials are not
  sent to a downgraded scheme while probing well-known; plain HTTP entered
  directly is unchanged (F-13 warning on the connections screen).

### Included

- `internal/dav`: `ResolveWellKnown` anchors well-known URIs at the origin.
- `internal/discover`: `acceptWellKnownTarget` gates principal and scheme
  before adopting a well-known redirect.
- Tests: `TestResolveWellKnownFromOriginRoot`,
  `TestWellKnownHTMLWithoutPrincipalStaysOnEnteredURL`,
  `TestAcceptWellKnownTargetRejectsHTTPDowngrade`,
  `TestWellKnownAcceptsValidRedirect`.

## v0.0.26 — 2026-09-08

Closes escrow admin UI gaps from the pre-provider gate
([server.md](../../docs/plan/server.md), users / escrow).

### For consumers

- **Escrow recovery shows the temporary password on `/admin/escrow`.** After
  a successful recover the page returns with a copyable one-time password and
  an honest mail status (`SMTP not configured` until S-26). The password is
  not stored in state; refresh clears it.
- **Changing the escrow master password has a form when escrow is on.** Policy,
  master rotation, and account recovery are separate forms on the same page.

### Included

- `view`: `EscrowRecoveryResult`; `AdminEscrowPage.Master` and `.Result`.
- `internal/web`: recover success renders the page instead of redirecting;
  master-password form when escrow is enabled.
- Tests: `TestPostAdminEscrowRecoverShowsTempPassword`,
  `TestGetAdminEscrowMasterFormWhenEnabled`, `TestPostAdminEscrowChangeMaster`.

## v0.0.25 — 2026-09-08

Closes the audit №2 tail. Remaining work (discovery,
Linux) closed in `v0.0.27`…`v0.0.29`.

### For consumers

- **Escrow recovery burns outstanding wipe tokens (Ф2-4).** After an
  administrator recovers a deposited account, old `/forgot/{token}` and
  console wipe links cannot empty it. The recovery journal entry no longer
  claims a notice was mailed while outbound mail (S-26) is absent.

### Included

- `internal/user`: `burnWipeTokens` in `RecoverEscrow`; honest recover audit.
- Test: `TestEscrowRecoverBurnsWipeTokens`.

### Not yet included

Discovery defects and Linux compose closed in `v0.0.27`…`v0.0.28`.

## v0.0.24 — 2026-09-08

Closes audit perimeter №3 (secret-in-URL headers and public stubs).

### For consumers

- **Pages with a secret in the URL send `Referrer-Policy: no-referrer`.**
  Applies to `/invite/{token}`, `/forgot/{token}`, `/confirm-email/{token}`,
  and `/p/{token}` (including subpaths). Outbound links from these pages no
  longer leak the token through the `Referer` header.
- **Public secret stubs answer invalid, revoked, and expired tokens the same
  way.** Download, file, and backup routes use `tokenNotFound` until
  implemented, so probing cannot distinguish a dead token from a missing one.

### Included

- `internal/web`: `headers.go`, `setSecretPathHeaders` in middleware;
  published download/file stubs always route through `tokenNotFound`.
- Tests: `TestSecretPathReferrerPolicy`, `TestPublicSecretStubsAnswerAlike`.

## v0.0.23 — 2026-09-08

Closes admin panel handlers from the audit №2 tail
and the pre-provider gate §3 (people,
invites, roles, wipe/recover, audit).

### For consumers

- **`POST /admin/` and `POST /admin/invites` perform administrator actions**
  instead of re-rendering GET. People: disable, delete (type login to confirm),
  change role, end sessions, reset password (same wipe token as `/forgot`, with
  actor in the journal). Invites: create, revoke, extend (+7 days, same
  secret), resend (new secret). Last administrator cannot be disabled,
  demoted, or deleted. Escrow-covered accounts get recovery instead of wipe.
  Without confirmed email the wipe link is shown to the administrator; until
  S-26 outbound mail stays honest (`SMTP not configured`).
- **`GET /admin/` lists people** (role, created, last login, connection count,
  live sessions, disabled, escrowed). **`GET /admin/invites`** lists live
  invites with copyable links. **`GET /admin/audit`** shows the instance
  journal.

### Included

- `internal/store`: invite `Secret` inside encrypted state for panel links
  (S-10.3).
- `internal/user`: `RevokeInvite`, `ExtendInvite`, `DeleteUser`, `SetUserRole`,
  `EndUserSessions`, `AdminResetPassword`, `SetSelfRegEnabled`.
- `internal/session`: `LiveCount`.
- `internal/web`: admin panel handlers; `sendInviteMail` stub until S-26.
- Tests: `admin_test.go` in `user` and `web`.

### Not yet included

Admin settings, DAV allowlist, install screen; escrow recovery UI gaps.

## v0.0.22 — 2026-09-08

Closes `CARREL_BASE_PATH` from the audit №2 tail.

### For consumers

- **`CARREL_BASE_PATH` prefixes routes, redirects, form actions, and cookie
  paths.** Set it when Carrel sits behind a reverse proxy in a subdirectory
  (for example `/carrel`). Requests must include the prefix; bare `/login` no
  longer matches. Session and CSRF cookies are scoped to the base path.

### Included

- `internal/conf`: `NormalizeBasePath` canonicalises the setting on load.
- `internal/web`: `http.StripPrefix` on the handler, `route()` for outbound
  paths, cookie `Path` from `CARREL_BASE_PATH`.
- Tests: `TestNormalizeBasePath`, `TestBasePathRoutesAndCookies`.

### Not yet included

Admin panel actions (other than escrow), escrow recovery UI gaps.

## v0.0.21 — 2026-09-08

Closes `/app/password` from the pre-provider gate.

### For consumers

- **`POST /app/password` changes the login password without re-encrypting
  data.** The data key is re-wrapped under the new password; saved DAV
  connections and the sealed blob stay readable. Wrong current password shares
  the login rate bucket. After a successful change, `MustChangePassword` is
  cleared, outstanding wipe links for the account are burned, and every other
  live session of that person ends (the browser that submitted the form keeps
  its session).

### Included

- `internal/store`: `ChangePassword` via `RewrapDataKey`.
- `internal/user`: `ChangePassword` — audit `password.change`, wipe burn,
  session cleanup.
- `internal/session`: `EndAccountExcept`.
- `internal/web`: `POST /app/password` handler and form (current, new, repeat).
- Tests: `TestChangePasswordPreservesDataKey`,
  `TestChangePasswordBurnsWipeTokensAndEndsOtherSessions`,
  `TestEndAccountExceptKeepsNamedSession`, `TestChangePasswordHTTP`.

### Not yet included

Admin panel actions (other than escrow), escrow recovery UI gaps,
`CARREL_BASE_PATH` on cookies and routes.

## v0.0.20 — 2026-09-07

Closes key escrow (S-10.5) from the audit №2 tail.

### For consumers

- **Key escrow is implemented end-to-end.** An administrator enables it with a
  master password on `/admin/escrow`. New accounts created while coverage is on
  deposit their data key when the password is set; existing accounts opt in from
  `/app/settings/escrow` after unlock. Recovery re-wraps the data key under a
  temporary password (connections stay readable), is rate-limited like login,
  and is always logged and mailed. Withdrawal removes the copy in one step.
- **`/forgot` asks for the administrator** when the account has a deposited
  key, not merely when escrow policy applies. Wipe links are unchanged for
  everyone else.
- **First login under escrow coverage** shows an explicit notice in the shell.

### Included

- `internal/crypt`: X25519 deposit/open, master-password wrap (`escrow.go`).
- `internal/store`: escrow fields in state, deposit on `ApplyPassword`,
  `RewrapPasswordWithKey` for recovery.
- `internal/user`: `escrow.go` — enable, policy, master change, deposit,
  withdraw, recover, first-login notice.
- `internal/web`: `handlers_escrow.go`, mail stub for recovery notice.
- Tests: `TestEscrowNotRetroactive`, `TestEscrowWithdrawRemovesCopy`,
  `TestEscrowRecoverRewrapsDataKey`, `TestForgotEscrowRequiresAdminWhenDeposited`,
  `TestEscrowDepositRoundTrip`, `TestEscrowMasterWrapRoundTrip`.

### Not yet included

Admin panel actions (other than escrow), escrow recovery UI gaps,
`CARREL_BASE_PATH` on cookies and routes.

## v0.0.19 — 2026-09-07

Continues the audit №2 tail: multipart CSRF.

### For consumers

- **Multipart POST accepts `_csrf` in the body.** When `X-CSRF-Token` is
  absent, the server reads `_csrf` from `multipart/form-data` within the
  route body limit (1 MiB for forms). Upload and import routes can rely on
  the header without buffering the file for the token; native form POSTs
  still send `_csrf` as a field (C-11).

### Included

- `internal/web`: `csrf_body.go`; middleware uses `ParseMultipartForm` for
  CSRF when the header is missing.
- Test: `TestMultipartCSRFWithoutHeader`.

### Not yet included

Admin panel actions (other than escrow), `/app/password`,
`CARREL_BASE_PATH` on cookies and routes.

## v0.0.18 — 2026-09-07

Continues the audit №2 tail:
anonymous CSRF, entity ID prefixes, and honest mail copy before S-26. Each
fix ships with a test that was red on v0.0.17.

### For consumers

- **Anonymous forms require double-submit CSRF.** GET `/login`, `/setup`,
  `/register`, `/forgot`, `/invite/{token}`, and `/forgot/{token}` set
  `carrel_csrf` and `_csrf`. POST without a matching cookie and field (or
  `X-CSRF-Token`) answers 403 before the handler runs. Cross-site POST
  `/login` no longer authenticates (Ф2-15). Logged-in POST still uses the
  session token.
- **People, invitations, and connections have distinct ID prefixes.**
  `User.ID` is `u-`, `Invite.ID` is `i-`, DAV connections stay `a-`. Invite
  placeholders no longer share the invitation id.
- **`/forgot` and `/register` no longer claim an email was sent** while the
  mail layer (S-26) is absent. Tokens stay in state and never appear in HTTP
  responses; the page says outbound mail is unavailable instead.

### Included

- `internal/web`: `csrf_anon.go`, `anonPlainForm`, anonymous CSRF in
  `middleware`; `mail_stub.go` until S-26 dispatches messages.
- `internal/user`: `NewUserID`, `NewInviteID`; `NewAccountID` reserved for
  connections.
- Tests: `TestAnonymousCSRFBlocksLoginWithoutCookie`,
  `TestForgotDoesNotClaimMailSent`, `TestEntityIDPrefixes`; existing web
  tests updated for anonymous CSRF on public forms.

### Not yet included

Key escrow (S-10.5), admin panel actions, `/app/password`,
`CARREL_BASE_PATH` on cookies and routes.

## v0.0.17 — 2026-09-07

Closes Fable audit №2 (accounts and secrets, Ф2-1…Ф2-15 in
[reviews.md](../../docs/plan/reviews.md)). Every fix ships with a test that
was red on v0.0.16. Layers after the audit closed in `v0.0.18`…`v0.0.29`.

### For consumers

- **Open self-registration can no longer wipe an existing account.** A
  record is reused only as the same unconfirmed self-registration (same
  login, same address, password already set, no pending invite); anything
  else answers "login is already taken". Before: `POST /register` with an
  existing login that had no confirmed address — the first administrator,
  for one — destroyed that account's data key (Ф2-1).
- `{account}` in every `/app/...` route is now checked against the
  connections in the viewer's sealed blob, opened with the session key. A
  foreign connection and the viewer's own user ID both answer 404 (Ф2-7).
- `POST /app/settings/unlock` requires the account password, opens the
  window in this session only, counts against the login bucket (5/min), and
  writes `secret.unlock` to the audit log. `UnlockSheet` is served by
  `GET /app/settings/unlock` as a fragment (Ф2-9).
- `CARREL_TRUSTED_PROXIES` is honoured: behind a listed proxy the rate
  buckets key on the rightmost untrusted `X-Forwarded-For` address and
  `X-Forwarded-Proto: https` sets the Secure cookie flag. From an untrusted
  peer both headers are ignored (Ф2-11).
- Logging in ends the session the browser presented and issues a fresh SID;
  the session stores the normalised login (Ф2-8).
- Resending an invitation kills the previous secret; setting a password by
  any path burns every outstanding forgot and console link of that account
  (Ф2-3, Ф2-4). Disabled accounts get no `/forgot` link (Ф2-14).
- Unknown login, disabled account, unconfirmed address, and an account
  without a password each cost one Argon2 derivation, like a wrong password
  (Ф2-6).
- Password forms on `/invite/{token}` and `/forgot/{token}` show "at least 8
  characters" and "passwords do not match" instead of a 404; internal error
  text no longer reaches any form (Ф2-10).
- `/healthz` and `/about` are exempt from the session bucket (Ф2-13).
- `carrel-wipe` fails on a data directory without a volume instead of
  creating a `server.key` there (Ф2-12).

### Included

- `internal/user`: `Register` and `ForgotRequest` return the link token for
  the mail layer; `CreateInvite(actorID, …)` records the actor; `Unlock`;
  `OwnsAccount(viewerID, dataKey, account)`; `verifyDummy`;
  `burnWipeTokens`; `pruneTokens` (used and expired tokens are dropped).
- `internal/store`: `Invite.Used`; `VerifyPassword`.
- `internal/session`: `UnlockUntil` / `SetUnlockUntil`, cleared on end.
- `internal/web`: `Limiter` sweeps refilled buckets past 4096 entries per
  family (Ф2-2); `tooLarge` uses `errors.As`; `NewStubUsers` removed —
  tests run against `user.Service`.
- Tests: 17 new in `internal/user`, 14 in `internal/web`, 2 in the limiter.
  `internal/user` coverage across module tests 34.3% → 81.0%.

### Not yet included

Key escrow (S-10.5), admin panel actions, `/app/password`, CSRF on
multipart uploads, `CARREL_BASE_PATH` on cookies and routes. Mail delivery
(S-26): tokens are stored; dispatch is still a stub.

## v0.0.16 — 2026-09-05

Adds Linux-only delivery and acceptance infrastructure from
[server.md](../../docs/plan/server.md) («Linux-машина»).

### For consumers

- `Dockerfile`: multi-stage static `carrel` image, non-root user, `/data` volume.
- `docker-compose.linux.yml` + `testdata/radicale/`: live Radicale for manual
  S-30.2 checks on the dev Linux host (not CI).
- `scripts/linux-gates.sh`: runs F-10 (`server.key` mode test), S-28 container
  smoke (`--read-only`, `--cap-drop=ALL`, `/healthz`), and S-30.2 integration
  tests against Radicale.

### Included

- `integration/` with `-tags=integration`: discovery on a real Radicale, PUT
  `If-Match`, streaming GET.
- S-28 Dockerfile at `carrel/server/Dockerfile` per [architecture.md](../../docs/plan/architecture.md).

### Not yet included

Full S-30.2 checklist (collection CRUD through providers, merge, large-file RSS)
— blocked on provider layers; integration tests cover transport and discovery only.

## v0.0.15 — 2026-09-05

Closes the Composer tail from [server.md](../../docs/plan/server.md) (Fable audit
№1 leftovers and the 05.09.2026 verification pass).

### For consumers

- Argon2id runs under a process-wide semaphore (four concurrent derivations).
  Extra login, registration, password-change, and escrow attempts wait on the
  request context instead of piling up memory (F-7).
- HTTPS DAV connections can proceed with a self-signed or otherwise untrusted
  leaf certificate only after the human accepts its SHA-256 fingerprint on the
  connections screen; the fingerprint is stored in the sealed connection blob
  beside the password and must be accepted again when the certificate changes
  (F-13). Plain `http://` still shows a warning, not a refusal.
- `store.RewrapLogin` re-seals the wrapped data key and user blob when a login
  rename is added later (F-14).

### Included

- `internal/crypt`: KDF semaphore; `DeriveKey`, `HashLogin`, `VerifyLogin`, and
  `DeriveWrapKey` take `context.Context`.
- `internal/dav`: `LeafFingerprint`, `HTTPClientWithCertPin`, pin-aware TLS
  dial; `NewClient` gains a `certPin` argument.
- `internal/sealed`: connection blob with `certFingerprint`.
- `internal/store`: `RewrapLogin`; `ApplyPassword` and `DataKeyForLogin` pass
  context through to KDF.
- `internal/web`: certificate fingerprint field and warning on the connections
  screen.
- `cmd/carrel-wipe/main_test.go`: one-time link, 15-minute token, audit entry.

### Fixed

- CHANGELOG no longer uses `carrel/server/v0.0.N` headings that looked like git
  tags when none existed.

## v0.0.14 — 2026-09-05

Restores the `docs/spec/check.py` step in `carrel-server.yml`. No server
code changes.

### For consumers

- No API, binary, or configuration changes. Skip this tag if you only track
  functional server releases.

### Fixed

- Checks 3 and 3b failed on every server commit: `carrel-client.md` still
  named `tc-row--file` in five U-6 widget lists after Typecase renamed the
  row to `tc-row--meta`, and map U-5.4 had no row for frame class
  `m-row--file`. The spec now matches L-4 and the mockups; `check.py` returns 0.

## v0.0.13 — 2026-09-05

Implements S-10 user administration: accounts, invitations, registration,
forgot-password wipe flow, and console recovery.

### For consumers

- `internal/user` manages two roles, first-run setup, invite links (≥32-byte
  secrets, hashed at rest, constant-time compare), self-registration when mail
  is configured, and the two-step `/forgot/{token}` wipe (not recovery).
- `cmd/carrel-wipe <login>` prints a one-time 15-minute console wipe link and
  records the event in the audit log.
- Escrow stays disabled by default; instance settings live in `state.enc`.
- One in-memory unlock window (`CARREL_UNLOCK_TTL`) applies to all secret
  releases via `UnlockUntil` / `SetUnlockUntil`.
- The last administrator cannot be disabled (`ErrLastAdmin`).

### Included

- `internal/store`: `InstanceSettings`, `Invite`, `SecretToken`, profile fields
  on `User`, `ApplyPassword`, `WipeCredentials`, `DataKeyForLogin`, lookups.
- `internal/user`: `Service` implementing `web.Users`.
- `cmd/carrel-wipe`, `cmd/carrel` wired to `user.New`.

### Not yet included

Providers and S-11…S-26 features above the request layer.

## v0.0.12 — 2026-09-05

Adds the HTTP request layer (S-9): all C-5 routes, CSRF, rate limits,
ownership checks, JSON `Renderer`, and page/fragment distinction by
`Carrel-View` header.

### For consumers

- `internal/web` registers every route from C-5. Mutating requests with a
  session require `X-CSRF-Token` or `_csrf` before the handler runs; bodies are
  capped at 1 MiB.
- Rate buckets from architecture.md: login 5/min per account and 20/min per IP,
  anonymous tokens 30/min per IP, 120/min per session after login.
- Foreign `{account}` / `{col}` / `{uid}` returns 404, same as missing (S-9.5).
- Invalid public secrets use uniform timing and a single 404 response.
- `http://` connections show a warning on the connections screen (F-13); DAV
  transport errors surface as `DiagnosticTrace` on screen, not HTTP 500.
- Out-of-band fragment targets follow the closed C-7.3 table.
- App routes return minimal view models until providers land; auth routes call
  `web.Users` (implemented by `internal/user` in v0.0.13).

### Included

- `internal/web`: routes, middleware, `JSONRenderer`, `NewStubUsers` for tests.
- `cmd/carrel`: serves the full route map through `web.Server`.

### Not yet included

Providers, fanout, and feature handlers that need live DAV data.

## v0.0.11 — 2026-09-05

Fixes a cache-eviction defect found while verifying that the Fable debt was
actually closed. The gates were green; the test that guards the rule passed by
luck about seven runs in eight.

### For consumers

- Cache eviction now picks the genuinely least recently used entry. Recency is
  a monotonic counter, not `time.Now()`. Two entries stored in the same clock
  tick carried identical timestamps, and the victim was chosen by walking a Go
  map — whose iteration order is randomised — so the **newest** body was
  evicted instead of the oldest in 12.75% of rounds (255 failures in 2000).
  Both the per-session and the process-wide paths were affected.

  The kind ordering (bodies before version maps, S-4.2 and C-12) was never in
  doubt; what was random was which body went. In service this showed up as a
  cache that could drop a body fetched moments ago and keep an older one —
  the thrash S-4.2 exists to prevent.

- `TestBodiesEvictedBeforeMaps` now runs its scenario 200 times. A single pass
  could not tell a correct cache from a coin flip; the strengthened test fails
  on round 4 against the old code.

### Note on the previous fix

`v0.0.9` replaced insertion order with `at` timestamps to give the per-session
and process-wide paths one rule. The instruction to do that is mine — it is in
the debt section of [server.md](../../docs/plan/server.md) — and it traded a
deterministic tiebreak for a coarse clock. One rule was the right goal; the
clock was the wrong key for it.

## v0.0.10 — 2026-09-05

Closes the Fable verification debt on commit `1dd02cb` (findings Ф-15…Ф-17 and
follow-ups from [reviews.md](../../docs/plan/reviews.md)).

### For consumers

- DAV transport credentials no longer leak on cross-host redirects: custom methods
  use the same `HTTPClientWithBasicAuth` path as go-webdav, not a transport-level
  Basic wrapper.
- `httpStatusError` reads at most 256 bytes of a remote error body.
- `cmd/carrel` runs `PurgeExpired` every minute so abandoned sessions release
  keys and cache without another request.
- `Cache.InvalidateCollectionList(account)` clears a cached collection list after
  collection CRUD (S-12).
- Ended sessions ignore `AddHistory` and `NoteLoss`.

### Included

- `internal/dav/client.go`: single guarded client, `hc.Do` for all methods;
  redirect auth tests; limited error-body read test.
- `cmd/carrel/main.go`: session purge ticker with shutdown hook.
- `internal/session`: `InvalidateCollectionList`; `ended` on history/loss;
  `TestPurgeExpiredRemovesOrphanSession`.

### Not yet included

The request layer — ready to start per [server.md](../../docs/plan/server.md).

## v0.0.9 — 2026-09-05

Closes the first Fable audit debt from [docs/plan/server.md](../../docs/plan/server.md)
("Долг после аудита Fable №1", findings Ф-1…Ф-12).

### For consumers

- **Breaking (`internal/session`).** Cache methods for bodies, etag maps, and
  thumbnails now take a connection identifier first:
  `PutBody(connID, path, version, data)`, `GetBody`, `PutETagMap`, `GetETagMap`,
  `PutThumb`, `GetThumb`, `InvalidatePath(connID, path)`. Collection lists are
  unchanged (still keyed by account). `Session.DataKey()` returns `([32]byte, bool)`.
- `Manager.Get` ends expired sessions and reports them as missing; `Manager.Touch`
  uses the same lock order. `Manager.EndAccount(accountID)` ends every live session
  for one account.
- `CARREL_DAV_CONNECT_TIMEOUT`, `CARREL_DAV_REQUEST_TIMEOUT`,
  `CARREL_DAV_MAX_RESPONSE_BYTES`, `CARREL_PROGRESS_SOURCE_TIMEOUT`, and
  `CARREL_PROGRESS_TOTAL_TIMEOUT` must be positive; `CARREL_DAV_MAX_REDIRECTS`
  must not be negative. Zero previously disabled timeouts and response caps.
- Outbound DAV methods that bypassed go-webdav (`Get`, `Put`, `Delete`, `PropFind`,
  `PropPatch`, `MkColProps`) now send Basic auth like the rest of the client.
- `LoadServerKey` rejects `server.key` with group/other permissions on Unix
  (`ErrServerKeyPermissive`).
- Stored KDF parameters are validated on load and on every `Mutate`; zero params
  are allowed only before login/wrap material exists.

### Included

- `internal/dav`: extended SSRF blocks (NAT64/6to4 embedded IPv4, deprecated v6
  ranges, extra v4 ranges); IPv6 allowlist literals; redirect and NAT64 resolver
  tests; Basic auth on custom transport methods; 256-byte sanitized remote errors.
- `internal/session`: expiry in `Get`/`Touch`; inert ended sessions; connection-
  scoped cache keys; `EndAccount`.
- `internal/crypt`: `ValidateKDFParams`; `UnwrapDataKey` wipes plaintext buffer;
  `DeriveWrapKey` documents caller wipe duty.
- `internal/store`: KDF bounds on decode and mutate.
- `internal/conf`: positive DAV/progress timeout validation.

### Not yet included

The request layer — Ф-7 (KDF process semaphore), Ф-8 (`store.User` projection),
Ф-13 (Basic over plain HTTP), Ф-14 (login rename re-wrap) are constraints on
later layers; audit №2 checks them.

## v0.0.8 — 2026-09-04

Closes the layer-review debt from [docs/plan/server.md](../../docs/plan/server.md)
("Долг после ревью слоёв"), except the Fable audit perimeter.

### For consumers

- `CARREL_CACHE_COLLECTION_TTL` now governs collection lists and etag maps in
  the session cache. Bodies and thumbnails stay keyed by object version and are
  not TTL-governed (S-4.2).
- `CARREL_LOG_LEVEL` configures `slog` at startup (`debug`, `info`, `warn`,
  `error`). An unknown level is a startup error, not a silent fallback.
- Session cache eviction within one session now uses the same rule as the
  process-wide budget: least recently used by `at`, with bodies evicted before
  maps.

### Included

- `internal/session/cache.go`: TTL on read for collection lists and etag maps;
  unified eviction by `at`; tests for TTL expiry and recency under byte pressure.
- `cmd/carrel`: `setupLog` from C-12; S-29 test that debug startup lines carry
  no passwords or tokens.
- `internal/conf`: `ParseLogLevel`, negative cache-limit validation, expanded
  tests for `settings.env`, env override, and bounds.
- `internal/dav/client_mutate_test.go`: `Put`, `Delete`, `Move`, `PropPatch`,
  `MkCol`, `MkColProps` against a fake server, including `If-Match` and 404-on-
  `Delete` as success.
- `.github/workflows/carrel-server.yml`: separate `CGO_ENABLED=0` build and
  `CGO_ENABLED=1 go test -race`, plus `gofmt`, `go vet`, and `docs/spec/check.py`.
- `go.mod`: `toolchain go1.27.1` per [architecture.md](../../docs/plan/architecture.md).

### Not yet included

The request layer — unblocked after v0.0.9 closes Fable audit №1 debt.

## v0.0.7 — 2026-09-04

Review of the crypt / store / session / dav layers, and the eight defects it
found. No new packages.

### For consumers

- **Breaking (`internal/model`).** `NewObjectFromRaw` is replaced by
  `NewObject(path, etag string, props map[string][]RawValue)`. The old
  constructor was exported but took `map[string]propState`, an unexported
  type — no provider package could name the argument, so the model had no
  usable way in. `RawValue` carries a value's text and parameters; property
  and parameter names fold to upper case. `Object.Props()` returns the record
  for display as a copy. `raw` stays unexported: nothing above this package
  can rebuild an iCalendar or vCard object out of form fields (S-7).
- **Breaking (`internal/session`).** `Session.Drafts()` and `Session.Losses()`
  are gone; they handed out pointers to fields that other requests were
  writing. Use `Draft(slot)` / `SetDraft(slot, data)` with `DraftRejectedEdit`,
  `DraftPendingPhoto`, `DraftPendingImport`, and `NoteLoss` / `HasLoss` /
  `LostProps`. `Session.Losses()` still returns the aggregator to hand to
  `model.CompareAfterPut` — now a `LossNotes` value that locks the session,
  rather than a pointer into it.
- `internal/dav` exports `ErrResponseTooLarge`. A reply over
  `CARREL_DAV_MAX_RESPONSE_BYTES` is a state to put on the screen, not a 500.
- `conf.Load` returns an error for a malformed value instead of panicking, and
  rejects `CARREL_DAVLOOM_PORT` equal to `CARREL_PORT` — davloom gets its own
  listener.
- Outbound DAV requests no longer honour `HTTP_PROXY`/`HTTPS_PROXY`. With a
  proxy set, the dialler connected to the proxy and the real target was never
  IP-checked, which is the whole of the guard.
- Audit `What` must now be an event code (lowercase letters, digits, `.`, `_`,
  `-`; no spaces) such as `user.create` or `password.change`.

### Fixed

- `Transport.PropFind` returned property names with no values: the decoder
  kept only the element name. Nothing could read `displayname`, `getctag`, or
  `getetag`. It also took the status from the last `propstat`, so an ordinary
  reply (200 for the properties present, 404 for those asked for and absent)
  came back as 404 with both sets of names merged.
- `Session` and its `Cache` had no lock. One session serves several requests
  at once, so `Cache.put` was a `concurrent map writes` crash, not a latent
  race; and the "at most four live fanouts" ceiling did not hold under
  concurrent starts.
- Cache eviction ran backwards: version maps were given up before bodies,
  the reverse of C-12 and S-4.2. `CARREL_CACHE_MAX_ETAG_ENTRIES` and
  `CARREL_CACHE_MAX_THUMB_ENTRIES` count entries, as C-12 says, instead of
  being multiplied into a byte budget; `CARREL_CACHE_MAX_COLLECTIONS` is now
  enforced at all.
- A reply of exactly `CARREL_DAV_MAX_RESPONSE_BYTES` was refused as an
  overrun. One byte more still is.
- A login containing `secret`, `token`, or `password` made every audit entry
  about that account invalid, and a rejected audit entry fails the whole
  `Mutate` — the account could not be created, changed, or disabled. The
  secret scan now covers `Detail`, the one free-form field.
- `state.enc` is durable across a crash: the directory is synced after the
  rename. `server.key` is synced before it is trusted.
- Discovery reported `trace.OK = true` even when both home-set lookups
  failed. A principal with no collections is a bad check on the screen.
- `Guard` tried only the first permitted address for a host with several.
- `cmd/carrel` sets `ReadHeaderTimeout`.

### Included

- `internal/dav/multistatus_test.go`: PROPFIND keeps values and takes the
  status from the successful `propstat`; the response cap is exact.
- `internal/session/concurrent_test.go`: sixteen goroutines on one session;
  the four-fanout ceiling under a burst; bodies evicted before maps within a
  session and across the process budget.
- Regression tests for the config errors, the davloom port collision, audit
  identities that look like keywords, and the model's ingress.
- `internal/discover/run_test.go`: `Run` is finally executed — no principal
  gives file collections, a bare server falls back to the root, a principal
  with no home-sets is a bad check, and every step leaves a trace with a
  detail. Coverage of the package went from 18.5% to 79.0%; the test that
  claimed to cover the file-only path asserted the fields of a struct literal
  it had just written, so neither `Run` nor `fileCollections` had ever run.
- Compile-time contract assertions: `Client` satisfies `Transport`,
  `calReports`/`cardReports` satisfy the reporter interfaces, and `LossNotes`
  satisfies `model.LossAggregator`. Nothing in the module refers to these
  interfaces as types yet, so until the request layer and the providers exist
  they were held to nothing.

### Verified

The whole module is clean under `go test -race` (gcc 16.1.0 from the MSYS2
UCRT64 environment). Both concurrency tests were shown red first, as C-16
asks: with the cache locks removed the detector reports the race in
`Cache.put` against `Cache.InvalidatePath`, and with the lock removed from
`AcquireFanout` the four-fanout ceiling test fails outright.

Building the server stays `CGO_ENABLED=0` — S-28 wants a static binary and
architecture.md forbids cgo in the product. The compiler is for the test
run only, and the two are separate commands.

The `gofmt` gate was red on any fresh clone: with `core.autocrlf=true` and no
`.gitattributes`, a Windows checkout gets CRLF and `gofmt -l` flags every Go
file in the repository. A root `.gitattributes` now pins LF. This working
copy only looked green because its files had been written with LF and never
checked out again.

### Known debt

The layer review fixed the eight defects above but left the rest for the work
between v0.0.7 and the request layer. Checklist with rationale:
[docs/plan/server.md](../../docs/plan/server.md) ("Долг после ревью слоёв").
None of these add new `CARREL_*` variables.

- **`CARREL_CACHE_COLLECTION_TTL` and `CARREL_LOG_LEVEL` are parsed but not
  applied.** Collections can sit in the session cache past the configured TTL;
  `slog` stays at its default level. At `debug`, S-29 still needs a check that
  passwords, tokens, and record bodies never reach the log.
- **Session cache eviction follows two rules.** `evictProcessWide` evicts by
  `at`, while `c.order` is named LRU but tracks insertion order — a `get` bumps
  the timestamp, not the position.
- **Coverage gaps (04 Sep 2026).** `session` 87.5%, `discover` 79.0%,
  `crypt` 78.0%, `store` 77.4%, `model` 73.8%; `internal/dav` 44.2% and
  `internal/conf` 41.2% still need tests — DAV mutating methods against a fake
  server (including `If-Match` / S-8 and 404-on-`Delete` as success), and
  `settings.env` merge order with bounds.
- **Gates have no CI home yet.** Build stays `CGO_ENABLED=0`; tests need
  `CGO_ENABLED=1 go test -race` as a separate command. `gofmt` must run on a
  clean checkout, not only this working tree. The toolchain pin says Go 1.27.1;
  this machine runs 1.27.0.
- **First Fable audit not run.** Two SSRF cases beyond the architecture.md
  fork list (IPv4-mapped IPv6, NAT64, 6to4 with a private address inside) and
  `store.containsSecret` as a `Detail`-only heuristic are for the audit to
  decide, not for Composer to close alone. The request layer must not start
  until that audit is done ([reviews.md](../../docs/plan/reviews.md)).

### Not yet included

The request layer, users, providers, and everything above them — after the
debt listed below is closed (done in v0.0.8 except the Fable perimeter).

## v0.0.6 — 2026-09-04

DAV transport, SSRF guard, discovery, and loss detection.

### For consumers

- `internal/dav` implements the S-5 `Transport` interface. `Get` returns an
  `io.ReadCloser` stream with an ETag, not `[]byte`. Outbound requests use
  `Guard` from C-12: resolve → IP check → dial to the verified address,
  re-check after each redirect, host allowlist (`CARREL_DAV_SSRF_ALLOWLIST`),
  connect/request timeouts, redirect cap, and `LimitReader` on responses.
- CalDAV and CardDAV `REPORT` methods live on separate `CalReporter` and
  `CardReporter` interfaces — the file provider needs only `Transport`.
- `internal/discover` runs S-6 discovery: `.well-known`, direct URL, principal,
  home-sets, collections. Each step appends to `view.DiagnosticTrace`. No
  principal means a file collection, not failure. `CompareCollections` returns
  added/removed paths for re-discovery without silent replacement.
- `internal/model.CompareAfterPut` compares a sent object with a re-read copy
  after PUT (S-7.1). Losses aggregate per server through `LossAggregator`
  (implemented by `session.LossRegistry`); repeat losses set `Aggregated`.
- Runtime dependency: `github.com/emersion/go-webdav`
  `v0.0.0-20260625083450-1901d193ce99`.

### Included

- `internal/dav`: `Transport`, `Guard`, `Client`, `CalReporter`, `CardReporter`;
  SSRF and streaming GET tests.
- `internal/discover`: `Run`, `CompareCollections`, `Collection`; unit tests.
- `internal/model`: `CompareAfterPut`, `LossAggregator`, `Loss`; loss tests
  (patch apply tests unchanged from v0.0.4).

### Not yet included

The request layer, users, providers, and everything above them.

## v0.0.5 — 2026-09-04

Encrypted storage and in-memory sessions.

### For consumers

- `internal/store` owns the on-disk instance state: one `state.enc` file
  (`CRL1` header, format version, XChaCha20-Poly1305 under `server.key`).
  Writes are atomic (temp + fsync + rename); a failed write leaves neither
  data nor audit lines changed. Files from a newer build are rejected.
- `ConnectionCount` lives outside each user's sealed blob so administration
  can read it without the data key.
- `internal/session` holds data keys, CSRF tokens, idle (12 h) and absolute
  (7 d) lifetimes, drafts, ten-line query history, per-server loss notes, and
  at most four live fanouts per session.
- Session cache keys include object versions; bodies are evicted before etag
  maps; a process-wide byte budget evicts across all live sessions without
  mixing data. `End`, expiry, and `EndAll` wipe keys and cache entries.
- `cmd/carrel` opens the store on startup and logs first-run mode when
  `state.enc` is absent. Routing is still only `/healthz`.

### Included

- `internal/store`: `Open`, `Mutate`, `Snapshot`, `ConnectionCount`,
  `FirstRun`; audit validation; S-30.1 storage tests.
- `internal/session`: `Manager`, `Session`, cache, drafts, history, loss
  registry, fanout slots; S-30.1 cache and session tests.

### Not yet included

Transport, guard, discovery, the request layer, users, providers, and
everything above them.

## v0.0.4 — 2026-09-04

Configuration, crypto, and the record model foundations.

### For consumers

- `cmd/carrel` now loads all C-12 settings through `internal/conf` instead of
  reading `CARREL_BIND` and `CARREL_PORT` alone. Defaults match C-12; a
  `settings.env` file in `CARREL_DATA_DIR` is overridden by the process
  environment. `CARREL_DAVLOOM_PORT` is supported here (empty = disabled) even
  though it is not in C-12.
- Session durations, secret TTLs, the 1 MiB form-body cap, rate-bucket limits,
  and password-length floors live as named constants in `internal/conf`, not as
  new `CARREL_*` variables.
- `internal/crypt` implements S-2: two independent Argon2id applications
  (login vs wrap), XChaCha20-Poly1305 with per-purpose AAD, and `server.key`
  creation with `O_EXCL`. Nothing else imports it yet.
- `internal/model` defines `Object` and `Patch` with the closed operation set
  from architecture.md (`SetText`, `AddValue`, `SetValues`, `RemoveProp`,
  `SetParam`, `RemoveParam`). `raw` stays unexported; rejected patches change
  nothing.

### Included

- `internal/conf`: full C-12 surface, `CARREL_DAVLOOM_PORT`, `settings.env`
  merge order, and architecture.md limit constants.
- `internal/crypt`: KDF, wrap/blob/state AEAD, re-wrap on password change,
  wipe, server-key load/create; S-30.1 crypto tests (independent KDFs, password
  change preserves the data key, wipe, AAD binding).
- `internal/model`: atomic patch apply with upfront validation; UID and VERSION
  are not patchable.

### Not yet included

Storage, sessions, transport, the request layer, and everything above them.
`cmd/carrel` still serves only `/healthz`.

## v0.0.3 — 2026-09-04

Three spec corrections, and `view` follows them.

### For consumers

- **Breaking (`view`).** `EventPage.Title` and `TaskPage.Title` are now
  `EventPage.Summary` and `TaskPage.Summary`, JSON key `summary`. v0.0.2
  worked around the collision with `Shell.Title` by moving the key alone
  (`recordTitle`) while keeping the Go name; C-6.5 now renames the field
  itself, so the key is the ordinary lowercase-first one again and the
  fixture rule in `docs/spec/fixtures/README.md` needs no exception.
  `Summary` is what the same datum is already called in `AgendaRow` and
  `TaskRow`.
- `ImportStep`'s constants are unchanged here (`ImportStepSource`,
  `ImportStepPreview`, `ImportStepReport`) — C-8.3 now spells them the same
  way, so the workaround noted under v0.0.1 is no longer a divergence.

### Spec

Changed together with the code, as C-3.3 requires:

- **C-6.1** gains the rule the two renames follow: a page field never
  reuses the name of a field of the `Shell` it embeds. Both land in one
  flat object (C-3.2), where one would silently displace the other.
- **C-6.5**: `EventPage.Title` and `TaskPage.Title` → `Summary`.
- **C-8.3**: `ImportStep`'s constants were named `ImportSource`,
  `ImportPreview`, `ImportReport`, colliding with the struct types C-6.6
  declares for the same screen — the block could not compile. Renamed to
  `ImportStepSource`, `ImportStepPreview`, `ImportStepReport`.
- **`fixtures/README.md`** listed `section` among the fields `Shell`
  flattens onto a page. C-6.3 has no such field and never did.

### Included

- `TestEnumValuesMatchSpec` now compares constant *names* as well as
  values, which C-8.3's collision had made impossible.
- `TestTypesMatchSpec` reads the spec whether the working copy has LF or
  CRLF endings. It silently found nothing under CRLF before, and a test
  that finds nothing passes.

## v0.0.2 — 2026-09-04

Two wire-format corrections in `view`, and gates that hold the package to
the spec.

### For consumers

- **Breaking (`view`).** `EventPage.Title` and `TaskPage.Title` now carry
  the JSON key `recordTitle`, not `title`. Both pages embed `Shell`, which
  also declares `Title`; under one key `encoding/json` resolved the
  collision by depth, so the shallower record title silently shadowed the
  document title and `Shell.Title` never reached the wire. The Go fields
  stay named as C-6 names them — only the key moved, and only on these two
  pages. A fixture for either screen must use `recordTitle`.
- **Breaking (`view`).** `SearchBox.Whole` lost `omitempty`: a nil `Whole`
  now encodes as `"whole": null` instead of dropping the key. This is the
  rule `docs/spec/fixtures/README.md` already states — `null` where C-6 has
  a pointer, a missing key where it does not — and `Whole` was the one
  pointer in the package that broke it. The three existing fixtures gained
  `"whole": null`.
- Nothing else about the seam changed: same types, same fields, same enum
  values.

### Included

- `spec_test.go`: parses the fenced Go blocks of `docs/spec/carrel-common.md`
  and compares them with this package's own declarations — every type in
  C-6/C-8/C-9 present with the same fields in the same order and the same
  types, nothing beyond them, and every C-8 enum carrying the same values.
  A field added here that no frame shows, or a spec edit not transcribed,
  fails the build rather than waiting to be noticed.
- `schema_test.go`: every field carries a lowercase-first JSON tag; no tag
  collides with one promoted from an embedded struct (the defect above); no
  pointer field is `omitempty`.

### Fixed

- `Shell.Title` reaching the client on `EventPage` and `TaskPage`.
- `SearchBox.Whole` encoding absence as a missing key rather than `null`.

## v0.0.1 — 2026-09-04

Module skeleton and the `view` package.

### For consumers

- Add `carrel.dev/carrel/server` as a module dependency (`go 1.27`) once a
  client needs it. Import only `carrel.dev/carrel/server/view` — every
  `internal/*` package is off limits, including from `cmd/carrel` once a
  client exists.
- `view` carries the view-model schemas from C-6, the state dictionaries
  from C-8, and the error dictionaries from C-9 of
  `docs/spec/carrel-common.md`, transcribed field-for-field as Go structs.
  JSON tags are camelCase; `Shell` and `Plain` are embedded anonymously and
  flatten onto the enclosing page, matching the fixture layout.
- Run `go run ./cmd/carrel` for a bare skeleton that answers `GET /healthz`
  on `CARREL_BIND`/`CARREL_PORT` (default: all interfaces, port `8080`).
  There is no store, session, or routing yet — no other route answers.

### Included

- `go.mod`, module `LICENSE` (AGPL-3.0-or-later), and the product's root
  `LICENSE` (same text).
- `internal` independence gate: `carrel/server` does not import
  `carrel.dev/typecase` anywhere in its dependency graph
  (`nodep_test.go`, `go list -deps`).
- `cmd/carrel`: listens, serves `/healthz`, and does not crash against an
  empty/nonexistent data directory — there being no store yet to point at
  one, "first run" here means "starts with nothing on disk."
- `view` package: every type from C-6.2 through C-6.9, C-8, and C-9.
  Fixture test (`fixtures_test.go`) loads every screen under
  `docs/spec/fixtures/**/*.json`, decodes it into its view model with
  `json.Decoder.DisallowUnknownFields`, then compares the fixture and the
  re-encoded model as decoded JSON trees via `cmp.Diff` (not a
  byte-for-byte string compare).

### Known spec inconsistency

C-8.3 names `ImportStep` constants `ImportSource`, `ImportPreview`, and
`ImportReport` — but C-6.6 already defines struct types `ImportSource` and
`ImportReport` for the same screen. Go cannot have a type and a constant
share one identifier in the same package, so the constants are named
`ImportStepSource`, `ImportStepPreview`, and `ImportStepReport` here
instead. **Resolved in v0.0.3:** C-8.3 now names them the same way.

### Not yet included

Everything past the module skeleton and `view`: configuration, crypto,
storage, sessions, transport/guard/discovery/model, the request layer,
users, providers, the S-11…S-26 features, davloom, delivery, and the S-30
acceptance gates. See [docs/plan/server.md](../../docs/plan/server.md).
