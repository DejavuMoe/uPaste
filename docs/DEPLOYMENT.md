# uPaste native Linux deployment

This guide covers the supported native deployment: one `upaste` binary, one
SQLite database plus local object store under `/var/lib/upaste`, native systemd
supervision, and a trusted reverse proxy terminating TLS for two distinct
origins. Docker and container packaging are intentionally not provided.

## Supported layout

| Path | Owner:group | Mode | Purpose |
|---|---|---|---|
| `/usr/local/bin/upaste` | `root:root` | `0755` | Single production binary |
| `/etc/upaste` | `root:upaste` | `0750` | Configuration directory |
| `/etc/upaste/upaste.env` | `root:upaste` | `0640` | Environment file read by systemd |
| `/var/lib/upaste` | `upaste:upaste` | `0700` | SQLite database and object store |

The service runs as the unprivileged `upaste` user and group. Never run uPaste
as root. The application listens on loopback only:

```text
127.0.0.1:8080   application origin: frontend, API, raw text, healthz
127.0.0.1:8081   File origin: /f/:id attachments only
```

## 1. Create the service account and directories

```sh
sudo groupadd --system upaste
sudo useradd --system --gid upaste --home-dir /var/lib/upaste \
  --shell /usr/sbin/nologin upaste

sudo install -d -o root -g upaste -m 0750 /etc/upaste
sudo install -d -o upaste -g upaste -m 0700 /var/lib/upaste
```

## 2. Install a release archive

Download the archive for your architecture and the matching `SHA256SUMS`. Always
verify the checksum before extracting or installing.

```sh
grep ' upaste-v0.1.0-linux-amd64.tar.gz$' SHA256SUMS | sha256sum --check -
tar -xzf upaste-v0.1.0-linux-amd64.tar.gz
cd upaste-v0.1.0-linux-amd64
./upaste --version
cat BUILDINFO
```

`./upaste --version` must not contact the network, create a data directory, or
start listeners. `BUILDINFO` mirrors the embedded version, commit, build date,
Go version, and target for operator verification. Install the binary atomically:

```sh
sudo install -o root -g root -m 0755 ./upaste /usr/local/bin/.upaste.new
sudo mv -f /usr/local/bin/.upaste.new /usr/local/bin/upaste
```

The release archive also contains `DEPLOYMENT.md`, `BUILDINFO`, `upaste.service`,
`upaste.env.example`, `nginx.conf.example`, and `Caddyfile.example`.

## 3. Configure the service

```sh
sudo install -o root -g upaste -m 0640 upaste.env.example /etc/upaste/upaste.env
sudoedit /etc/upaste/upaste.env
```

Production-safe starting values:

```text
UPASTE_ADDR=127.0.0.1:8080
UPASTE_FILE_ADDR=127.0.0.1:8081
UPASTE_FILE_ORIGIN=https://files.example.com
UPASTE_DATA_DIR=/var/lib/upaste
UPASTE_TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128
```

`UPASTE_FILE_ORIGIN` MUST be a distinct public origin from the application
origin. Never route `paste.example.com/f/*` to the File listener as a substitute
for `files.example.com`.

`UPASTE_TRUSTED_PROXY_CIDRS` controls whether `X-Forwarded-For` is trusted for
client identity and rate limiting. Include only the directly connected reverse
proxy address. For a same-host proxy, `127.0.0.1/32,::1/128` is appropriate.
Leave it empty when uPaste is reached directly. Never use `0.0.0.0/0` or `::/0`,
and do not add public CDN ranges without a separate reviewed design.

## 4. Install and start the systemd service

```sh
sudo install -o root -g root -m 0644 upaste.service /etc/systemd/system/upaste.service
sudo systemctl daemon-reload
sudo systemctl enable --now upaste
systemctl status upaste
```

The unit loads `/etc/upaste/upaste.env`, runs as `upaste:upaste`, uses
`UMask=0077`, restarts on genuine failure, and allows a graceful `SIGTERM`
shutdown. It also applies systemd hardening including `NoNewPrivileges=true`,
`ProtectSystem=strict`, `ProtectHome=true`, empty capabilities, and
`ReadWritePaths=/var/lib/upaste`.

## 5. Reverse proxy and firewall

Public origins and upstreams:

```text
https://paste.example.com  -> 127.0.0.1:8080
https://files.example.com  -> 127.0.0.1:8081
```

Use `deploy/nginx/upaste.conf.example` or `deploy/caddy/Caddyfile.example` as a
starting point. Both examples keep the app and File upstreams separate, forward
`Host`, `X-Forwarded-For`, and `X-Forwarded-Proto`, allow the File upload body
above the 64 MiB limit, and keep proxy timeouts above the application's
10-minute File transfer deadline. Do not add `Access-Control-Allow-Origin`
headers to either origin.

Intended network exposure:

```text
public:   TCP 80/443 on the reverse proxy only
private:  127.0.0.1:8080 application listener
          127.0.0.1:8081 File listener
```

Do not expose 8080 or 8081 directly to the Internet. TLS terminates at the
trusted reverse proxy. uPaste does not provide in-process TLS.

