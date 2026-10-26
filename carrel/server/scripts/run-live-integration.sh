#!/usr/bin/env bash
# Live S-30.2 integration tests.
#
# The credentials are read by the tests themselves (integration/live_creds.go):
# myenv/carrel/credentials.txt in the nearest folder above the checkout, or the
# file CARREL_CREDENTIALS names. A second reader of that file is a second thing
# to break when the file is edited. It is not in git and never travels to the
# Linux machine, so there the servers that need it skip themselves.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ "$(uname -s)" == "Linux" ]]; then
  echo "== Radicale (compose) =="
  docker compose -f docker-compose.linux.yml up -d radicale
  for _ in $(seq 1 30); do
    if curl -sf -u test:test http://127.0.0.1:5232/test/ >/dev/null 2>&1; then
      break
    fi
    sleep 0.2
  done
  export CARREL_RADICALE_URL=http://127.0.0.1:5232/test/
fi

echo "== integration tests =="
go test -tags=integration -count=1 ./integration/... -v
