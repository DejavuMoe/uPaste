# uPaste

uPaste is a new self-hosted application for sharing text and files. Security, reliability, and simple single-server operation are its priorities.

## Phase 0.1 status

Phase 0.1 provides only the hardened project foundation: a Go HTTP service with `GET /healthz`, a minimal React shell, developer tooling, CI, and frozen architecture/security contracts. Share CRUD, persistence, uploads, encryption, authorization, rate limiting, and production UI are not implemented.

## Quick start

Install [mise](https://mise.jdx.dev/), then:

```sh
mise install
make install
make test check build
make dev-backend   # http://127.0.0.1:8080/healthz
make dev-frontend  # Vite development server
```

See [development](docs/DEVELOPMENT.md), [product scope](docs/PRODUCT.md), [architecture](docs/ARCHITECTURE.md), and [security invariants](docs/SECURITY.md).

No license has been selected.
