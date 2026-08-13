# Roadmap

What is not built yet, in the order it matters, and — just as usefully — what will not be built.

Everything in the main scope of v1 is implemented: the framework, transport and discovery, contacts, calendar, tasks, notes, the unified view, cross-source search, duplicates, and WebDAV files with attachments. What is left before a v1 tag is a person's judgement rather than code.

Sources: §23 of [carrel-spec.md](carrel-spec.md) for the features, §25.6 for the order in which they are worth showing anyone. The per-stage implementation plans were removed once every stage was done; what they still carried and the code did not is in the gaps section below, and what was worth keeping of their working practice is in [development.md](development.md).

---

## Before v1 can be tagged

**Manual acceptance** ([manual-acceptance.md](manual-acceptance.md)). Five checks block the tag, and they block it because nothing automated replaces them:

- A note round-tripping through **jtx Board** and through **Evolution**, keeping the `X-` properties those clients write. The compatibility claim of §23.9 is about how real clients behave, and a fake server agreeing with us proves nothing.
- **`docker compose` from an empty volume** through a whole scenario, with the read-only filesystem, dropped capabilities and non-root user actually in force.
- **Debug logs holding no password or token** after that scenario. A test asserts about lines it thought to look at; a person reads what is there.
- **Pasting a screenshot into a note taking a couple of seconds.** §23.10 says outright that if this is slow the feature is dead, and no assertion measures whether something felt instant.

**`go test -race ./...` on a machine with a C compiler.** The suite passes without it; the goroutine-leak and race gate of §21 needs it, and the development machine has been without gcc in `PATH` for part of this work.

**A live end-to-end pass of attachments** against a real Baikal plus a real WebDAV server, through the browser. The pieces are now each verified live: discovery against Baikal, and listing, streaming, range requests, conditional creates and folder creation against SFTPGo. What the integration tests do not cover is the two combined through the interface — attaching to a note on Baikal with the file landing on the separate WebDAV account — which is check **P5** in [manual-acceptance.md](manual-acceptance.md).

> Running these tests for the first time found three bugs, all in Carrel: discovery could not reach a principal under a base path, a download was truncated at the response ceiling, and a create trusted a precondition SFTPGo ignores. Each now has a regression test offline. The lesson is worth keeping in view: a fake DAV server agrees with whatever the code assumes, and a test tier that skips silently when unconfigured is a tier nobody notices is not running.

---

## Gaps in what is built

Found by reading the stage plans back against the code they describe, and worth being honest about rather than discovering later. The first four are requirements from the main specification that were carried from plan to plan as "a later strengthening" and never landed; the rest are consequences of decisions that were correct at the time.

### Requirements not met

**A process-wide memory ceiling for the cache (§12).** The cache has per-session limits — collections, ETag entries, thumbnail bytes and count — and they work. What §12 also asks for is a ceiling across *all* sessions, with LRU eviction reaching across users, precisely so that ten people with large address books cannot together exhaust the container's memory. There is no such ceiling: ten sessions get ten times the per-session allowance. On a single-user instance this is invisible; on anything shared it is the difference between a slow instance and one the kernel kills. It is also the one item on this list that becomes load-bearing the moment multi-tenancy (§23.5) is taken seriously.

**Byte accounting for object bodies, and eviction in the right order (§12).** Thumbnails are counted in bytes and evicted against a byte ceiling. Object bodies are not counted at all — a collection is evicted whole, taking its ETag map with it. §12 is explicit that the order should be the other way round: bodies go first and ETag maps are held longer, because the maps are small and save the most. Today evicting one collection under pressure throws away the cheap thing along with the expensive one, and the next visit pays a deep `PROPFIND` it did not need to.

**The public self-registration form (§5.2).** The setting exists, is off by default, and is stored and audited correctly. The administration panel offers the checkbox with the label *"Allow self-registration (no public form in this stage)"* — which is honest, and has been that label since stage 1. There is no public form behind it: enabling the flag changes nothing a visitor can see. Either the form gets built or the checkbox should say so more plainly than in parentheses.

