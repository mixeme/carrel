#!/usr/bin/env bash
# Manual gates for the Linux dev machine (docs/plan/server.md, «Linux-машина»).
# Run from carrel/server after reading dev/env.txt on that host.
#
# The code comes packed by scripts/linux-ship.sh and unpacked into
# /opt/carrel-dev/run-<commit>/: this repository as carrel/ and the clone of
# Typecase at the commit of the client's plan as typecase/ beside it. The
# binary wires in the client, the client takes Typecase by a replace onto the
# neighbouring clone (../../../typecase from here), and the image is built
# from the directory that holds both.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "linux-gates.sh is for Linux only (Windows skips F-10 by design)" >&2
  exit 1
fi

# The directory that holds carrel/ and typecase/: the build context of the
# image and the mount of the race detector.
ctx=$(cd ../../.. && pwd)
if [[ "$(basename "$(cd ../.. && pwd)")" != "carrel" || ! -f "$ctx/typecase/go.mod" ]]; then
  echo "want this repository as carrel/ with Typecase as typecase/ beside it in $ctx" >&2
  echo "(scripts/linux-ship.sh packs them so)" >&2
  exit 1
fi

# The whole unit suite, first and unconditionally. It used to run nowhere but
# a developer's Windows box, and on 23.09.2026 that showed: view's fixture
# check had been red since v0.2.1 — fixtures for sixty screens were added
# without the entry the test asks for in the same commit — and nothing caught
# it, because nothing ran it. A gate that runs only by hand is not a gate.
# Formatting and vet, before anything is run. Both lived only in the manual
# race script, and on 24.09.2026 an unformatted file went into a commit
# unnoticed — the same lesson as the unit suite's, learned a second time.
echo "== gofmt =="
# The toolchain's gofmt, not the host's. GOTOOLCHAIN=auto switches the go
# command to the version go.mod asks for, but a bare gofmt on PATH stays the
# host's own: go1.26.5 on this machine, which formats a multi-value return
# of composite literals differently from 1.27 and failed the first run of
# this step on a file untouched since v0.0.125 (24.09.2026).
unformatted=$("$(go env GOROOT)/bin/gofmt" -l .)
if [[ -n "$unformatted" ]]; then
  echo "not formatted:" >&2
  echo "$unformatted" >&2
  exit 1
fi
echo "== vet =="
go vet ./...
go vet -tags=integration ./integration/...

echo "== unit suite =="
# An explicit ceiling. go test's own default is ten minutes per package, and
# internal/web — hundreds of tests against real HTTP stands — crossed it on
# 24.09.2026: the run was killed mid-test, which reads as a failure and is
# not one. How long the package takes is watched on its own (docs/plan).
go test -timeout 30m ./...

# The race detector, behind a flag: it needs cgo and a C compiler, the host
# has neither, so it runs in golang:1.27-bookworm with a module cache volume
# of Carrel's own, and takes about seven minutes. First run 24.09.2026: no
# data race; one test counted on wall-clock speed and was made not to.
# One package at a time and one test at a time: under the detector every
# harness pays Argon2 at 64 MiB several times over, and on 28.09.2026
# internal/web in parallel was killed at the 3 GB limit on a 4 GB host;
# serially it passes (about five minutes for web alone). On 01.10.2026 even
# the serial run was killed at 2.3 GB: the heap grows faster under the
# detector than the collector frees it. GOMEMLIMIT makes it collect sooner;
# with it internal/web passes in about six minutes and no race is found.
if [[ "${CARREL_RACE:-}" == "1" ]]; then
  echo "== race detector =="
  docker run --rm \
    -v "$ctx":/src \
    -v carrel-race-gomod:/go/pkg/mod \
    -w /src/carrel/carrel/server \
    --memory=3g \
    -e GOTOOLCHAIN=auto \
    -e GOMEMLIMIT=1400MiB \
    golang:1.27-bookworm \
    go test -race -count=1 -p 1 -parallel 1 -timeout 120m ./...
fi

echo "== F-10: server.key permission check =="
go test -run '^TestLoadServerKeyRejectsPermissiveMode$' ./internal/crypt/...

