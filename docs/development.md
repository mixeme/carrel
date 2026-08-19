# Development

Building, running and testing Carrel, and the conventions the code follows. For the shape of the code see [architecture.md](architecture.md); for what the tests cover, [tests.md](tests.md).

---

## What you need

- **Go 1.22 or newer.** Nothing else to build or run: templates, stylesheet and the vendored htmx are compiled in.
- **A C compiler** only for `go test -race`. On Windows, MSYS2's gcc; on Debian, `build-essential`. Without one, `-race` refuses to build and the rest of the suite is unaffected.
- **Docker** for the container and for the local Baikal used by integration tests.

There is no Node, no npm, no bundler and no generated code. `go build` is the whole build.

---

## Running it

```bash
go run ./cmd/carrel
```

With no `CARREL_DATA_DIR` it uses `/var/lib/carrel`, which is not what you want on a workstation:

```bash
CARREL_DATA_DIR=./dev/data go run ./cmd/carrel
```

PowerShell:

```powershell
$env:CARREL_DATA_DIR = "./dev/data"; go run ./cmd/carrel
```

An empty directory starts in bootstrap mode: open <http://127.0.0.1:8080/> and create the first administrator. To start over, delete the directory — `state.enc` and `server.key` are all there is, and without the second the first cannot be read, so deleting both is the reset.

`dev/` is not tracked. Local notes, a data directory and credentials all live there.

### Useful settings while developing

```bash
CARREL_LOG_LEVEL=debug            # structured JSON to stdout; never secrets, even here
CARREL_DAV_SSRF_ALLOWLIST=127.0.0.1,localhost   # a DAV server on this machine
CARREL_PROGRESS_MODE=poll         # to work on the fallback rather than the stream
```

The allowlist is the one you will forget. Without it a DAV server on `localhost` is refused, correctly, and the connection screen will tell you the address is not allowed.

---

## Building the container

```bash
docker compose build \
  --build-arg VERSION=0.9.0 \
  --build-arg COMMIT=$(git rev-parse --short HEAD)
docker compose up -d
```

The image is a multi-stage build: `golang:alpine` compiles with `CGO_ENABLED=0`, and the result goes into `gcr.io/distroless/static:nonroot`. There is no shell and no curl in the final image, which is why the health check invokes the binary itself:

```bash
docker compose exec carrel /carrel healthcheck
```

Version and commit reach `/about` through `-ldflags -X`. A build without them says `0.1.0` and `unknown`, which is fine locally and wrong in a release.

---

## Tests

```bash
go test ./...                      # the gate: must pass
CGO_ENABLED=1 go test -race ./...  # needs a C compiler
go test -tags=integration ./...    # needs live servers; skips without them
go vet ./...
gofmt -l .                         # must print nothing
```

`go test ./...` is what has to be green before anything is committed. It needs no network and no server: the DAV-facing tests run against fake servers in `httptest`.

Integration tests are behind the `integration` build tag and skip themselves when their environment variables are unset, so running them without credentials is not an error. See [dev-credentials.md](dev-credentials.md) for the two sets — Baikal for CalDAV and CardDAV, a separate WebDAV account for files and attachments.

A local Baikal for the DAV tests:

```bash
docker compose -f compose.test.yaml up -d
```

Writing tests is covered in [tests.md](tests.md), including the fake-server helpers to reuse rather than rebuild.

### Desktop application

Wails v2 wrapper — see [plans/desktop-wrapper.md](plans/desktop-wrapper.md).

```bash
# Remote mode: pass a URL or set desktop.json (mode remote).
go build -o carrel-desktop ./cmd/carrel-desktop
./carrel-desktop -remote-url https://carrel.example

# Local mode: sidecar on 127.0.0.1 (desktop.json mode local, or -local).
go build -o carrel ./cmd/carrel
go build -o carrel-desktop ./cmd/carrel-desktop
./carrel-desktop -local -sidecar ./carrel
```

`wails build` uses [wails.json](../wails.json) at the repo root. On Linux install `libwebkit2gtk-4.1-dev` and `pkg-config` first. Windows needs the WebView2 Evergreen runtime. The webview loads the full Carrel UI from the remote instance; fan-out progress uses the same SSE stream (or poll fallback) as in a browser.

---

## Code conventions

The point of these is that a reader can tell what is load-bearing. Most of them are visible in any existing file; the ones worth stating are the ones a reasonable person would otherwise do differently.

### Every file

An SPDX header, matching what is already there:

```go
// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later
```

### Comments say why, not what