**The narrow-screen requirements of §13.** There is a `@media (max-width: 640px)` block, and it does three of the things asked for: inputs at 16 px so iOS does not zoom on focus, one column instead of two, and a stacked top bar. The rest of §13's list is not done — the source rail does not become a slide-out panel, it simply stacks above the content, so on a phone every screen with sources starts with a list of checkboxes to scroll past; tap targets are not brought to 44 px, and the source checkboxes and 3 px colour strips are still the mouse-sized ones §13 names specifically; photo cropping on a narrow screen is the full pan-and-zoom rather than the centre crop §13 asks for. The lifeboat floats, but it is not the lifeboat that was specified.

### Consequences of earlier decisions

| | What is missing | Why it was left |
|---|---|---|
| **A service worker** | The PWA manifest and icon are embedded and the interface installs as an app. There is no service worker at all | §13 asks for a minimal one covering the shell and static assets, and forbids it caching collection contents. What was built is the safe half of that: nothing to cache wrongly. The cost is that an installed app with no network shows a browser error rather than a shell |
| **Export from several collections at once** | Contacts and calendar export one collection each, the calendar optionally over a date range | Stage 4 planned "one event, an agenda range, or the selected collections". The third case is missing, which matters most for a backup taken by hand — and is subsumed by backup proper (§23.3) |
| **Multi-platform images, CI, `ghcr.io`** | The Dockerfile and compose file are complete and build locally. Nothing publishes `linux/amd64` and `linux/arm64` images, and there is no CI at all | Explicitly out of scope from stage 1 onward. §18 describes the intended arrangement: GitHub Actions building for the mirror and publishing to `ghcr.io`, with secrets not duplicated into Gitea |
| **Attachments on tasks** | `ATTACH` is modelled on events and notes. A VTODO keeps the property untouched and shows it among its foreign properties, but there is no attach or detach on a task card | §23.10 names events and notes. The model work is done; this is one card's worth of interface |
| **Inline attachments** | An `ATTACH` carrying base64 is shown, named and never rewritten — as §23.10 requires — but cannot be opened or downloaded | Decoding it means holding the object's body in memory to serve a file, which is the one thing the file section avoids. A person can still read it in any client that supports it |
| **Attachment size and type** | Taken from the `SIZE` and `FMTTYPE` parameters the writing client set, not measured | Measuring means one request per attachment on every page that lists them. A missing parameter shows as a missing detail rather than a wrong one |
| **Markdown export of inline attachments** | An export carries attachment links; base64 data is not carried | A note file with a megabyte of encoded image in its front matter is not a note file. It is stated in the export rather than left to be found |
| **Renaming and moving files** | The transport has `MOVE` and the provider does not use it | Nothing in §23.10 needs it, and §23.10 is explicit that this is not a file manager |
| **Folder listings are not paginated** | A folder past the configured ceiling says so and shows the first N | The alternative is `PROPFIND` paging, which DAV does not really have. The ceiling is a setting |
| **Original time zone of an event** | One global zone, as §10 fixes for v1 | §23.8 has the cheap remedy: show the source zone beside the time when it differs. The data is already in `DTSTART`. Not yet done |
| **Per-collection sync state** | §23.8 asks for a line saying when a collection was last read and last changed on the server | The cache already knows all of it; only the display is missing |

Both §23.8 items are worth doing before anything in the next section: they are display over data Carrel already computes, they are the two cheapest things left on this page, and §23.8's whole argument is that showing what is already known is what distinguishes this from clients that stay silent.

---

## Next, in order

The order below is value first, and follows §25.6 rather than the engineering order that got us here.

### 1. New device screen (§23.1)

The single largest saving of effort per line of code in the whole specification. After discovery Carrel knows everything that manual client setup wastes time on: the working base URL, the principal, the home-sets, exact collection paths and names, and the credentials.

