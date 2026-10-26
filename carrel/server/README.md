# Carrel server

One desk for the contacts, calendars, tasks, notes and files you already keep
on CalDAV, CardDAV and WebDAV servers. Carrel reads and writes them in place.
It stores no copy of your records: only your connections, sealed under a key
that comes from your password.

The binary is `cmd/carrel`. It serves the web interface and, when you turn it
on, **davloom**: a DAV proxy that gives a device one account in place of
every server you connected.

The specification is in [`docs/spec/`](../../docs/spec/) and the plan in
[`docs/plan/server.md`](../../docs/plan/server.md). What changed, version by
version, is in [`CHANGELOG.md`](CHANGELOG.md).

## Running

```sh
docker build -t carrel .
docker run -d --name carrel \
  --read-only --cap-drop=ALL --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  -v carrel-data:/data \
  -p 127.0.0.1:8080:8080 \
  -e CARREL_PUBLIC_URL=https://carrel.example.org \
  -e CARREL_TRUSTED_PROXIES=127.0.0.1 \
  carrel
```

The first visit sets up the administrator. Everything the instance keeps is
in the one volume: the server key and the encrypted state.

### Configuration

Carrel reads `settings.env` in the data folder, and then the environment of
the process; the environment wins. The complete list of variables is in C-12
of [`docs/spec/carrel-common.md`](../../docs/spec/carrel-common.md).
The variables that matter on day one:

| Variable | What it is |
|---|---|
| `CARREL_PUBLIC_URL` | The address people open. Links in letters start with it, and cookies are marked Secure for an https one. Admin → Installation can write it for you. |
| `CARREL_TRUSTED_PROXIES` | The reverse proxy's address. Only then does Carrel believe `X-Forwarded-*`. |
| `CARREL_DAVLOOM_PORT` | Turns davloom on, on its own port. It must differ from `CARREL_PORT`. |
| `CARREL_DAV_SSRF_ALLOWLIST` | Hosts on your own network that Carrel may reach. Private addresses are refused otherwise. |
| `CARREL_TIMEZONE` | The instance's time zone. |

These are set on the admin screens and kept in the state rather than in
variables:
- the mail relay, the letters people may send and their ceilings;
- the instance name;
- the lifetimes of sessions and invitations.

### Behind a reverse proxy

Carrel does not terminate TLS. Put it behind a proxy, with both ports bound
to the loopback. **Admin → Installation** calls Carrel at its public address
and reports eleven checks, each with what to change. nginx:

```nginx
server {
    listen 443 ssl;
    server_name carrel.example.org;
    client_max_body_size 0;          # Carrel sets no ceiling on a file
    proxy_read_timeout 120s;         # longer than CARREL_PROGRESS_TOTAL_TIMEOUT

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    # davloom. No path after the address: with one ("…:8081/") nginx decodes
    # the path first, the %2F in a collection's address becomes /, and
    # requests miss the collection. davloom removes the /dav prefix itself.
    location /dav/ {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

The event stream needs no `proxy_buffering off`: Carrel sends
`X-Accel-Buffering: no`, and nginx honours it. With Apache, set
`AllowEncodedSlashes NoDecode` and `ProxyPass … nocanon` for `/dav/`.

## Checks

| Command | What it runs |
|---|---|
| `scripts/gates.sh` | gofmt, vet, the tests of every package, and the spec's own check. `--fix` formats. |
| `go test -tags=integration ./integration/` | The live set against real servers. Credentials come from `myenv/carrel/credentials.txt` in the nearest folder above the checkout (or the file `CARREL_CREDENTIALS` names), which is not in the repository. A server without credentials is skipped. |
| `scripts/linux-gates.sh` | On Linux: the key file's permissions, the image and its first start, and Radicale. |
| `scripts/proxy-check/run.sh` | On Linux: Carrel, nginx and Radicale in Docker. It runs the installation check and davloom through nginx, once with nginx at its defaults and once set up as above. |

## What a person checks

Not everything can be checked by a machine (S-30.4). What remains is listed
here, so that nobody takes it as done.

- **Third-party clients.** Make a record in DAVx5, Thunderbird or Apple
  Calendar, read and change it in Carrel, then open it there again.
  Conflicts are the case that matters.
- **A device profile on a real device**: the Apple profile applied on an
  iPhone, and the DAVx5 code scanned from a phone.
- **A restore from an archive a month old**, not from the copy just made.
- **The letter as the recipient sees it**, in a real mail client.

Two checks that S-30.4 leaves to a person now run on a machine:
- **A real reverse proxy:** `scripts/proxy-check/`.
- **A letter delivered to a real mailbox:** `TestSelfPostRelayDelivers`.

Both were green on 19.09.2026. What they cannot judge is what a person sees:
how the letter looks, and how the proxy behaves on your own host.

## License

AGPL-3.0-or-later. See [LICENSE](LICENSE).
