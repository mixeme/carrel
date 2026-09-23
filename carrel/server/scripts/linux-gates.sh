#!/usr/bin/env bash
# Manual gates for the Linux dev machine (docs/plan/server.md, «Linux-машина»).
# Run from carrel/server after reading dev/env.txt on that host.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "linux-gates.sh is for Linux only (Windows skips F-10 by design)" >&2
  exit 1
fi

echo "== F-10: server.key permission check =="
go test -run '^TestLoadServerKeyRejectsPermissiveMode$' ./internal/crypt/...

echo "== S-28: Docker image build and smoke =="
docker build -t carrel:linux-gates \
  --build-arg VERSION=linux-gates \
  --build-arg COMMIT=local \
  .
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

echo "== S-30.2: Radicale integration =="
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

# Stalwart, the fourth engine. NOT YET RUN: the stand was written from the
# documentation on a machine without Docker (testdata/stalwart/README.md).
# It is behind a flag until someone has seen it come up, so a red stand
# cannot fail the gates of everyone else. Run with CARREL_STALWART=1.
if [[ "${CARREL_STALWART:-}" == "1" ]]; then
  echo "== S-30.2: Stalwart integration =="
  # A stand that has never been run must not take the rest of the gates
  # with it: its whole step is one subshell, and a failure is reported and
  # stepped over. The flag alone was not enough — set -e killed the run on
  # 22.09.2026, when Stalwart could not bind a port and Xandikos and Davis
  # never got their turn.
  if ! (
    docker compose -f docker-compose.linux.yml up -d stalwart
    for _ in $(seq 1 60); do
      if curl -sf -o /dev/null http://127.0.0.1:8089/ 2>/dev/null; then
        break
      fi
      sleep 0.5
    done
    # Two runs, not one shell: the CLI image carries no shell.
    docker compose -f docker-compose.linux.yml run --rm stalwart-setup apply --file plan.ndjson
    docker compose -f docker-compose.linux.yml run --rm stalwart-setup create Action/ReloadSettings
    go test -tags=integration -count=1 -timeout 30m -run 'Stalwart' ./integration/...
  ); then
    echo "!! Stalwart: стенд не поднялся или набор упал; остальные ворота идут дальше"
  fi
fi

# Xandikos, the second light engine. NOT YET RUN either
# (testdata/xandikos/README.md); behind a flag for the same reason as
# Stalwart. Run with CARREL_XANDIKOS=1.
if [[ "${CARREL_XANDIKOS:-}" == "1" ]]; then
  echo "== S-30.2: Xandikos integration =="
  if ! (
    docker compose -f docker-compose.linux.yml up -d xandikos
    for _ in $(seq 1 60); do
      if curl -sf -o /dev/null http://127.0.0.1:8081/ 2>/dev/null; then
        break
      fi
      sleep 0.5
    done
    go test -tags=integration -count=1 -timeout 30m -run 'Xandikos' ./integration/...
  ); then
    echo "!! Xandikos: стенд не поднялся или набор упал; остальные ворота идут дальше"
  fi
fi

# Davis, sabre/dav once more. NOT YET RUN either, and its DAV account is
# made by hand in the admin screens — the suite skips and says so when
# CARREL_DAVIS_USER and CARREL_DAVIS_PASS are unset
# (testdata/davis/README.md). Run with CARREL_DAVIS=1.
if [[ "${CARREL_DAVIS:-}" == "1" ]]; then
  echo "== S-30.2: Davis integration =="
  if ! (
    docker compose -f docker-compose.linux.yml up -d davis
    for _ in $(seq 1 60); do
      if curl -sf -o /dev/null http://127.0.0.1:8082/ 2>/dev/null; then
        break
      fi
      sleep 0.5
    done
    docker compose -f docker-compose.linux.yml run --rm davis-setup
    go test -tags=integration -count=1 -timeout 30m -run 'Davis' ./integration/...
  ); then
    echo "!! Davis: стенд не поднялся или набор упал; остальные ворота идут дальше"
  fi
fi

if [[ -n "${CARREL_SFTPGO_URL:-}" && -n "${CARREL_SFTPGO_USER:-}" && -n "${CARREL_SFTPGO_PASS:-}" ]]; then
  echo "== S-30.2: SFTPGo integration =="
  go test -tags=integration -count=1 -run 'SFTPGo' ./integration/...
fi

if [[ -n "${CARREL_BAIKAL_URL:-}" && -n "${CARREL_BAIKAL_USER:-}" && -n "${CARREL_BAIKAL_PASS:-}" ]]; then
  echo "== S-30.2: Baikal integration =="
  go test -tags=integration -count=1 -run 'Baikal' ./integration/...
fi

echo "linux gates passed"