echo "== S-28: Docker image build and smoke =="
# From the directory above the repository, with Typecase beside it; the
# Dockerfile's own ignore file (Dockerfile.dockerignore, read by BuildKit)
# lets in the server, the client and Typecase and nothing else.
DOCKER_BUILDKIT=1 docker build -t carrel:linux-gates \
  -f "$ctx/carrel/carrel/server/Dockerfile" \
  --build-arg VERSION=linux-gates \
  --build-arg COMMIT=local \
  "$ctx"
# A fresh volume every run: the first start on an empty /data is what ships.
# CARREL_BIND stays at the image default (0.0.0.0 inside the container);
# the host side is loopback-only through -p.
docker volume rm -f carrel-linux-gates-data >/dev/null
cid=$(docker run -d \
  --read-only \
  --cap-drop=ALL \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  -v carrel-linux-gates-data:/data \
  -e CARREL_PORT=8080 \
  -p 127.0.0.1:18080:8080 \
  carrel:linux-gates)
cleanup_smoke() {
  docker rm -f "$cid" >/dev/null 2>&1 || true
  docker volume rm -f carrel-linux-gates-data >/dev/null 2>&1 || true
}
trap cleanup_smoke EXIT

for _ in $(seq 1 30); do
  if curl -sf http://127.0.0.1:18080/healthz | grep -qx ok; then
    echo "healthz ok"
    break
  fi
  sleep 0.2
done
if ! curl -sf http://127.0.0.1:18080/healthz | grep -qx ok; then
  docker logs "$cid" >&2 || true
  exit 1
