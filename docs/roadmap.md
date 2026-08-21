# Roadmap

What is not built yet, in the order it matters, and — just as usefully — what will not be built.

Everything in the main scope of v1 is implemented: the framework, transport and discovery, contacts, calendar, tasks, notes, the unified view, cross-source search, duplicates, and WebDAV files with attachments. What is left before a v1 tag is a person's judgement rather than code.

Sources: §23 of [carrel-spec.md](carrel-spec.md) for the features, §25.6 for the order in which they are worth showing anyone. The per-stage implementation plans were removed once every stage was done; what they still carried and the code did not is in the gaps section below, and what was worth keeping of their working practice is in [development.md](development.md). Active implementation plans for features not yet built live in [plans/](plans/) and are deleted after closeout. The order in which everything below is actually built — together with the redesign the mockups ask for — is [plans/global-plan.md](plans/global-plan.md).

---

## Before v1 can be tagged

**Manual acceptance** ([manual-acceptance.md](manual-acceptance.md)). Five checks block the tag, and they block it because nothing automated replaces them:

- A note round-tripping through **jtx Board** and through **Evolution**, keeping the `X-` properties those clients write. The compatibility claim of §23.9 is about how real clients behave, and a fake server agreeing with us proves nothing.
- **`docker compose` from an empty volume** through a whole scenario, with the read-only filesystem, dropped capabilities and non-root user actually in force.
- **Debug logs holding no password or token** after that scenario. A test asserts about lines it thought to look at; a person reads what is there.
- **Pasting a screenshot into a note taking a couple of seconds.** §23.10 says outright that if this is slow the feature is dead, and no assertion measures whether something felt instant.

**A live end-to-end pass of attachments** against a real Baikal plus a real WebDAV server, through the browser. The pieces are each verified live now: discovery against Baikal, and listing, streaming, range requests, conditional creates and folder creation against SFTPGo. What the integration tests do not cover is the two combined through the interface — attaching to a note on Baikal with the file landing on the separate WebDAV account — which is check **P5** above.

### Already cleared

- **`go test ./...`** — green, and the gate for every commit.
- **`go test -race ./...`** — green across all seventeen packages, including the goroutine-leak and race gate of §21 in `internal/fanout`: concurrent snapshots against retries and subscribers, cancellation, and a cancelled task opening no further connections. Needs gcc in `PATH`; `-race` refuses to build without one rather than quietly skipping, so a run that cannot start says so.
- **`go test -tags=integration ./...`** — green against a live Baikal and a live SFTPGo. Running it for the first time found three bugs, all in Carrel: discovery could not reach a principal under a base path, a download was truncated at the response ceiling, and a create trusted a precondition SFTPGo ignores. Each now has an offline regression test.

> The lesson from that last one is worth keeping in view. A fake DAV server agrees with whatever the code assumes — ours put DAV at the root and honoured every header, which is exactly why two of the three were invisible. And a test tier that skips silently when unconfigured is a tier nobody notices is not running: `-tags=integration` reported `ok` in two seconds for as long as nobody had credentials loaded.

---

## Gaps in what is built

Found by reading the stage plans back against the code they describe, and worth being honest about rather than discovering later. The first five are requirements from the main specification — three of them carried from plan to plan as "a later strengthening" and never landed, and two that were never written down at all until the mockups went looking for them; the rest are consequences of decisions that were correct at the time.

### Requirements not met

**The row menu of §13.** The rest of the narrow-screen list was done in wave 1.17: the source rail became a full-screen panel that closes with «Apply», tap targets reached 44 px, the bottom bar carries the five sections, records fit two lines, and there is no properties panel on a phone — a row opens the record itself. What the mockups also draw and the code does not have is the `⋯` menu on the row that was supposed to take the panel's place for operations. Until it exists, an operation on a phone means opening the record first. Small, and named here rather than quietly dropped.

**The note's edit bar is not one line (mockups §6, §7.4).** Every other toolbar retreats by rank now — what does not fit goes under `⋯` and the bar stays a single line at any width. `.m-editbar` is the one that still wraps: the markup marks nothing `is-2nd` and the screen has no `⋯` to put it in, so on the note's own column the right-hand group (draft label, text-width switch, Focus) sits on a line of its own above the buttons. The library gave it the symmetric padding of a bar; the retreat needs the note screen's markup, which is why it is here and not in the component.

