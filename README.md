# uPaste

uPaste is a new self-hosted application for sharing text and files. Security, reliability, and simple single-server operation are its priorities.

## Phase 5 status

Phase 5 adds asynchronous expired-data purge, local-object reconciliation, trusted-proxy-aware process-local rate limits, and bounded File-transfer concurrency. Existing Standard/Encrypted Text and Standard File contracts remain unchanged.

> These application-layer controls target private self-hosted deployments and complement trusted reverse-proxy/network protections; they do not provide DDoS resistance or make uPaste a public multi-tenant Pastebin. There is no malware scanning, accounts, moderation, distributed limiter, or storage quota.

Encrypted Files, product UI, search/listing, and deployment packaging are not implemented.

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

See the [Phase 4 API](docs/API.md), [development guide](docs/DEVELOPMENT.md), [product scope](docs/PRODUCT.md), [architecture](docs/ARCHITECTURE.md), and [security invariants](docs/SECURITY.md).

No license has been selected.