## 6. Health checks and logs

```sh
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8081/healthz
```

`GET /healthz` on the application listener returns `200`. The File listener is
intentionally attachment-only, so `/healthz` there returns `404`; a TCP connect
check or a known `/f/:id` download is sufficient for that listener.

Logs are structured JSON on stdout/stderr and are captured by the systemd
journal:

```sh
journalctl -u upaste
journalctl -u upaste -f
```

Logs never intentionally contain request bodies, `Authorization` values, owner
tokens, encryption keys, or encrypted-share plaintext.

## Shutdown behavior and recovery

systemd sends `SIGTERM`. uPaste stops accepting new connections and allows the
HTTP servers up to 10 seconds for active handlers to finish. Ordinary API
requests complete quickly. An in-flight 64 MiB File transfer may be abandoned;
an interrupted upload never commits a Share, and any staging object is either
removed immediately or becomes eligible for the existing 30-minute orphan
reconciliation. Committed Shares remain intact, and restart reopens the same
SQLite database and object store. `make qualify-release` includes a shutdown
under active upload followed by restart and read verification.

## 7. Backup

SQLite metadata and File objects are one logical state. Use a cold backup:

```sh
sudo systemctl stop upaste
sudo install -d -o root -g root -m 0700 /var/backups/upaste
sudo tar -C /var/lib -czf "/var/backups/upaste/upaste-$(date -u +%Y%m%dT%H%M%SZ).tar.gz" upaste
sudo systemctl start upaste
curl -fsS http://127.0.0.1:8080/healthz
```

The backup tree must include `upaste.db`, any `upaste.db-wal`/`upaste.db-shm`
side files that exist at shutdown, and the entire `objects/` directory. Do not
copy only `upaste.db` while the service is running and mutating File state.

## 8. Restore

```sh
sudo systemctl stop upaste
sudo mv /var/lib/upaste "/var/lib/upaste.pre-restore.$(date -u +%Y%m%dT%H%M%SZ)"
sudo tar -C /var/lib -xzf /var/backups/upaste/upaste-<timestamp>.tar.gz
sudo chown -R upaste:upaste /var/lib/upaste
sudo chmod 0700 /var/lib/upaste
sudo systemctl start upaste
curl -fsS http://127.0.0.1:8080/healthz
```

Then fetch a known Standard Text Share and a known File URL to confirm both
payload paths. Do not merge partial object directories or database snapshots
from unrelated backups.

## 9. Upgrade

```sh
grep ' upaste-v0.1.0-linux-amd64.tar.gz$' SHA256SUMS | sha256sum --check -
tar -xzf upaste-v0.1.0-linux-amd64.tar.gz
cd upaste-v0.1.0-linux-amd64
./upaste --version
# Take a cold backup first (section 7).
sudo install -o root -g root -m 0755 ./upaste /usr/local/bin/.upaste.new
sudo systemctl stop upaste
sudo mv -f /usr/local/bin/.upaste.new /usr/local/bin/upaste
sudo systemctl start upaste
systemctl status upaste
curl -fsS http://127.0.0.1:8080/healthz
```

Verify the frontend on the application origin and a File download on the File
origin after every upgrade.

## 10. Rollback

Database migrations are forward-only. Replacing the binary with an older version
is safe only when the newer version has not advanced the SQLite schema beyond
what the older binary supports. When a release performed a migration and a
rollback is required:

```sh
sudo systemctl stop upaste
# Restore the pre-upgrade complete backup tree (section 8).
sudo install -o root -g root -m 0755 <previous-version>/upaste /usr/local/bin/.upaste.new
sudo mv -f /usr/local/bin/.upaste.new /usr/local/bin/upaste
sudo systemctl start upaste
curl -fsS http://127.0.0.1:8080/healthz
```

Do not assume that swapping only the binary rolls back persistent state.

## 11. Smoke verification

```sh
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/ | grep -q 'id="root"'
curl -fsS -H 'Content-Type: application/json' \
  -d '{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"deployment smoke"},"expires_at":null}' \
  http://127.0.0.1:8080/api/v1/shares
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/f/AAAAAAAAAAAAAAAAAAAAAA
```

The last command must return `404`: the application origin never serves File
bytes.

## 12. Troubleshooting

- **Service cannot bind:** confirm `UPASTE_ADDR` and `UPASTE_FILE_ADDR` differ
  and are above port 1024 if `CapabilityBoundingSet=` is empty.
- **Permission failures:** `/var/lib/upaste` must be writable by `upaste:upaste`
  (`0700`), while `/etc/upaste/upaste.env` stays `root:upaste 0640`.
- **Rate limits or logs show the proxy address:** check
  `UPASTE_TRUSTED_PROXY_CIDRS` contains the proxy's actual directly connected
  address and the proxy sends `X-Forwarded-For`.
- **File downloads point at the wrong host:** `UPASTE_FILE_ORIGIN` must match
  the public File origin exactly and use HTTPS in production.
- **Uploads fail at the proxy:** raise the reverse-proxy body limit above the
  64 MiB File limit and keep read/send timeouts at or above 600 seconds.
