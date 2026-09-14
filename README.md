# uPaste

uPaste is a new self-hosted application for sharing text and files. Security, reliability, and simple single-server operation are its priorities.

## Phase 4 status

Phase 4 implements Standard File Shares: streamed multipart upload to local object storage, metadata persistence, isolated attachment-only file delivery, owner expiration updates/deletion, and Range/HEAD download support. Standard and zero-knowledge Encrypted Text behavior remains unchanged; no product UI is added.

> Phase 4 does not yet include rate limiting or public-instance abuse controls. The anonymous creation endpoint is not production-ready for unrestricted Internet exposure.

Encrypted Files, expiration cleanup/reconciliation, accounts, search/listing, and product frontend UI are not implemented.

## Quick start

Install [mise](https://mise.jdx.dev/), then:

```sh
mise install
make install
make test check build
make dev-backend   # http://127.0.0.1:8080/healthz
make dev-frontend  # Vite development server
```

The backend creates `./data/upaste.db` by default. Configuration precedence is CLI over environment over defaults:

| Setting | Environment | CLI | Default |
|---|---|---|---|
| Listen address | `UPASTE_ADDR` | `-addr` | `127.0.0.1:8080` |
| Data directory | `UPASTE_DATA_DIR` | `-data-dir` | `./data` |

The resolved data directory is normalized to an absolute clean path. Binding beyond loopback must be an explicit operator choice.

See the [Phase 4 API](docs/API.md), [development guide](docs/DEVELOPMENT.md), [product scope](docs/PRODUCT.md), [architecture](docs/ARCHITECTURE.md), and [security invariants](docs/SECURITY.md).

No license has been selected.
