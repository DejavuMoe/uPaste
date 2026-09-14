# uPaste

uPaste is a new self-hosted application for sharing text and files. Security, reliability, and simple single-server operation are its priorities.

## Phase 3 status

Phase 3 implements anonymous Standard and zero-knowledge Encrypted Text Share lifecycles. Encrypted plaintext and its AES-256-GCM key stay in the browser; the server stores only protocol metadata, nonce, and authenticated ciphertext. The browser-compatible crypto library is implemented and tested without adding product UI.

> Phase 3 does not yet include rate limiting or public-instance abuse controls. The anonymous creation endpoint is not production-ready for unrestricted Internet exposure.

File Shares, expiration cleanup, accounts, search/listing, and product frontend UI are not implemented.

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

See the [Phase 3 API](docs/API.md), [development guide](docs/DEVELOPMENT.md), [product scope](docs/PRODUCT.md), [architecture](docs/ARCHITECTURE.md), and [security invariants](docs/SECURITY.md).

No license has been selected.
