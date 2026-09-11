#!/usr/bin/env bash
# Live S-30.2 integration tests. Credentials from dev/credentials.txt (not in git).
set -euo pipefail

cd "$(dirname "$0")/.."
CREDS="${CREDS_FILE:-$(cd ../.. && pwd)/dev/credentials.txt}"
if [[ ! -f "$CREDS" ]]; then
  echo "missing credentials: $CREDS" >&2
  exit 1
fi

section=""
while IFS= read -r line || [[ -n "$line" ]]; do
  case "$line" in
    "Baikal DAV") section=baikal; continue ;;
    "SFTPGo WebDAV") section=sftpgo; continue ;;
    "") section=""; continue ;;
  esac
  case "$section" in
    baikal)
      if [[ "$line" =~ ^https?:// ]]; then export CARREL_BAIKAL_URL="$line"
      elif [[ "$line" =~ ^login:\ (.+)$ ]]; then export CARREL_BAIKAL_USER="${BASH_REMATCH[1]}"
      elif [[ "$line" =~ ^password:\ (.+)$ ]]; then export CARREL_BAIKAL_PASS="${BASH_REMATCH[1]}"
      fi ;;
    sftpgo)
      if [[ "$line" =~ ^https?:// ]]; then export CARREL_SFTPGO_URL="$line"
      elif [[ "$line" =~ ^login:\ (.+)$ ]]; then export CARREL_SFTPGO_USER="${BASH_REMATCH[1]}"
      elif [[ "$line" =~ ^password:\ (.+)$ ]]; then export CARREL_SFTPGO_PASS="${BASH_REMATCH[1]}"
      fi ;;
  esac
done < "$CREDS"

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
