# uPaste

uPaste is a self-hosted application for sharing text and files. Security, reliability, and simple single-server operation are its priorities.

## Current status

Phase 10 is complete: uPaste has passed the pre-release security, race, fuzz, recovery, resource, and packaging qualification gate. The repository is ready for a deliberate first-release decision, but no version tag or GitHub Release has been created; release tooling and qualification are prepared only.

Implemented:

- Standard Text, Encrypted Text, and Markdown Text Shares.
- Single-file Standard File Shares with attachment-only delivery from a separate listener/origin.
- Capability-based owner update and delete; no accounts.
- Synchronous expiration enforcement and asynchronous purge/reconciliation.
- Process-local, trusted-proxy-aware rate limiting and File concurrency gates.
- Production React UI embedded in the Go binary with strict SPA routing, immutable asset caching, and a restrictive CSP.
- Deterministic `linux/amd64` and `linux/arm64` release archives with SHA-256 checksums and embedded version/commit/build metadata.
- Native systemd service, example environment file, and Nginx/Caddy two-origin reverse-proxy examples.
- Pre-release qualification covering race and fuzz parsers, concurrency and recovery stress, trusted-proxy spoofing, package/runtime integrity, automated backup/restore, and shutdown-under-load behavior.

Not implemented: Encrypted File Shares, public listing/search, accounts, malware scanning, hard storage quotas, distributed rate limiting, Docker/container packaging, and automatic updates. See [docs/PRODUCT.md](docs/PRODUCT.md) for the frozen V1 scope.

## Quick start

