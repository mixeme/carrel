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
cid=$(docker run -d --rm \
  --read-only \
  --cap-drop=ALL \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  -v carrel-linux-gates-data:/data \
  -e CARREL_BIND=127.0.0.1 \
  -e CARREL_PORT=8080 \
  -p 127.0.0.1:18080:8080 \
  carrel:linux-gates)
trap 'docker rm -f "$cid" >/dev/null 2>&1 || true' EXIT

for _ in $(seq 1 30); do
  if curl -sf http://127.0.0.1:18080/healthz | grep -qx ok; then
    echo "healthz ok"
    break
  fi
  sleep 0.2
done
curl -sf http://127.0.0.1:18080/healthz | grep -qx ok

echo "== S-30.2: Radicale integration =="
docker compose -f docker-compose.linux.yml up -d radicale
trap 'docker compose -f docker-compose.linux.yml down -v; docker rm -f "$cid" >/dev/null 2>&1 || true' EXIT
for _ in $(seq 1 30); do
  if curl -sf -u test:test http://127.0.0.1:5232/test/ >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
CARREL_RADICALE_URL=http://127.0.0.1:5232/test/ \
  go test -tags=integration -count=1 ./integration/...

echo "linux gates passed"