The code says what it does. A comment earns its place by recording a constraint the code cannot show: which specification section a rule comes from, what breaks if it is changed, why an obvious alternative was rejected. `§` references are to [carrel-spec.md](carrel-spec.md) and are worth keeping — they are how a rule that looks arbitrary is traced back to the reason for it.

What not to write: a comment restating the next line, a note about who changed something, or an explanation of why a diff is correct. The last one is a message to a reviewer and becomes noise the moment it merges.

### Errors

Wrap with context and the package name, and let `errors.Is` and `errors.As` work:

```go
return fmt.Errorf("files: list %s: %w", dir, err)
```

Sentinel errors for conditions a caller decides about (`ErrOutsideCollection`, `ErrSSRF`), typed errors when the caller needs the detail (`*HTTPError`, `*ConflictError`). Handlers turn these into something a person can act on through `userFacingDAVError`; the raw error goes to the log, not to the page.

### Two rules with no exceptions

**Never build an object from form fields.** Read a display view, build a `model.Patch`, apply it to the object the server sent. This is how properties from other clients survive an edit, and it is enforced by the payload being unexported — if you find yourself wanting to construct one, that is the design working.

**Never write without a precondition.** A create carries `If-None-Match: *`; an update carries the `If-Match` of the version it was read at. An update with no ETag is refused inside the provider rather than sent.

### Handlers

One file per feature area, split when the read and write halves grow apart (`notes.go` / `notes_io.go`). A handler is a method on `*Server`:

```go
func (s *Server) NotesList(w http.ResponseWriter, r *http.Request)
```

The shape, in order: read path values, decode the collection, get the session, build the provider, do the work, render. Path parameters are `r.PathValue`; the collection path is base64url in a single segment (`EncodeCollectionPath`), and anything below it is a query parameter rather than a second layer of encoding.

Mutations dispatch on a form field named `action`. Every form carries `csrf_token`; an htmx post that is not a form carries `X-CSRF-Token`. Every edit form carries the `etag` it was rendered from.

Add routes inside the existing `app`, `admin` or `photos` mux so the guards apply automatically. If you add a prefix that serves bytes rather than pages, give it its own mux and chain the guards explicitly, as `/d/` and `/a/` do.

### Templates

`{{define "body"}}`, rendered against `base.html`. A fragment reused on more than one screen is defined in `base.html` — the sources panel, the progress panel, the attachments block, the administration subsection nav. Strings go directly in the template: English, one language, no message catalogue.

`template.HTML` is never applied to anything a user typed.

### CSS and JavaScript

One stylesheet, hand-written, no preprocessor. Print rules live in the `@media print` block at the end.

JavaScript is for what an htmx attribute cannot express — printing, copying, the progress fallback, paste and drop. It goes in `carrel.js` as a delegated listener keyed off a `data-` attribute. There is nothing inline anywhere and the CSP has no `unsafe-inline` at all, so an inline handler will silently not run.

### Dependencies

The specification fixes the list: `emersion/go-webdav`, `emersion/go-ical`, `emersion/go-vcard`, `teambition/rrule-go`, `golang.org/x/crypto`. Adding one is a decision to be agreed, not a commit — and it goes in `THIRD_PARTY.md` with its licence text at the same time.

Web frameworks are out: `net/http` and `html/template` are enough, and the routing this needs is what `http.ServeMux` gained in Go 1.22.

---

## Which tasks need which care

Carrel has been built largely with an AI coding agent, and the stage plans carried a table of which model to use for what. The plans are gone; the judgement in them is worth keeping, because it is really a statement about where mistakes are expensive and where they are cheap.

| Kind of work | Care needed | Why |
|---|---|---|
| Crypto, key handling, constant-time comparison | The most capable model available, in reasoning mode, plus a second pass over the diff alone | A mistake here loses somebody's data or leaks it, and neither shows up as a failing test |
| Outbound connections, SSRF, path checks | The same | The edge cases are the whole feature: redirects, DNS rebinding, a name that looks like a traversal |
| Preserving unknown properties (§8) | The same, and an audit of `model` before merging | A silent loss is discovered months later, when the original is gone |
| DAV protocol: multistatus, discovery, fan-out | A reasoning model | Wire formats and concurrency; the `PROPFIND` bug that asked servers for nothing passed every test that only read responses |
| Concurrency: goroutines, cancellation, caches | A reasoning model, and `-race` is not optional | Leaks and races do not reproduce on demand |
| Destructive operations: server merge, delete | A reasoning model, in its own session, after the non-destructive path is settled | The order of writes and deletes *is* the safety |
| Provider CRUD over a transport that already works | A capable model without extra ceremony | The patterns are established; follow the neighbouring provider |
| htmx templates, CSS, print rules | A fast model, iterating | Cheap to check by looking, cheap to fix |
| Compose files, config plumbing, `THIRD_PARTY.md` | A fast model | Mechanical |

