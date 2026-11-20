# Carrel on carrel.mixfed.ru

The first working version (v0.4.0, docs/development.md) runs here: one
container on the loopback, behind the host's apache2. The host is shared:
SelfPost (`127.0.0.1:8080`, mail on 25/465/587) and Wynd (`127.0.0.1:7676`)
live on it too — their containers, volumes and vhosts are not touched.

| What | Where |
|---|---|
| Pages | `127.0.0.1:8090` → `https://carrel.mixfed.ru/` |
| davloom | `127.0.0.1:8091` → `https://carrel.mixfed.ru/dav/` |
| Data (server key, encrypted state) | docker volume `carrel_carrel-data` |
| Compose, vhost, source | `/opt/carrel` (`docker-compose.yml`, `carrel.conf`, `src/`) |

## Deploy

From a clean checkout, what is committed:

```bash
carrel/server/scripts/deploy.sh
```

It packs the tree as the Linux gates do, builds the image on the host and
restarts the container; the volume stays. apache2 is not touched.

## First start

An empty instance gives the administrator to whoever opens it first
(S-10.1: no default password). So the order is:

1. Deploy: the container answers on the loopback only.
2. The owner opens it through a tunnel and creates the administrator:
   `ssh -N -L 8090:127.0.0.1:8090 root@carrel.mixfed.ru`, then
   `http://localhost:8090/`.
3. The certificate, as SelfPost's and Wynd's are made:
   `certbot certonly --apache -d carrel.mixfed.ru`.
4. The vhost: `cp /opt/carrel/carrel.conf /etc/apache2/sites-available/`,
   `a2ensite carrel`, `apache2ctl configtest`, `systemctl reload apache2`
   (a reload, not a restart: SelfPost and Wynd keep their connections).
5. Admin → Installation: the eleven checks at the public address.