**A full DAV server test in the administration panel (§6).** *(done in wave 3.3)* Discovery at `/admin/dav` stays read-only and remains the default. A second stage — **Run full test**, with an explicit consent checkbox — runs after discovery succeeds: short-lived objects in writable collections, the operations Carrel actually performs (`calendar-query` at Depth 1, multiget, conditional create, ETag conflict as 412, CardDAV and WebDAV reads), verification, cleanup, step-by-step trace, and an audit entry `dav_exercise`.

**Creating, renaming and deleting collections (§10.1).** *(done in wave 3.1)* Carrel can now create calendars and address books on a connected server (`MKCALENDAR`, extended `MKCOL`), rename and recolour them (`PROPPATCH`), and delete them with confirmation by typing the collection name. Entry points: rail-foot buttons on merged section views (including an empty rail), **New collection** on each account card in Connections with a Calendar / Address book switch, and Rename/Delete behind ⋯ on each collection row there. The delete sheet lists published links, backup jobs and davloom devices even while those rows stay empty for wave 4.

**A process-wide memory ceiling for the cache, and body-first eviction (§12).** *(done in wave 3.2)* Object bodies are counted in bytes and evicted LRU before the collection's ETag map, so pressure no longer throws away the cheap thing that saves a deep `PROPFIND`. Across every live session the process holds a single byte ceiling (bodies and thumbnails; maps stay), with LRU reaching across users. Per-session collection, ETag and thumbnail limits are unchanged.

### Built past the spec

**Escrow withdrawal (§5.4).** Done in wave 0.1: the administrator can no longer forbid withdrawing a deposited copy; a `forbid_opt_out` flag in older state is ignored.

### Consequences of earlier decisions

| | What is missing | Why it was left |
|---|---|---|
| **Export from several collections at once** | Contacts and calendar export one collection each, the calendar optionally over a date range | Stage 4 planned "one event, an agenda range, or the selected collections". The third case is missing, which matters most for a backup taken by hand — and is subsumed by backup proper (§23.3) |
| **Multi-platform images, CI, `ghcr.io`** | The Dockerfile and compose file are complete and build locally. Nothing publishes `linux/amd64` and `linux/arm64` images, and there is no CI at all | Explicitly out of scope from stage 1 onward. §18 describes the intended arrangement: GitHub Actions building for the mirror and publishing to `ghcr.io`, with secrets not duplicated into Gitea |
| **Attachments on tasks** | `ATTACH` is modelled on events and notes. A VTODO keeps the property untouched and shows it among its foreign properties, but there is no attach or detach on a task card | §23.10 names events and notes. The model work is done; this is one card's worth of interface |
| **Inline attachments** | An `ATTACH` carrying base64 is shown, named and never rewritten — as §23.10 requires — but cannot be opened or downloaded | Decoding it means holding the object's body in memory to serve a file, which is the one thing the file section avoids. A person can still read it in any client that supports it |
| **Attachment size and type** | Taken from the `SIZE` and `FMTTYPE` parameters the writing client set, not measured | Measuring means one request per attachment on every page that lists them. A missing parameter shows as a missing detail rather than a wrong one |
| **Markdown export of inline attachments** | An export carries attachment links; base64 data is not carried | A note file with a megabyte of encoded image in its front matter is not a note file. It is stated in the export rather than left to be found |
| **Renaming and moving files** | The transport has `MOVE` and the provider does not use it | Nothing in §23.10 needs it, and §23.10 is explicit that this is not a file manager |
| **Folder listings are not paginated** | A folder past the configured ceiling says so and shows the first N | The alternative is `PROPFIND` paging, which DAV does not really have. The ceiling is a setting |
| **Original time zone of an event** | One global zone, as §10 fixes for v1 | §23.8: source zone beside the time when it differs; data is in `DTSTART` | Done |
| **Per-collection sync state** | §23.8 asks for a line saying when a collection was last read and last changed on the server | Source block in detail panels and Connections | Done |

Both §23.8 items are worth doing before anything in the next section: they are display over data Carrel already computes, they are the two cheapest things left on this page, and §23.8's whole argument is that showing what is already known is what distinguishes this from clients that stay silent.

---