- **Apple** takes a `.mobileconfig` — an XML plist with CalDAV and CardDAV payloads. One profile for every connected account at once: the person opens it, confirms, and every calendar and address book appears. Most of the value, and by volume it is generating XML.
- **Android** has `davx5://user@host/path/` as a QR code, which opens DAVx5 with the URL and username filled in. The password cannot travel that way — DAVx5's own documentation forbids credentials in the URL — but the two fields people get wrong are covered. `caldavs://` and `carddavs://` work for anything else compatible.
- **Thunderbird** has no import mechanism and needs none: a screen of copy buttons per collection removes the fiddling.

The security requirements are not optional and are the reason this is not smaller than it looks: an Apple profile contains passwords in cleartext, and it is a file that ends up in Downloads and in messengers. So it is generated only after the Carrel password is entered again, served over a one-time link with a short lifetime, never written to the server's disk, preceded by an explicit warning about what the file contains, and recorded in the audit log.

### 2. Backup to WebDAV (§23.3)

Carrel reads from one server and writes to another, so §2 stays intact: nothing is stored here. The WebDAV transport and streaming `PUT` already exist.

The archive is raw `.ics` and `.vcf` **as they are**, in `account/collection` directories, with a manifest recording the date, the version, the collections and the ETags at the time. A copy that only its own program can restore is a bad copy.

In order of increasing compromise, and the order they should be built in:

1. **On demand.** The person is signed in, the key is unwrapped, the archive goes where they chose. No concession to the security model at all.
2. **On sign-in.** If the last copy is older than N days, offer or take one in the session that is already open. Someone who signs in weekly gets weekly copies for free, and no secret is stored.
3. **External trigger.** `POST /api/backup/run` with an app-password whose scope is exactly one thing: starting a backup. Not reading collections, not settings, nothing else. Asynchronous, rate limited, audited, and refusing a second run while one is going. The scheduler is `cron` or anything else, and the secret lives on the machine that holds the schedule. Plus `GET /api/backup/{id}/download` for people with nowhere to write, streamed and never landing on the server's disk.
4. **A scheduler with a stored password.** Only if the three above prove insufficient, and knowing what it costs: the password would sit on the volume beside the wrapper it opens, so Argon2id protects nothing against whoever owns the host. Its real value is compartmentalisation and revocation, and the interface would have to say so without euphemism.

Non-negotiable whatever the trigger: the archive is encrypted under its own password, distinct from the login password, because the target may be somebody else's hosting; the file is dated rather than overwritten and N are kept; what was written is read back and checked, because a backup that failed quietly is worse than none; a failure on one collection marks it in the manifest and continues; and **restore is part of the feature**, because a copy with no tested restore is a feeling and not a protection.

### 3. Sharing: publish and subscribe (§23.4)

Three things of very different cost, and the value is in not confusing them.

- **Read-only publication** is nearly free and has no state: a secret link serves a collection as `.ics` or `.vcf` built from a live read. Token of at least 32 random bytes stored as a digest, one-click revocation, a configurable lifetime, the active links listed in the profile with when each was last used, `Cache-Control: private`, `X-Robots-Tag: noindex`, and its own rate limit because a public link will be guessed at. One collection per link, never a whole account. This closes a hole Baikal simply does not fill.
- **A writable link** is the same mechanism, and carries a warning that cannot be hidden: every edit will look on the server as if the owner made it, because it goes through the owner's account. That is not shared access and must not be called it in the interface. Real sharing is a server-side ACL, granted by whoever owns the data, and Carrel does not create accounts on Baikal.
- **Subscribing to an external calendar** is honest in the web interface only as a fetch on demand, because a subscription has nowhere to live between sessions and caching the feed on disk would be exactly the local content persistence §2 forbids. It becomes nearly free in proxy mode, where the feed is one more read-only collection the client decides when to refresh — so it belongs to davloom rather than here.

### 4. davloom — DAV proxy mode (§23.2)

