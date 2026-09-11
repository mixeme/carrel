# Changelog

All notable changes to the Carrel server are documented here. Versioning
follows [semantic versioning](https://semver.org/). This file tells
consumers what to do, not what the authors did.

## carrel/server/v0.0.5 — 2026-09-04

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

## carrel/server/v0.0.4 — 2026-09-04

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

## carrel/server/v0.0.3 — 2026-09-04

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

## carrel/server/v0.0.2 — 2026-09-04

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

## carrel/server/v0.0.1 — 2026-09-04

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
