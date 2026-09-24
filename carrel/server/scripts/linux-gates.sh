#!/usr/bin/env bash
# Manual gates for the Linux dev machine (docs/plan/server.md, «Linux-машина»).
# Run from carrel/server after reading dev/env.txt on that host.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "linux-gates.sh is for Linux only (Windows skips F-10 by design)" >&2
  exit 1
fi

# The whole unit suite, first and unconditionally. It used to run nowhere but
# a developer's Windows box, and on 23.09.2026 that showed: view's fixture
# check had been red since v0.2.1 — fixtures for sixty screens were added
# without the entry the test asks for in the same commit — and nothing caught
# it, because nothing ran it. A gate that runs only by hand is not a gate.
echo "== unit suite =="
go test ./...

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