fi
# A page of the client from the image, and the library's files it names,
# from the binary (C-5.9): on an empty volume /login leads to /setup.
ctype=$(curl -sfL -o /tmp/carrel-linux-gates-login.html -w '%{content_type}' http://127.0.0.1:18080/login)
if [[ "$ctype" != "text/html; charset=utf-8" ]]; then
  echo "/login is not the client's page: $ctype" >&2
  exit 1
fi
files=$(grep -oE '(src|href)="/static/tc/[0-9a-f]{12}/[^"]+"' /tmp/carrel-linux-gates-login.html | sed -E 's/^(src|href)="//; s/"$//')
rm -f /tmp/carrel-linux-gates-login.html
if [[ $(grep -c . <<<"$files") -ne 3 ]]; then
  echo "/login does not name the three files of the library: $files" >&2
  exit 1
fi
for f in $files; do
  if ! curl -sfI "http://127.0.0.1:18080$f" | grep -qi '^cache-control: public, max-age=31536000, immutable'; then
    echo "$f: not served from the image as a file kept for good" >&2
    exit 1
  fi
done
echo "client page and library files ok"

echo "== S-30.2: Radicale integration =="
# Start from nothing of ours running. This step deliberately runs the whole
# integration package — the remote suites skip themselves when their
# credentials are unset, and that is how a new one never gets forgotten — but
# the local stands skip by *reachability*, not by credentials. So a Xandikos
# left up by hand runs its suites inside this step, and set -e ends the run
# there: on 23.09.2026 that is exactly what happened, and Stalwart and Davis
# never got their turn although each has its own isolated step. The stands
# behind a flag come up in their own step and nowhere else.
docker compose -f docker-compose.linux.yml down -v 2>/dev/null || true
docker compose -f docker-compose.linux.yml up -d radicale
trap 'docker compose -f docker-compose.linux.yml down -v; cleanup_smoke' EXIT
for _ in $(seq 1 30); do
  if curl -sf -u test:test http://127.0.0.1:5232/test/ >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
CARREL_RADICALE_URL=http://127.0.0.1:5232/test/ \
  go test -tags=integration -count=1 -timeout 30m ./integration/...

# Stalwart, the fourth engine and the only one on Rust. It came up on
# 23.09.2026 and all seven suites are green, so it stands in the ordinary
# row beside Radicale: no flag, and a failure fails the gates like any other.
echo "== S-30.2: Stalwart integration =="
# In recovery mode to be provisioned: that mode runs the management listener
# and nothing else — an ordinary account authenticates on it and is then
# refused, and even the administrator's DAV homes answer 404 (SV-46).
CARREL_STALWART_RECOVERY=1 docker compose -f docker-compose.linux.yml up -d stalwart
for _ in $(seq 1 90); do
  if curl -sf -o /dev/null -u admin:carrel-admin http://127.0.0.1:8089/api/schema 2>/dev/null; then
    break
  fi
  sleep 1
done
# The plan makes the domain, the account with its password, and the DAV
# listener. The CLI image carries no shell, so it is invoked directly.
# --no-deps: without it compose brings the dependency up to *this* command's
# configuration, where recovery mode is unset and so off — it recreated
# Stalwart out of recovery mode before the plan ran (gates, 23.09.2026).
docker compose -f docker-compose.linux.yml run --rm --no-deps stalwart-setup apply --file plan.ndjson
# And again without recovery mode, so the listener the plan made is the one
# that runs. --force-recreate, because a listener is read at start and
# nothing else makes it: if the container already runs without recovery
# mode, a plain `up -d` finds nothing to change and the plan's listener
# never starts — port 8088 is published and nobody behind it answers. The
# volume stays, so the account and the listener are still there.
CARREL_STALWART_RECOVERY=0 docker compose -f docker-compose.linux.yml up -d --force-recreate stalwart
for _ in $(seq 1 60); do
  if curl -sf -o /dev/null http://127.0.0.1:8088/.well-known/caldav 2>/dev/null; then
    break
  fi
  sleep 1
done
go test -tags=integration -count=1 -timeout 30m -run 'Stalwart' ./integration/...

# Xandikos, the second light engine beside Radicale: Python, every collection
# a git repository, and the one server in the lineup that asks for no
# password. All seven suites went green on 23.09.2026 after five defects of
# ours were fixed against it (docs/servers.md, SV-40 to SV-44, SV-47), so it
# stands in the ordinary row: no flag, and a failure fails the gates.
echo "== S-30.2: Xandikos integration =="
docker compose -f docker-compose.linux.yml up -d xandikos
for _ in $(seq 1 60); do
  if curl -sf -o /dev/null http://127.0.0.1:8081/ 2>/dev/null; then
    break
  fi
  sleep 0.5
done
go test -tags=integration -count=1 -timeout 30m -run 'Xandikos' ./integration/...

# Davis, sabre/dav once more, and the third engine here that serves files
# beside calendars and contacts. First run 24.09.2026: all eight suites green
# once the stand itself was right (testdata/davis/README.md), so it stands in
# the ordinary row: no flag, and a failure fails the gates. Its one DAV
# account is made through its own admin form, by a script.
echo "== S-30.2: Davis integration =="
docker compose -f docker-compose.linux.yml up -d davis
for _ in $(seq 1 60); do
  if curl -sf -o /dev/null http://127.0.0.1:8082/ 2>/dev/null; then
    break
  fi
  sleep 0.5
done
docker compose -f docker-compose.linux.yml run --rm davis-setup
bash testdata/davis/make-account.sh
go test -tags=integration -count=1 -timeout 30m -run 'Davis' ./integration/...

# DAViCal: PHP over PostgreSQL and not sabre/dav, an engine of its own. First
# run 24.09.2026: all seven suites green once two findings of its own were
# answered (docs/servers.md, SV-50, SV-51), so it stands in the ordinary row.
# The image makes its database and its admin at start; the one account the
# suites use is an ordinary principal, made through its admin form.
echo "== S-30.2: DAViCal integration =="
docker compose -f docker-compose.linux.yml up -d davical-db davical
for _ in $(seq 1 120); do
  if curl -sf -o /dev/null http://127.0.0.1:8083/ 2>/dev/null; then
    break
  fi
  sleep 1
done
bash testdata/davical/make-account.sh
go test -tags=integration -count=1 -timeout 30m -run 'DAViCal' ./integration/...

if [[ -n "${CARREL_SFTPGO_URL:-}" && -n "${CARREL_SFTPGO_USER:-}" && -n "${CARREL_SFTPGO_PASS:-}" ]]; then
  echo "== S-30.2: SFTPGo integration =="
  go test -tags=integration -count=1 -run 'SFTPGo' ./integration/...
fi

if [[ -n "${CARREL_BAIKAL_URL:-}" && -n "${CARREL_BAIKAL_USER:-}" && -n "${CARREL_BAIKAL_PASS:-}" ]]; then
  echo "== S-30.2: Baikal integration =="
  go test -tags=integration -count=1 -run 'Baikal' ./integration/...
fi

echo "linux gates passed"
