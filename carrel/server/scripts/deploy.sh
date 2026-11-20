#!/usr/bin/env bash
# Deploys what is committed to carrel.mixfed.ru (deploy/carrel.mixfed.ru):
#
#   carrel/server/scripts/deploy.sh
#
# The tree is packed as for the Linux gates (scripts/linux-ship.sh: this
# repository at HEAD as carrel/, Typecase at the client plan's commit as
# typecase/), unpacked into /opt/carrel/src on the host, and the image is
# built and started there by docker compose. The container listens on the
# loopback only; apache2 is not touched — its vhost is carrel.conf beside
# the compose file, put in place by hand (the README there says when).
# The data volume stays across deploys.
set -euo pipefail

host=${CARREL_DEPLOY_HOST:-root@carrel.mixfed.ru}
here=$(cd "$(dirname "$0")/.." && pwd)
root=$(cd "$here/../.." && pwd)

if [[ -n "$(git -C "$root" status --porcelain)" ]]; then
  echo "the tree has uncommitted changes: deploy what is committed, from a clean tree" >&2
  exit 1
fi
commit=$(git -C "$root" rev-parse --short=8 HEAD)
# The server's version: the highest of its CHANGELOG.
version=$(sed -n 's/^## v\([0-9][0-9.]*\) .*/\1/p' "$here/CHANGELOG.md" | sort -V | tail -1)

out=$(mktemp -d)
bash "$here/scripts/linux-ship.sh" "$out/carrel.tar" >/dev/null
scp -q "$out/carrel.tar" "$host:/tmp/carrel-deploy-$commit.tar"
ssh "$host" "mkdir -p /opt/carrel"
scp -q "$root/deploy/carrel.mixfed.ru/docker-compose.yml" "$root/deploy/carrel.mixfed.ru/carrel.conf" "$host:/opt/carrel/"
rm -rf "$out"

ssh "$host" bash -s -- "$commit" "$version" <<'REMOTE'
set -euo pipefail
commit=$1 version=$2
cd /opt/carrel
rm -rf src.new && mkdir src.new
tar -xf "/tmp/carrel-deploy-$commit.tar" -C src.new
rm -f "/tmp/carrel-deploy-$commit.tar" src.new/run.sh
rm -rf src && mv src.new src
CARREL_VERSION=$version CARREL_COMMIT=$commit docker compose up -d --build
# Up and answering on the loopback: the first page, or the way to it.
for _ in $(seq 1 30); do
  code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8090/ || true)
  case $code in 200|302|303) echo "carrel $version ($commit) answers $code on 127.0.0.1:8090"; exit 0;; esac
  sleep 1
done
echo "carrel does not answer on 127.0.0.1:8090" >&2
docker compose logs --tail 40 carrel >&2
exit 1
REMOTE
