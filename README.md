# uPaste

uPaste is a new self-hosted application for sharing text and files. Security, reliability, and simple single-server operation are its priorities.

## Phase 6 status

Phase 6 freezes the frontend product contract, information architecture, wireframes, interaction state matrix, visual language, and implementation boundaries ([FRONTEND_PRODUCT_SPEC](docs/FRONTEND_PRODUCT_SPEC.md), [FRONTEND_INTERACTION_SPEC](docs/FRONTEND_INTERACTION_SPEC.md), [FRONTEND_VISUAL_SPEC](docs/FRONTEND_VISUAL_SPEC.md), and [ADR 0013](docs/adr/0013-frontend-product-architecture.md)). No production runtime code was changed.

> These application-layer controls target private self-hosted deployments and complement trusted reverse-proxy/network protections; they do not provide DDoS resistance or make uPaste a public multi-tenant Pastebin. There is no malware scanning, accounts, moderation, distributed limiter, or storage quota.

Encrypted Files, production React UI implementation, search/listing, and deployment packaging are not implemented.

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
| Application address | `UPASTE_ADDR` | `-addr` | `127.0.0.1:8080` |
| File listener address | `UPASTE_FILE_ADDR` | `-file-addr` | `127.0.0.1:8081` |
| Public file origin | `UPASTE_FILE_ORIGIN` | `-file-origin` | `http://127.0.0.1:8081` |
| Data directory | `UPASTE_DATA_DIR` | `-data-dir` | `./data` |

The resolved data directory is normalized to an absolute clean path. Binding beyond loopback must be an explicit operator choice.

See the [frontend product spec](docs/FRONTEND_PRODUCT_SPEC.md), [interaction spec](docs/FRONTEND_INTERACTION_SPEC.md), [visual spec](docs/FRONTEND_VISUAL_SPEC.md), [Phase 4 API](docs/API.md), [development guide](docs/DEVELOPMENT.md), [product scope](docs/PRODUCT.md), [architecture](docs/ARCHITECTURE.md), and [security invariants](docs/SECURITY.md).

No license has been selected.