Install [mise](https://mise.jdx.dev/), then:

```sh
mise install
make install
make check test build    # build creates ./upaste with the embedded frontend
./upaste --version
./upaste                 # application and File listeners

make dev-backend         # API/File listeners without the embedded UI
make dev-frontend        # Vite development server for the UI
```

`make build` requires Go and Node/pnpm at build time. The resulting `./upaste` needs neither `web/dist` nor Node at runtime.

The backend creates `./data/upaste.db` and `./data/objects/` by default. Configuration precedence is CLI over environment over defaults:

| Setting | Environment | CLI | Default |
|---|---|---|---|
| Application address | `UPASTE_ADDR` | `-addr` | `127.0.0.1:8080` |
| File listener address | `UPASTE_FILE_ADDR` | `-file-addr` | `127.0.0.1:8081` |
| Public file origin | `UPASTE_FILE_ORIGIN` | `-file-origin` | `http://127.0.0.1:8081` |
| Trusted proxy CIDRs | `UPASTE_TRUSTED_PROXY_CIDRS` | `-trusted-proxy-cidrs` | empty |
| Data directory | `UPASTE_DATA_DIR` | `-data-dir` | `./data` |

The resolved data directory is normalized to an absolute clean path. Binding beyond loopback must be an explicit operator choice.

## Routing

Application origin:

- `/` — create a Share.
- `/s/:id` — public Share viewer.
- `/manage/:id` — owner management (requires the owner capability in browser memory).
- `/api/v1/*` — JSON/multipart API.
- `/raw/:id` — inert plain-text delivery for Standard Text.
- `/healthz` — database-independent liveness check.

File origin (separate listener): `/f/:id` only, always served as an attachment. The application listener never serves File bytes and returns 404 for `/f/*`.

## Release artifacts

`upaste --version` prints stable build metadata without touching configuration, SQLite, listeners, or maintenance:

```text
uPaste v0.1.0
commit: <full-git-sha>
built: <deterministic-utc-timestamp>
go: go1.27.1
```

Build deterministic release archives for `linux/amd64` and `linux/arm64`:

```sh
make dist VERSION=v0.1.0
make verify-dist VERSION=v0.1.0
```

`release/` receives `upaste-v0.1.0-linux-amd64.tar.gz`, `upaste-v0.1.0-linux-arm64.tar.gz`, and `SHA256SUMS`. Each archive contains `upaste`, `README.md`, `DEPLOYMENT.md`, `BUILDINFO` (version/commit/build date/Go/target), `upaste.service`, `upaste.env.example`, `nginx.conf.example`, and `Caddyfile.example`. Archives use normalized ownership, permissions, ordering, timestamps, and gzip headers; `make verify-dist` checks checksums, embedded metadata, AArch64 identity, amd64 standalone runtime and restart persistence, and a deterministic rebuild.

A draft-only GitHub release workflow exists at `.github/workflows/release.yml` for future `v*` tags. It validates with least privilege, runs the complete pre-release gate on the tagged source, makes no release on manual dispatch, and creates or refreshes a draft only when triggered by a real `v*` tag. It was not triggered by this phase: no version tag and no GitHub Release were created.

Run the complete deterministic pre-release gate locally with:

```sh
make qualify-release VERSION=v0.0.0-test
```

Longer fuzz campaigns (`make fuzz`), machine-specific performance baselines (`make benchmark`), and bounded concurrent load smoke (`make load-smoke`) remain explicit local/pre-release commands rather than every-push CI.

## First real release checklist (human action required)

1. Select the release version and review the scope.
2. Decide the license separately; no license has been selected.
3. Ensure `master` CI is green for the candidate commit.
4. Run `make qualify-release VERSION=<version>` locally or in CI.
5. Review release notes/scope; no changelog generator is required.
6. Decide the signed/annotated tag policy.
7. Push the `v*` tag intentionally.
8. Wait for the release workflow and inspect the draft release.
9. Verify `SHA256SUMS`, `BUILDINFO`, and `upaste --version` on the draft artifact.
10. Download and smoke-test the draft artifact.
11. Publish the draft manually only after review.

Phase 10 did not perform steps 1–11; they remain an explicit owner decision.

## Native deployment in brief

The supported production deployment is native Linux with systemd and a trusted reverse proxy for two distinct origins:

```text
https://paste.example.com  -> 127.0.0.1:8080   frontend, API, raw, healthz
https://files.example.com  -> 127.0.0.1:8081   /f/:id attachments only
```

1. Verify the archive checksum and extract it.
2. Install `upaste` to `/usr/local/bin/upaste` as `root:root 0755`.
3. Create the unprivileged `upaste` user/group and `/var/lib/upaste` (`upaste:upaste 0700`).
4. Install `upaste.env.example` as `/etc/upaste/upaste.env` (`root:upaste 0640`) and set `UPASTE_FILE_ORIGIN` to the public File origin.
5. Install `upaste.service` as `/etc/systemd/system/upaste.service`, then `systemctl enable --now upaste`.
6. Terminate TLS at the reverse proxy using the Nginx or Caddy example.
7. Verify `curl -fsS http://127.0.0.1:8080/healthz`.

See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for installation, permissions, firewall, backup, restore, upgrade, rollback, and troubleshooting details.

## Development vs production frontend

- Development: `make dev-backend` plus `make dev-frontend`. Vite serves the UI and proxies `/api` and `/raw` to the Go server.
- Production: `make build` runs the TypeScript check and Vite build, stages the output into `internal/webapp/dist/`, and compiles Go with `-tags production`. The staged directory and `web/dist/` are generated, Git-ignored, and safe to delete.

## Documentation

- [Product scope](docs/PRODUCT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [HTTP API](docs/API.md)
- [Native deployment](docs/DEPLOYMENT.md)
- [Development guide](docs/DEVELOPMENT.md)
- [Security invariants](docs/SECURITY.md)
- [Threat model](docs/THREAT_MODEL.md)
- [Frontend product](docs/FRONTEND_PRODUCT_SPEC.md), [interaction](docs/FRONTEND_INTERACTION_SPEC.md), and [visual](docs/FRONTEND_VISUAL_SPEC.md) specifications
- [Architecture decisions](docs/adr/)

No license has been selected.
