#!/usr/bin/env bash
# The Linux machine's stands — Radicale, Stalwart, Xandikos, Davis, DAViCal —
# held up for the acceptance stand on the test servers (TestLiveStand,
# docs/visual/fixtures/README.md), with their accounts made as the gates make
# them. Project carrel-stands on ports of their own (testdata/stands-ports.yml):
# the gates' stands and their `down -v` are another project.
#
#   linux-stands.sh up     — bring them up and make the accounts
#   linux-stands.sh down   — take them down with their volumes
#
# From Windows, through one tunnel (Windows OpenSSH):
#   ssh -N -L 15232:127.0.0.1:15232 -L 18081:127.0.0.1:18081 -L 18082:127.0.0.1:18082 \
#       -L 18083:127.0.0.1:18083 -L 18088:127.0.0.1:18088 root@<machine>
set -euo pipefail
cd "$(dirname "$0")/.."
dc() { docker compose -p carrel-stands -f docker-compose.linux.yml -f testdata/stands-ports.yml "$@"; }
wait_for() { # url tries pause [curl options]
  local url=$1 tries=$2 pause=$3
  shift 3
  for _ in $(seq 1 "$tries"); do
    if curl -sf -o /dev/null "$@" "$url" 2>/dev/null; then
      return 0
    fi
    sleep "$pause"
  done
  echo "no answer at $url" >&2
  return 1
}

case "${1:-}" in
up)
  dc up -d radicale xandikos davis davical-db davical
  CARREL_STALWART_RECOVERY=1 dc up -d stalwart
  wait_for http://127.0.0.1:18089/api/schema 90 1 -u admin:carrel-admin
  dc run --rm --no-deps stalwart-setup apply --file plan.ndjson
  CARREL_STALWART_RECOVERY=0 dc up -d --force-recreate stalwart
  wait_for http://127.0.0.1:18088/.well-known/caldav 60 1
  # Radicale's web interface is off (testdata/radicale/config): a GET is
  # 403, the principal answers PROPFIND.
  wait_for http://127.0.0.1:15232/test/ 30 0.5 -u test:test -X PROPFIND -H 'Depth: 0'
  wait_for http://127.0.0.1:18081/ 60 0.5
  # Davis answers 500 until its database is made: the setup first.
  dc run --rm davis-setup
  wait_for http://127.0.0.1:18082/login 60 0.5
  bash testdata/davis/make-account.sh http://127.0.0.1:18082
  wait_for http://127.0.0.1:18083/ 120 1
  bash testdata/davical/make-account.sh http://127.0.0.1:18083
  echo "STANDS-UP"
  ;;
down)
  dc down -v
  echo "STANDS-DOWN"
  ;;
*)
  echo "usage: $0 up|down" >&2
  exit 2
  ;;
esac
