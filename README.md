# Carrel

**One web client for several CalDAV/CardDAV servers at once — calendars, contacts, tasks and notes in a single view, with no database of its own.**

Carrel is a self-hosted web front end for DAV servers you already run. It connects to Baikal, Radicale, Davis, Nextcloud or anything else that speaks CalDAV, CardDAV or plain WebDAV, and gives you full read and write access to contacts, events, tasks, notes and files — from several accounts on several servers, merged into one list.

It stores no copy of your data. The DAV server is the source of truth; Carrel is the reading room.

> **The name.** A carrel is the individual study booth in a library — your own desk, for working with material that belongs to somebody else. That is exactly what this is. Nothing to do with Alexis Carrel.

[Why Carrel](#why-carrel) · [Everything it does](#everything-it-does) · [Not built yet](#not-built-yet) · [What it is not](#what-it-is-not) · [Running it](#running-it) · [Configuration](#configuration) · [Threat model](#what-carrel-protects-you-from-and-what-it-does-not) · [Licence](#licence)

---

## Why Carrel

Five things, in the order they matter. Three are here now; the last two are what the [roadmap](docs/roadmap.md) is for.

1. **Several servers, one view.** Calendars, contacts, tasks and notes from every account you connect, merged into one list, with no database of its own. Nothing else living does all four across several servers: the one actively developed competitor covers calendars and tasks only and brings its own database.
2. **Not a Nextcloud.** One binary, one container, Go and htmx — no Node, no npm, no bundler, no CDN, no local copy of your content. If you want only calendars, contacts and notes and not the whole machine, this is that.
3. **Edit your jtx Board notes on a big screen.** VJOURNAL has clients on Android and Linux and no web link between them. Carrel is that link, and compatibility is a tested promise rather than a claim.
4. **One account for every device** *(planned — [davloom](docs/roadmap.md))*. Point DAVx5 or Thunderbird at Carrel once and get every configured server behind it, instead of setting up five accounts on every device.
5. **Setting up a new device in one tap** *(planned)*. An Apple configuration profile carrying every connected account at once; a QR code that opens DAVx5 with the fields already filled.

---

## Everything it does

The complete inventory, kept current. If something is here, it works; if it is missing, look under [Not built yet](#not-built-yet).

### Accounts and discovery

- Connect a DAV account with a URL, a username and a password. `.well-known` is tried first, then the URL exactly as typed — which is the reliable path for Baikal.
- One principal gives calendars **and** address books, so an account is one connection rather than one per protocol.
- Several accounts on several servers per person, each enabled or disabled independently.
- **Failure tells you which step broke and what the server actually replied** — the well-known probe, the principal lookup, each home-set — instead of "cannot connect".
- Read-only collections are detected from the server's privileges and shown without edit controls.
- Plain WebDAV folders are detected automatically and become the Files section. No setting to turn it on.
- Administrators get the same discovery as a "test this server" tool, without saving anything.

### Contacts

- Address books in the sidebar; list loads in batches as you scroll.
- Create, edit and delete cards. **Properties this build does not know about survive the edit** — an X- property from another client is kept byte for byte, shown, and never dropped.
- A card edited elsewhere first opens a conflict screen with a property-by-property diff and three choices; it is never silently overwritten.
- Photos: upload, EXIF orientation applied, all metadata stripped, crop by pan, zoom and rotate, delete. A photo held as an external link is proxied and marked as not editable; a card with no photo at all gets a generated SVG of its initials, coloured from the UID.
- Import `.vcf`, one file or a zip of them, with a preview of counts, parse errors and UID collisions before anything is written. Import always creates: a colliding UID gets a fresh one and a line in the report.
- Export the whole address book as one `.vcf`.
- Print the list or a single card, with or without photos.

### Calendar

- Agenda grouped by day over a date range you choose. A list, not a grid.
- Create, edit and delete events, with the same conflict handling and property preservation as contacts.
- Recurrence editor: frequency, interval, weekdays, until, count, or a raw `RRULE`. Repeats are expanded locally rather than trusting the server. Series are edited whole.
- Attendees and participation status shown, read only.
- Import and export `.ics`, optionally over a date range.
- Print the agenda, with the range and capture date in the footer.
- **Write a note from a meeting** — the note arrives already linked to the event and dated to match.

### Tasks

- Tasks of one collection with open, done and all filters, counts and an overdue marker, ordered by what still needs doing.
- Ticking one off is a three-property edit, not a rebuilt object, so everything else on the task stays exactly as it was.
- Create, edit, delete, with conflicts handled as everywhere else.

### Notes

- Collection newest first, with an excerpt and a tag filter.
- Card with title, body, date, time, tags and links.
- **New note** in the navigation bar on every screen: one field, saved into the collection you used last, never asking where it goes.
- `RELATED-TO` links notes, tasks and events both ways, resolved to something clickable where the target is in the same collection.
- **Timeline on every contact card** — every event that person attended and every note that mentions them, across every server you ticked, matched on their addresses first and their display name second. Computed live, stored nowhere. This is the thing people usually buy a CRM for.
- Export one note as Markdown with YAML front matter, or a whole collection as a zip streamed straight to the browser. Anything with no front-matter field of its own is carried too, so an export loses nothing.
- Import Markdown from a file, a zip, **or a folder on your own WebDAV**, with or without front matter, previewed before it is written.
- Compatible with **jtx Board** and **Evolution**, including the `X-` properties they write.

### Files and attachments

- Browse a WebDAV collection with a breadcrumb, folders before files, size, type and modification time.
- Download streams straight through — a large file costs bandwidth, not memory — with byte-range support.
- Upload, create a folder, delete. **An upload never overwrites:** a name already taken is refused and said so.
- Attach a file to a note or an event by **pasting it (Ctrl+V) or dropping it on the card**. The folder is chosen once on your profile and never asked for again.
- Attachments are a standard `ATTACH` URI, not base64 — your `.ics` does not grow. Names are built from the date and the entry's title, so the folder stays readable outside Carrel.
- Opening an attachment goes through Carrel, and only for files in a collection you hold credentials for. A link elsewhere is shown, and not fetched.
- An attachment another client embedded is shown and left exactly as it is.
- **Removing an attachment does not delete the file** — another entry may point at it — and the interface says so.

### Across every source at once

- **Everything view:** one list from every ticked calendar over a date range, optionally narrowed to events, tasks or notes, merged by time. A slow source lands in its right position, not at the end.
- **Search** across calendars and address books simultaneously.
- Live progress per source: waiting, querying, done with a count, empty, timed out or unavailable — and which answers came from cache. A failed source has a Retry of its own; cancelling keeps what already arrived.
- Every row carries the account and collection it came from, because two servers holding the same meeting is normal.
- Which collections a screen polls is remembered per screen, encrypted, and survives a restart.

### Duplicates

- The same person in two address books on two servers is found with no extra requests, scored on what was already loaded.
- Each group is offered **with the reason it matched** — same address, same phone — beside the score, because a reason is something you can check.
- **Link** them: shown as one row with merged fields, and **nothing is written to any server**. **Not duplicates**: never offered again, across restarts. Or **merge on the server**, confirmed first, writing before it deletes and never deleting when the write failed.
- Repeatable fields merge as a union; a field the records disagree on is the one thing you are asked, and the answer is remembered.
- The matching threshold is a setting, because a shared family phone number is one person in one address book and two in another.

### Users, administration and security

- First run on an empty volume creates the first administrator. No default password.
- Invitations by copyable link or by email; the link always works even with SMTP completely unconfigured. The invitee chooses their own login and password.
- SMTP settings with a **test button that shows the server's whole reply**, and never the relay password.
- Email address change confirmed by a link.
- **Real isolation:** each person's DAV credentials are sealed under a key derived from their own password. No other user can read them, and neither can the administrator.
- **Optional key escrow**, off by default, never retroactive, and stated to the covered user at first sign-in and in their profile from then on — because it is a way in, not a key backup.
- Admin panel, as separate pages: users (roles, last login, account counts, live sessions; disable, delete, change role, end sessions, reset password announced as destructive), invitations, settings and mail, key escrow, and an audit log carrying no secrets.
- Changing your own password keeps every connection readable.
- CSRF everywhere, a CSP with no inline scripts at all, progressive rate limiting by address and account, and an SSRF guard that resolves, checks and dials the address it checked — after every redirect.

### How it runs

- One binary, one container, one volume. No database, no Redis, no sidecar.
- Read-only root filesystem, non-root user, all capabilities dropped, health check without a shell.
- Installs as a PWA with its own window and icon.
- Structured JSON logs that never carry a password, a token or the contents of a record.
- Graceful shutdown that drains requests and then wipes every key.
- English interface, one language.

---

## Not built yet

Named here so the list above can be read as complete. Detail, reasoning and order in the [roadmap](docs/roadmap.md).

**Next:** a new-device screen (Apple `.mobileconfig`, DAVx5 QR); backup to a WebDAV of your choice, encrypted, with restore; read-only publication by secret link — your own address book, calendar, notes, files and folders, or an external calendar or address-book feed, all viewable in the browser, and mailable — a link, or a letter with a contact, an event, a note or a file attached; [davloom](docs/roadmap.md), the proxy mode that makes Carrel one account for every device.

**Smaller gaps:** attachments on tasks; a per-collection line saying when it last changed on the server; showing an event's original time zone when it differs; a service worker so an installed app has a shell offline; a process-wide memory ceiling for the cache; three of the narrow-screen refinements.

---

## What it is not

Being clear about this saves everyone time.

- **Not a calendar grid.** The agenda is a list. No month view, no drag and drop. That is a decision, not a gap: a `REPORT` with a time range returns exactly what a list needs, and the grid is where every other web client spends its complexity budget.
- **Not a Nextcloud.** No file sharing, no chat, no office suite, no app store. If you want the whole machine, run the whole machine. This is for people who want calendars, contacts and notes and nothing else.
- **Not a sync engine.** There is no local copy of your data, no background refresh and no offline mode. If your DAV server is down, Carrel says so instead of showing you something stale.
- **Not a mobile app.** The interface works on a phone and is deliberately a lifeboat rather than a home: check the schedule, find a number, write a note. For comfortable phone use the right answer is **DAVx5 + jtx Board**, not this. Saying so up front is more useful than pretending otherwise.
- **Not a file manager.** The Files section exists to serve attachments and to fetch and place a file. No previews, no permissions, no renaming trees.
- **Not iTIP.** Attendees and participation status are shown, never negotiated. No invitations, no free/busy.
- **No password recovery.** Without your password there is no key, so there is nothing to recover with. The login page says this instead of offering a link that cannot work. The two real options — a destructive reset, or key escrow decided in advance — are both named.

---

## Running it

One container, one volume, nothing beside it.

```bash
git clone https://github.com/mixeme/carrel
cd carrel
docker compose up -d --build
```

Then open <http://127.0.0.1:8080/> and create the first administrator. An empty volume always lands on the setup screen; once an administrator exists that screen closes for good.

The compose file binds to loopback only, mounts the volume read-write and everything else read-only, drops all capabilities and runs as a non-root user. Do not publish the port: put it behind a reverse proxy that terminates TLS.

**Updating the image means a restart, and a restart signs every user out**, because keys live only in memory. That is deliberate and will not change.

### Behind a reverse proxy

Two things break by default, and both are worth getting right before you conclude something is wrong with Carrel.

**The progress stream must not be buffered.** The cross-source poll reports each server as it answers, over an event stream at `/app/find/{task}/stream`. A proxy that buffers responses delivers the whole thing at the end, which looks exactly like an indicator that hangs and never finishes. Carrel falls back to polling on its own when the stream will not open at all, but a buffering proxy is worse than a broken one — the stream opens and then says nothing.

**`X-Forwarded-Proto` must be set, not defaulted**, and Carrel must be told to believe it. Without `CARREL_TRUSTED_PROXIES` the header is ignored, the connection is treated as plain HTTP, and the session cookie goes out without `Secure`. With the header merely defaulted rather than overwritten, a client can forge it.

Apache:

```apache
<VirtualHost *:443>
    ServerName carrel.example.com

    SSLEngine on
    SSLCertificateFile    /etc/letsencrypt/live/carrel.example.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/carrel.example.com/privkey.pem

    ProxyPreserveHost On
    # `set`, not `setifempty`: the client's header must be overwritten.
    RequestHeader set X-Forwarded-Proto "https"

    # The progress stream of the cross-source poll. flushpackets=on is not
    # optional — without it the poll appears to hang.
    <LocationMatch "^/app/find/[^/]+/stream$">
        SetEnv proxy-sendchunked 1
        ProxyPass http://127.0.0.1:8080 flushpackets=on
    </LocationMatch>

    ProxyPass        / http://127.0.0.1:8080/
    ProxyPassReverse / http://127.0.0.1:8080/

    # Uploads: Apache's own default is 1 GB, which is below the file ceiling.
    LimitRequestBody 0
    # Longer than the 30 s a whole poll is allowed to take.
    ProxyTimeout 120
</VirtualHost>
```

Modules: `proxy`, `proxy_http`, `headers`, `ssl` (and `http2` if you use it).

nginx:

```nginx
server {
    listen 443 ssl;
    server_name carrel.example.com;

    ssl_certificate     /etc/letsencrypt/live/carrel.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/carrel.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto https;
        proxy_buffering off;          # the progress stream
        proxy_read_timeout 120s;      # longer than one whole poll
        client_max_body_size 0;       # uploads
    }
}
```

Either way, tell Carrel the proxy is trustworthy:

```yaml
environment:
  CARREL_TRUSTED_PROXIES: "127.0.0.1"
```

If the proxy really does buffer and cannot be reconfigured, `CARREL_PROGRESS_MODE=poll` turns the stream off and polls for status instead.

To serve Carrel from a subdirectory, set `CARREL_BASE_PATH=/carrel` as well, or the links and the htmx requests will point at the wrong place.

### Building with version metadata

```bash
docker compose build \
  --build-arg VERSION=0.10.0 \
  --build-arg COMMIT=$(git rev-parse --short HEAD)
```

Version and commit appear on `/about`, which is public and needs no login (AGPL §13).

### Without Docker

Go 1.22 or newer, and nothing else — templates, stylesheet and the vendored htmx are compiled into the binary.

```bash
go build -o carrel ./cmd/carrel
CARREL_DATA_DIR=./data ./carrel
```

---

## Configuration

Settings come from `config.json` in the data directory, and from the environment, which wins. Everything has a working default; the only value most people set is the data directory.

| Variable | Default | What it does |
|---|---|---|
| `CARREL_PORT` | `8080` | Listen port |
| `CARREL_BIND` | — | Listen address (IP only). Empty = all interfaces. Desktop local mode uses `127.0.0.1` |
| `CARREL_DATA_DIR` | `/var/lib/carrel` | The volume: server key and state file |
| `CARREL_BASE_PATH` | — | Mount under a prefix, e.g. `/carrel` |
| `CARREL_TRUSTED_PROXIES` | — | Addresses or CIDRs whose `X-Forwarded-For` is believed. **Set this** if you are behind a proxy: without it the rate limiter counts the proxy's address as everyone's |
| `CARREL_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `CARREL_DAV_SSRF_ALLOWLIST` | empty | Hosts whose private addresses may be reached. Needed only when your DAV server is on a local network — see below |
| `CARREL_DAV_CONNECT_TIMEOUT` | `10` | Seconds, connecting to a DAV server |
| `CARREL_DAV_REQUEST_TIMEOUT` | `30` | Seconds, one whole DAV request |
| `CARREL_DAV_MAX_RESPONSE_BYTES` | `10485760` | Ceiling on one DAV response |
| `CARREL_DAV_MAX_REDIRECTS` | `5` | Redirect chain limit |
| `CARREL_CACHE_COLLECTION_TTL` | `60` | Seconds before a collection is re-read even when the server reports no change |
| `CARREL_CACHE_MAX_COLLECTIONS` | `256` | Cached collections per session |
| `CARREL_CACHE_MAX_ETAG_ENTRIES` | `4096` | Cached object versions per session |
| `CARREL_CACHE_MAX_THUMB_BYTES` | `16777216` | Stricter ceiling for photo thumbnails |
| `CARREL_CACHE_MAX_THUMB_ENTRIES` | `512` | Thumbnails per session |
| `CARREL_PHOTO_MAX_SIDE` | `512` | Contact photos are scaled to this |
| `CARREL_PHOTO_JPEG_QUALITY` | `85` | Re-encode quality |
| `CARREL_PHOTO_MAX_PIXELS` | `100000000` | Refused before decoding, against decompression bombs |
| `CARREL_PHOTO_THUMB_SIDE` | `96` | Thumbnail size in lists |
| `CARREL_PHOTO_MAX_UPLOAD_BYTES` | `67108864` | Ceiling on a photo upload |
| `CARREL_IMPORT_MAX_BYTES` | `16777216` | Ceiling on a `.vcf` / `.ics` / `.md` / `.zip` upload |
| `CARREL_IMPORT_MAX_CARDS` | `5000` | Records one import may contain |
| `CARREL_FILES_MAX_UPLOAD_BYTES` | `268435456` | Ceiling on a file or attachment upload. Generous because nothing buffers it |
| `CARREL_FILES_MAX_ENTRIES` | `2000` | Members of one folder that are listed |
| `CARREL_PROGRESS_MODE` | `sse` | `poll` turns the progress stream off for a proxy that buffers |
| `CARREL_PROGRESS_POLL_MILLIS` | `700` | Fallback poll interval |
| `CARREL_PROGRESS_SOURCE_TIMEOUT` | `10` | Seconds before one slow source is marked timed out |
| `CARREL_PROGRESS_TOTAL_TIMEOUT` | `30` | Seconds before a whole poll gives up on what is left |
| `CARREL_DUPLICATES_THRESHOLD` | `60` | Points a pair must reach to be offered as a duplicate. Lower finds more and asks more of you |

### Connecting a DAV server on a local network

Carrel refuses to connect to private, loopback and link-local addresses by default, because you type the URL and the server follows it — that is a scanner of your own network otherwise, operated by whoever can log in. If your DAV server really is on the LAN, name its host explicitly:

```yaml
environment:
  CARREL_DAV_SSRF_ALLOWLIST: "dav.home.lan,192.168.1.20"
```

The list starts empty on purpose. Adding to it is a decision, and it applies to the host you name and not to the network it is on.

### Connecting an account

Enter the base URL, the username and the password on your profile page. Discovery tries `.well-known/caldav` and `.well-known/carddav` first and then the URL as you typed it — for Baikal the direct path is the reliable one, usually `https://host/dav.php/`, because `.well-known` lives in a web server configuration that is often not set up. If it fails you are shown which step failed and what the server actually replied, because "cannot connect" is not something you can act on.

One principal gives both calendars and address books, so an account is one connection and not one per protocol. A plain WebDAV folder found under the same root becomes the Files section; there is no setting for it.

---

## What Carrel protects you from, and what it does not

Stated plainly, because a security section that only lists strengths is not one.

**It does protect against:**

- **Someone without credentials.** Passwords and tokens are rate limited by address and by account together. Every token is 32 random bytes, stored only as a digest and compared in constant time. An unknown login, a wrong password and a disabled account all answer identically.
- **Another user of the same instance.** Your DAV credentials are sealed with a key derived from your password. Nobody else's session can open them, administrator included.
- **Someone who steals the volume.** The state file is encrypted. Without a user's password their saved credentials do not decrypt.
- **Your own browser being used against you.** CSRF tokens on every mutating request, a Content Security Policy with no inline scripts at all, `HttpOnly` and `SameSite` cookies, and a session identifier that rotates on login.
- **Carrel being used to scan your network.** Every outbound URL is resolved and the resulting address checked before connecting, after every redirect, connecting to the address already checked so a second DNS answer cannot redirect it.

**It does not protect against:**

- **Whoever runs the host.** DAV needs your password in cleartext for Basic authentication, so during a request it is in memory. A memory dump gives it up. This is inherent, not a bug to be fixed later — keep swap and core dumps off a host you care about.
- **An administrator with key escrow enabled.** Escrow is off on a fresh volume and is a way in, not a key backup: an administrator holding the master password can decrypt a covered user's credentials and therefore their data. That is the whole trade, which is why it is optional, never retroactive, and stated to the affected user at their first sign-in and in their profile from then on.
- **Your DAV server being compromised.** Carrel is a client.
- **Anything after a restart.** Keys live only in memory, so a restart signs everyone out. That is the point.

Your calendars and contacts are present in the process's memory while you are looking at them. That is what a cache in a session means.

---

## Licence

**AGPL-3.0-or-later.** See [LICENSE](LICENSE) and [THIRD_PARTY.md](THIRD_PARTY.md).

If you run a modified Carrel as a service for other people, AGPL §13 requires you to offer them the source of your modified version. The `/about` page is public and carries the version, the commit it was built from and a link to the source, which is how Carrel meets that obligation for itself — if you modify it, point that link at your own tree.

---

## More

| | |
|---|---|
| [docs/architecture.md](docs/architecture.md) | How it works inside: the layers, and why each boundary is where it is |
| [docs/development.md](docs/development.md) | Building, running, testing and the conventions to follow |
| [docs/tests.md](docs/tests.md) | What is tested, how, and what is deliberately left to a person |
| [docs/roadmap.md](docs/roadmap.md) | What is not built yet, and what will not be |
| [CHANGELOG.md](CHANGELOG.md) | What changed and why |
| [docs/carrel-spec.md](docs/carrel-spec.md) | The specification the whole thing is built against (Russian) |