The practical shape that worked: security, transport and data-preservation code in dedicated reasoning sessions before any interface exists, then interface work in fast iterations once the tests underneath are green — and a second pass over the diff, by a reasoning model, before merging anything in the first three rows.

---

## Where things live

A file-level map for the parts that are spread across several files.

| | |
|---|---|
| Key schedule, DEK/KEK, escrow, server key | `internal/crypto/` |
| The state file, users, invites, settings, audit | `internal/store/` |
| DAV accounts and the sealed blob | `internal/account/blob.go`, `internal/store/accounts.go` |
| Per-screen source selections and defaults | `internal/account/views.go` |
| Duplicate decisions | `internal/account/duplicates.go`, `internal/store/duplicates.go` |
| Transport, SSRF guard, multistatus, XML property types | `internal/dav/` |
| Discovery chain and its trace | `internal/dav/discovery/` |
| `Object`, `Patch`, loss comparison | `internal/model/object.go`, `patch.go`, `loss.go` |
| Display views | `internal/model/contact.go`, `event.go`, `note.go`, `todo.go` |
| `ATTACH` | `internal/model/attach.go` |
| Markdown export and import of notes | `internal/model/markdown.go`, `import.go` |
| Duplicate fingerprints, normalisation, scoring, clustering, field merge | `internal/merge/fingerprint.go`, `normalize.go`, `score.go`, `cluster.go`, `fields.go`, `vcard.go` |
| Fan-out tasks, progress, timeouts | `internal/fanout/` |
| Session cache, drafts, thumbnails | `internal/session/cache.go`, `scratch.go`, `thumb.go` |
| Photo pipeline: EXIF, crop, thumbnails | `internal/photo/` |
| Provider read paths | `internal/provider/*/provider.go` |
| Provider write paths and conflicts | `internal/provider/*/write.go` |
| File paths and the traversal guard | `internal/provider/files/path.go` |
| Routes and the body-limit table | `internal/web/handler/routes.go` |
| Middleware, CSRF, auth guards | `internal/web/handler/middleware.go`, `csrf.go`, `auth.go` |
| Template loading and rendering | `internal/web/handler/render.go` |
| Shared template fragments: sources, progress, duplicates, attachments | `internal/web/template/base.html` |

---

## Adding things

### A setting

`internal/config/config.go`: a field on the right struct, a default constant, the `fileConfig` mirror, the `applyEnv` case, and a `validate` that rejects a value that cannot work. Then pass it through `cmd/carrel/main.go` to the `Server` field. Then a test in `config_test.go` covering default, file and environment, and one bad value.

### An iCalendar or vCard property

1. Add the field to the display view (`Event`, `Note`, `Todo`, `Contact`).
2. Add its name to that view's `known*Props` set, or it will also appear among the foreign properties the card lists as untouched.
3. Read it in the view function.
4. Write it through a patch in the handler's `toPatch`.
5. Test that an unrelated edit leaves it alone, that a round trip reports no loss, and that a property you did *not* model is still in `Other`.

`internal/model/attach.go` is the worked example, including the case of a property whose instances must go back exactly as they came.

### A screen

Handler file, view struct, template with `{{define "body"}}`, a route in the right mux, a navigation entry in `base.html` keyed on the page title, and a handler test that renders it. If the screen polls several collections, reuse `fanout` and the shared progress fragments rather than writing a second poll.

---

## Committing

**Commits only when asked.** This is a rule of the project, not a preference: work is reviewed as a working tree and committed on request.

When asked, the shape is one commit per stage or per coherent change, subject in the imperative and under about 70 characters, with the changelog updated in the same commit. `CHANGELOG.md` is not a list of diffs — it records what changed and *why it was done that way*, which is the part that cannot be recovered from the code later.

Before committing: `go build ./...`, `go vet ./...`, `gofmt -l .` silent, `go test ./...` green.

---

## Documentation map

| File | For |
|---|---|
| [../README.md](../README.md) | Someone deciding whether to run it |
| [architecture.md](architecture.md) | Someone about to change it |
| development.md | This |
| [tests.md](tests.md) | What is covered and how to add to it |
| [roadmap.md](roadmap.md) | What is not built, and what will not be |
| [carrel-spec.md](carrel-spec.md) | The requirements, in Russian; the `§` references point here |
| [manual-acceptance.md](manual-acceptance.md) | The checks that need a person and a real client |
| [dev-credentials.md](dev-credentials.md) | Test server credentials, by convention, never committed |