## Desktop application (Wails, §18)

Native wrapper for Windows and Linux — own window and server lifecycle; PWA remains for the browser.

**Stack:** Wails v2 (not Tauri). **v1 platforms:** Windows + Linux; macOS is not in v1.

**Modes:** Remote (webview on a URL) or Local (sidecar `carrel`); chosen at first launch per OS-user. Switch modes via **Sign out** → onboarding again.

**Data:** `CARREL_DATA_DIR` always in the OS-user profile. Sidecar binary next to the desktop app in the install directory; an admin install shares one sidecar copy on the machine.

**Sidecar:** downloaded only for Local, or optionally during install («Download server component»). Dynamic port; `CARREL_BIND=127.0.0.1` for local desktop.

**Tray:** user setting — close window hides to tray (server keeps running) or stops the sidecar.

**Not in scope:** DAV on localhost, macOS v1, offline mode.

**Implementation plan:** [plans/desktop-wrapper.md](plans/desktop-wrapper.md) (temporary; removed after closeout).

**Acceptance:** Win+Linux; Remote/Local; single instance per user; fan-out SSE or polling in webview; installer server checkbox. See [manual-acceptance.md](manual-acceptance.md) P-desktop.

---

## Next, in order

The order below is value first, and follows §25.6 rather than the engineering order that got us here. The desktop application is a parallel track — its order relative to the items below is not fixed.

### 1. New device screen (§23.1)

The single largest saving of effort per line of code in the whole specification. After discovery Carrel knows everything that manual client setup wastes time on: the working base URL, the principal, the home-sets, exact collection paths and names, and the credentials.

- **Apple** takes a `.mobileconfig` — an XML plist with CalDAV and CardDAV payloads. One profile for every connected account at once: the person opens it, confirms, and every calendar and address book appears. Most of the value, and by volume it is generating XML.
- **Android** has `davx5://user@host/path/` as a QR code, which opens DAVx5 with the URL and username filled in. The password cannot travel that way — DAVx5's own documentation forbids credentials in the URL — but the two fields people get wrong are covered. `caldavs://` and `carddavs://` work for anything else compatible.
- **Thunderbird** has no import mechanism and needs none: a screen of copy buttons per collection removes the fiddling.

The security requirements are not optional and are the reason this is not smaller than it looks: an Apple profile contains passwords in cleartext, and it is a file that ends up in Downloads and in messengers. So it is generated only after the Carrel password is entered again, served over a one-time link with a short lifetime, never written to the server's disk, preceded by an explicit warning about what the file contains, and recorded in the audit log.

### 2. Backup to WebDAV (§23.3)

Carrel reads from one server and writes to another, so §2 stays intact: nothing is stored here. The WebDAV transport and streaming `PUT` already exist.

The archive is raw `.ics` and `.vcf` **as they are**, in `account/collection` directories, with a manifest recording the date, the version, the collections and the ETags at the time. A copy that only its own program can restore is a bad copy.

Backup is a list of **jobs**, not a switch in the settings: each with its own sources, destination, trigger and retention. Work calendars daily to a company server and personal things weekly to a home disk are two jobs, not a compromise between them.

In order of increasing compromise, and the order they should be built in:

1. **On demand.** The person is signed in, the key is unwrapped, the archive goes where they chose. No concession to the security model at all.
2. **On sign-in.** If the last copy is older than N days, offer or take one in the session that is already open. Someone who signs in weekly gets weekly copies for free, and no secret is stored.
3. **External trigger.** `POST /api/backup/run` with an app-password whose scope is exactly one job and starting it: not reading collections, not settings, not the neighbouring job. Asynchronous, rate limited, audited, and refusing a second run while one is going. The scheduler is `cron` or anything else, and the secret lives on the machine that holds the schedule. Plus `GET /api/backup/{id}/download` for people with nowhere to write, streamed and never landing on the server's disk.

   What the interface hands over is the finished command, not the password — that is what most people came for, and §23.3 fixes the answer shapes accordingly: the id as a bare line, the status as one word, so a `sh` script on somebody else's machine needs no `jq`. A job that also hands the file back needs **two** steps, and gets a short script rather than a line; a download-only job needs one request that makes the archive and streams it in the same breath.