The killer feature, and the largest piece of work in the roadmap. One account in DAVx5 or Thunderbird instead of five: Carrel becomes the single connection point and fans out to every configured server.

It ships as a **run mode of the same build**, not a second product: `carrel serve`, `carrel proxy`, or both. Two processes from one image is the recommended deployment, because the proxy parses untrusted bodies from machines and should not share an address space with unwrapped keys, and because a proxy crash should not sign out everyone using the web interface.

Most of the traffic is proxied blind — `REPORT`, `sync-collection`, `calendar-query`, `PROPPATCH`, `GET`, `PUT`, `DELETE` — with the authorisation header swapped and the upstream's answer passed through. Baikal or Radicale implements the semantics. Two things cannot go blind:

- **hrefs in bodies.** A multistatus carries upstream paths; handed on unchanged, the client's next request goes around the proxy. Solved by a mirrored path scheme and a streaming prefix substitution in both directions, without parsing XML and without buffering a body.
- **the root.** No upstream owns `/`. `.well-known`, `current-user-principal`, the principal and the home-sets are synthetic levels the proxy generates, and they are precisely what glues several servers into one account. `PROPFIND Depth: 1` on a synthetic home-set is a fan-out with the timeouts and degradation of §16 and §17 — which already exists. Exactly three synthetic levels; below them it is dumb proxying.

Authentication is per-device app-passwords, each with its own DEK wrapper, so revoking a device is deleting its wrapper and changing the Carrel password leaves the devices working — which the interface has to say, or people will assume otherwise. Nothing decrypted stays in memory between requests; a short KEK cache exists for the burst DAVx5 makes when several local edits happen in a row, not for the scheduled sync, which is hours apart.

The real risk is not the code: an implementation that follows the RFC and one that DAVx5, Thunderbird and Apple Calendar all digest are noticeably different, and that tail often costs more than the feature. Start read-only against one DAVx5.

### 5. Carrel as a service (§23.5)

Almost no code: §5 already isolates users, invites them and keeps administrators out of their data. What is missing is quotas — accounts and collections per user, and a per-user memory ceiling as well as the process-wide one, or one person with an enormous address book evicts everyone else — self-registration with email confirmation, terms and operator contact on the about page, and escrow transparency shown *before* somebody connects an account rather than after.

The honest limits have to be written into the terms, not just the README: the operator's instance holds users' DAV passwords encrypted and decrypts them in memory on every request, so an operator with process access can technically obtain them, and a restart signs everyone out.

---

## Undecided

Two ideas with a real price, recorded rather than planned. Neither is accepted.

**Parsing service exports** (§23.7) — not standard `.vcf` and `.ics`, which already work, but what Google Takeout, iCloud and phone exports actually produce: non-standard group `X-` properties, photos in several representations, several cards glued into one file, encodings, escaping, nested archives. This determines whether people arrive: somebody migrates once, and if the import stumbles they do not come back. The price is that the work never finishes — export formats change without notice and each source is its own set of special cases — and it needs real exports to test against. The alternative is accepting standard files honestly and recommending a converter.

**A wastebasket** (§23.7) — DAV servers have none, so a deleted contact is gone. Carrel could hold deleted objects for N days. This exists nowhere in the ecosystem and is exactly the feature that saves somebody's data once. The price is a direct breach of §2: restoring needs the **body**, not metadata, so it would be the first real local persistence of content. Mitigations are genuine (only deletions, only temporary, encrypted under the same DEK, bounded, auto-expiring) but so is the breach. And it would only cover deletions made through Carrel — a deletion from another client is seen as an object that has vanished, already without its body — which has to be said plainly or it creates a false sense of safety. The question is whether a partial wastebasket justifies giving up a founding principle, or whether backup (§23.3) closes the same scenario with no concession at all.

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
- **Protecting against the host operator.** DAV needs the password in cleartext for Basic authentication, so it is in memory during a request. This is inherent to the protocol.
