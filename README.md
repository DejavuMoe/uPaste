# uPaste

uPaste is a self-hosted application for sharing text and files. Security, reliability, and simple single-server operation are its priorities.

## Current status

Phase 8 is complete. The production React frontend is built by Vite and embedded in the Go application binary, so the normal production artifact is one self-contained executable plus a local data directory.

Implemented:

- Standard Text, Encrypted Text, and Markdown Text Shares.
- Single-file Standard File Shares with attachment-only delivery from a separate listener/origin.
- Capability-based owner update and delete; no accounts.
- Synchronous expiration enforcement and asynchronous purge/reconciliation.
- Process-local, trusted-proxy-aware rate limiting and File concurrency gates.
- Production React UI embedded in the Go binary with strict SPA routing, immutable asset caching, and a restrictive CSP.

Not implemented: Encrypted File Shares, public listing/search, accounts, malware scanning, hard storage quotas, distributed rate limiting, and deployment packaging (Docker, systemd, release archives). See [docs/PRODUCT.md](docs/PRODUCT.md) for the frozen V1 scope.

## Quick start

Install [mise](https://mise.jdx.dev/), then:

```sh
mise install
make install
make check test build    # build creates ./upaste with the embedded frontend
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

## Development vs production frontend

- Development: `make dev-backend` plus `make dev-frontend`. Vite serves the UI and proxies `/api` and `/raw` to the Go server.
- Production: `make build` runs the TypeScript check and Vite build, stages the output into `internal/webapp/dist/`, and compiles Go with `-tags production`. The staged directory and `web/dist/` are generated, Git-ignored, and safe to delete.

## Documentation

- [Product scope](docs/PRODUCT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [HTTP API](docs/API.md)
- [Development guide](docs/DEVELOPMENT.md)
- [Security invariants](docs/SECURITY.md)
- [Threat model](docs/THREAT_MODEL.md)
- [Frontend product](docs/FRONTEND_PRODUCT_SPEC.md), [interaction](docs/FRONTEND_INTERACTION_SPEC.md), and [visual](docs/FRONTEND_VISUAL_SPEC.md) specifications
- [Architecture decisions](docs/adr/)

No license has been selected.
