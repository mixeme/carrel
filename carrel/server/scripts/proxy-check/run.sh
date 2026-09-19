#!/usr/bin/env bash
# Live check of Carrel behind a real nginx (S-28.1, S-27): Carrel, nginx and
# Radicale in a network of their own, nginx on 127.0.0.1:18090 only.
# Nothing of SelfPost is touched; everything is removed at the end.
# From carrel/server on the Linux machine (docs/plan/server.md, «Linux-машина»):
#   bash scripts/proxy-check/run.sh "$PWD" "$PWD/scripts/proxy-check"
set -uo pipefail
src=$1
here=$2
net=carrel-proxy-check
cleanup() {
  docker rm -f carrel-pc nginx-pc radicale-pc >/dev/null 2>&1
  docker network rm $net >/dev/null 2>&1
  docker volume rm -f carrel-pc-data >/dev/null 2>&1
}
cleanup
set -e
docker build -q -t carrel:proxy-check --build-arg VERSION=proxy-check --build-arg COMMIT=local "$src" >/dev/null
docker network create --subnet 172.30.99.0/24 $net >/dev/null
docker run -d --name radicale-pc --network $net \
  -v "$src/testdata/radicale/config:/config/config:ro" -v "$src/testdata/radicale/users:/config/users:ro" \
  -p 127.0.0.1:15233:5232 tomsquest/docker-radicale:3.5.4.0 >/dev/null
docker run -d --name carrel-pc --network $net --ip 172.30.99.20 \
  --read-only --cap-drop=ALL --tmpfs /tmp:rw,noexec,nosuid,size=64m -v carrel-pc-data:/data \
  -e CARREL_DAVLOOM_PORT=8081 -e CARREL_TRUSTED_PROXIES=172.30.99.10 \
  -e CARREL_DAV_SSRF_ALLOWLIST=radicale-pc -e CARREL_LOG_LEVEL=info \
  carrel:proxy-check >/dev/null
set +e
status=0
for variant in tight open; do
  docker rm -f nginx-pc >/dev/null 2>&1
  docker run -d --name nginx-pc --network $net --ip 172.30.99.10 -p 127.0.0.1:18090:80 \
    -v "$here/nginx-$variant.conf:/etc/nginx/conf.d/default.conf:ro" nginx:1.27-alpine >/dev/null
  for _ in $(seq 1 50); do curl -sf -H 'Host: nginx-pc' http://127.0.0.1:18090/healthz >/dev/null && break; sleep 0.2; done
  echo "== nginx: $variant =="
  python3 "$here/client.py" "$variant" || status=1
done
echo "== carrel log (warnings and errors) =="
docker logs carrel-pc 2>&1 | grep -E 'level=(WARN|ERROR)' | head -20
cleanup
exit $status
