# Changelog

All notable changes to the Carrel server are documented here. Numbers like
`v0.0.N` name releases in this file only — they are not git tags unless an
entry lists **Tagged:**. This file tells consumers what to do, not what the
authors did.

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
