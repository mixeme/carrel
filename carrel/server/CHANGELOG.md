# Changelog

All notable changes to the Carrel server are documented here. Numbers like
`v0.0.N` name releases in this file only — they are not git tags unless an
entry lists **Tagged:**. This file tells consumers what to do, not what the
authors did.

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