Non-negotiable whatever the trigger: the file is dated rather than overwritten and N are kept; what was written is read back and checked, because a backup that failed quietly is worse than none; a failure on one collection marks it in the manifest and continues; and **restore is part of the feature**, because a copy with no tested restore is a feeling and not a protection — restorable into an empty instance, from a file, without a job.

Encryption under a separate password, distinct from the login one, is **on by default** rather than unconditional: the target may be somebody else's hosting, but a copy landing where nobody but the owner ever goes may reasonably be left open, and then any archiver reads it without Carrel. §23.3 states the consequence in the same breath — with encryption off, whoever holds the trigger password can pull the archive down in the clear.

### 3. Read-only publication (§23.4)

One mechanism for two directions — **outbound** (your own) and **inbound** (someone else's feed) — with the same constraints: read-only, no local content storage, served on demand.

**Outbound: your own via a secret link.** A secret link on one object or one collection, built from a live read of a connected upstream, openable without signing in:

| Object | In the browser | Download |
|---|---|---|
| Address book (CardDAV) | contact list | `.vcf` |
| Calendar (CalDAV) | agenda for a range | `.ics` |
| Note (VJOURNAL) | rendered page | Markdown or raw `.ics` |
| File (WebDAV) | image — inline preview; PDF — served as `application/pdf`, viewed in the browser; text (`text/plain`, `.txt`) and Markdown (`.md`) — body on the publication page; everything else — name and size | streamed with `Content-Type` |
| Folder (WebDAV) | listing with a format icon per file | — |

For files Carrel does not build a universal viewer: images inline, PDF to the browser, text and Markdown read from the stream into a simple HTML page — no new persistence model, only a size ceiling and a template like any other page in this feature. Text goes in a `<pre>` with escaping; Markdown gets a simple safe HTML render (headings, lists, emphasis, links, code), with raw HTML in the source escaped. A folder is a list of names with a type icon on each row — no content preview.

**Inbound: subscribe to an external calendar or address book.** The person points at an external feed URL — `.ics` for a calendar, `.vcf` or equivalent for an address book — and gets the same read-only access: view in the browser and/or a secret link outward. The subscription has nowhere to live between sessions: Carrel fetches the feed on every access, and caching it on disk would be exactly the local content persistence §2 forbids. In the owner's session — view on demand; outward — through the same secret-link mechanism as for their own collections. In proxy mode (§23.2) the feed additionally appears to DAV clients as one more read-only collection they refresh themselves.

**Shared requirements:** no state and no stored content — no concession to §2 or §4. Token of at least 32 random bytes stored as a digest, one-click revocation, configurable lifetime, active links listed in the profile with last access time, `Cache-Control: private`, `X-Robots-Tag: noindex`, its own rate limit. One object, one collection or one external feed per link — never a whole account. For external sources: timeout and response size limit; an unavailable source must not take down other collections (§17); source marked external and read-only; SSRF rules (§24.2) apply to the feed URL like any other outbound request.

### 4. Sending by mail (§23.5)

Two steps on one mechanism, and the second does not stand without the first. Both reuse the SMTP client that already sends invitations (§5.3) — what is missing is permission to use it for something other than a token link.

1. **A publication link by mail.** A button beside the link §23.4 already issues: recipient, an optional line, send. The letter carries the link and nothing else; revocation and lifetime stay where §23.4 put them, and sending creates no state to revoke.
2. **A plain letter with things attached.** Contact, address book, event, calendar range, note, task, file — attached; folder — always a link. Above a ceiling the attachment becomes a §23.4 link instead, and the interface says which is which *before* sending rather than leaving the recipient to report it. The recipient field completes from the connected address books — the same fan-out search as §16, narrowed to cards that have an address — because a client holding the address books and still sending you to another screen for an address would be absurd. A suggestion is an address rather than a person, so two addresses make two rows; typing an address by hand stays the ordinary case; and the list names any book that failed to answer, or a missing person reads as «you have no such contact» when the truth is that a server was down.

The interesting part is not the letter, it is the **From address**. The instance owns its domain, so the question is not whose address to take but which form the relay will let out — and that is a two-position admin setting: personal `<login>@<domain>`, or the service `noreply@<domain>` alone, which is the default. The envelope sender stays the configured relay address either way, so SPF passes on it regardless of what the header says. The administrator chooses because the administrator knows the relay; Carrel signs no DKIM and touches no DNS, and the test-message button of §5.3 answers the question empirically for whichever form was picked. Only one local part can collide with a login, and it becomes a **forbidden login** rather than a special case: `noreply` is the instance's service address whatever the setting says, so no such account may exist at all. Quietly swapping one person's From is the kind of behaviour later explained from logs; refusing the login at validation is cheaper and more honest, and where such an account already exists, enabling personal addresses refuses and names it — Carrel cannot rename a login, and that is the administrator's call to make deliberately. `admin` needs no rule: the login is ordinary, the domain belongs to the instance, and where `admin@domain` points is decided by the same person who turns personal addresses on.

Two smaller consequences. The profile has a login and an address but **no display name** — «sender name from the profile» means adding that field first, and it lands in two places from the one field: the `From` header and the signature at the foot of the letter. That is not field economy. Under the service From the header reads `noreply@domain`, and the signature is then the only place saying who wrote. And a calendar attachment goes with `METHOD:PUBLISH`; `REQUEST` would make it an iTIP invitation, which is on the list of things that will not happen.

`Reply-To` is the profile address **only once confirmed**, and confirmation turns out not to be a state the record can express. There is an address, a pending address and a change token, but no «this one was proved» flag, and it was never needed: three of the four ways an address arrives confirm it by construction — an invitation by mail, self-registration, a change of address — and the fourth, an invitation by **link**, lets the person type any address at activation and stores it unchecked. Until now the worst case was an undelivered notice. Here it is a `Reply-To` steering the recipient's reply at an address nobody proved, so the flag gets added, a link activation counts as unconfirmed, and it is confirmed by the same mailed link as a change. With no confirmed address the letter ends with a line saying not to reply, because Carrel parses no inbound mail at all.

The rest is the abuse surface, and it is why this is a feature with a switch: off by default, ceilings on attachment size, message size, recipients per letter and letters per user per day, `CR`/`LF` stripped from the subject, sender name and recipient addresses so nobody appends a `Bcc:`, and the audit recording who sent how many with what kind of attachment — not the addresses, not the subject, not the contents. One constraint is structural rather than defensive: SMTP wants the whole message, base64 inflates it by a third, and this is the only place in Carrel where a file body is held whole in memory. The ceiling is the answer; a temporary file is not.

### 5. davloom — DAV proxy mode (§23.2)

The killer feature, and the largest piece of work in the roadmap. One account in DAVx5 or Thunderbird instead of five: Carrel becomes the single connection point and fans out to every configured server.

It ships as a **run mode of the same build**, not a second product: `carrel serve`, `carrel proxy`, or both. Two processes from one image is the recommended deployment, because the proxy parses untrusted bodies from machines and should not share an address space with unwrapped keys, and because a proxy crash should not sign out everyone using the web interface.

Most of the traffic is proxied blind — `REPORT`, `sync-collection`, `calendar-query`, `PROPPATCH`, `GET`, `PUT`, `DELETE` — with the authorisation header swapped and the upstream's answer passed through. Baikal or Radicale implements the semantics. Two things cannot go blind:

- **hrefs in bodies.** A multistatus carries upstream paths; handed on unchanged, the client's next request goes around the proxy. Solved by a mirrored path scheme and a streaming prefix substitution in both directions, without parsing XML and without buffering a body.
- **the root.** No upstream owns `/`. `.well-known`, `current-user-principal`, the principal and the home-sets are synthetic levels the proxy generates, and they are precisely what glues several servers into one account. `PROPFIND Depth: 1` on a synthetic home-set is a fan-out with the timeouts and degradation of §16 and §17 — which already exists. Exactly three synthetic levels; below them it is dumb proxying.

Authentication is per-device app-passwords, each with its own DEK wrapper, so revoking a device is deleting its wrapper and changing the Carrel password leaves the devices working — which the interface has to say, or people will assume otherwise. Nothing decrypted stays in memory between requests; a short KEK cache exists for the burst DAVx5 makes when several local edits happen in a row, not for the scheduled sync, which is hours apart.

The real risk is not the code: an implementation that follows the RFC and one that DAVx5, Thunderbird and Apple Calendar all digest are noticeably different, and that tail often costs more than the feature. Start read-only against one DAVx5.

---

## Undecided

Three ideas with a real price, recorded rather than planned. None is accepted.

**Counts beside every collection and section** — the mockups write a number next to Contacts, Calendar, Tasks and Notes in the rail, and next to each collection under them. To show one, the number has to come from somewhere, and every source is a compromise: asking each collection means a fan-out over collections the person did not tick, which §16 calls the most expensive place in the architecture and tells us not to spend twice; using only what the cache already holds means the numbers appear and disappear as the cache evicts, which is worse than no number; a cheap `PROPFIND` per collection is still one request per collection on every page load. The number is genuinely useful — it is how you notice that an address book you thought was empty has 300 cards in it — so this is not a refusal. It waits for §12, where cache ceilings and eviction order are reopened anyway, because the honest answer probably lives there: a count is worth showing exactly when it is already known, and the question is how long "already known" lasts. Found by the visual acceptance of wave 2.5.

**Parsing service exports** (§23.7) — not standard `.vcf` and `.ics`, which already work, but what Google Takeout, iCloud and phone exports actually produce: non-standard group `X-` properties, photos in several representations, several cards glued into one file, encodings, escaping, nested archives. This determines whether people arrive: somebody migrates once, and if the import stumbles they do not come back. The price is that the work never finishes — export formats change without notice and each source is its own set of special cases — and it needs real exports to test against. The alternative is accepting standard files honestly and recommending a converter.

**A writable link** (§23.7) — the same mechanism as read-only publication, but edits go through the owner's upstream account. Every change will look on the server as if the owner made it; upstream logs cannot tell who edited. That is not shared access and must not be called it in the interface. Real sharing is a server-side ACL. The question is whether this is needed at all, or read-only publication closes the real scenario.

---

## Not going to happen

Saying no is part of the design, and each of these is a decision with a reason rather than a gap waiting for time.

- **A calendar grid, month view, drag and drop.** The agenda is a list. A `REPORT` with a time range returns exactly what a list needs, and the grid is where every other web client spends its complexity.
- **Editing single occurrences of a recurring event.** `RECURRENCE-ID`, `EXDATE` and `THISANDFUTURE` are out of v1: whole series only. Not ruled out forever, but not cheap either, and the properties survive an edit meanwhile.
- **iTIP scheduling.** Attendees and participation status are shown, never negotiated.
- **Local persistence of content.** No offline mode, no background sync, no service worker caching collections. The DAV server is the source of truth; when it is down Carrel says so instead of showing something stale.
- **A database.** No Postgres, no SQLite, no Redis. One encrypted file.
- **Node, npm, a bundler, a CDN.** Go and htmx, with htmx vendored into the repository.
- **Localisation.** English only. The infrastructure for translations that do not exist is an extra layer in every template that buys nothing; if an audience that needs it appears, extracting the strings is mechanical work and better done knowing the real volume.
- **Password recovery.** Without the password there is no key. The login page names the two real options — a destructive reset, or escrow decided in advance — instead of offering a link that cannot work.
- **A file manager.** Previews, permissions, renaming trees at scale. The file section serves attachments; a file manager is a different product with different competitors.
- **Cleaning up the attachments folder.** No quotas, no garbage collection, no deleting orphans. It is the person's storage, and §23.10 is explicit that taking responsibility for somebody else's contents is not Carrel's job.
- **Being a mobile app.** The narrow layout is a lifeboat: read the schedule, find a number, write a note. Duplicate groups, merging, photo cropping, the administration panel and account setup are deliberately not optimised for it. For comfortable phone use the answer is DAVx5 and jtx Board, and saying so is more useful than pretending.
- **A wastebasket for deleted objects.** DAV servers have none, and backup (§23.3) closes the same scenario without breaching §2.
- **Carrel as a hosted service.** §5 already supports several users on one instance; positioning Carrel as a public multi-tenant offering is out of scope.
- **An internal backup scheduler with a stored password.** Scheduling belongs on the machine that holds the secret — `cron`, GoSentry, a Gitea job — via the external trigger (§23.3). Storing the password on the volume beside its wrapper is the compromise §4 deliberately avoids.
- **Protecting against the host operator.** DAV needs the password in cleartext for Basic authentication, so it is in memory during a request. This is inherent to the protocol.
