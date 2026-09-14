# Development

## Prerequisites and pinned tools

Install mise. The repository pins Go 1.27.1, Node.js 24.21.0, and pnpm 10.34.5 in `mise.toml`; `web/package.json` also pins pnpm. SQLite is supplied by the pure-Go `modernc.org/sqlite` dependency, so no system SQLite, GCC, or CGO toolchain is required. Browser crypto tests use Vitest 5.0.0 with Node's standards-compatible Web Crypto globals and need no DOM/browser service.

```sh
mise install
make install
```

pnpm is the only JavaScript package manager. Commit exactly `web/pnpm-lock.yaml`; do not generate npm or Yarn lockfiles.

## Commands

| Command | Purpose |
|---|---|
| `make format` | Format all project-owned Go source. |
| `make check` | Fail on unformatted project Go source, run `go vet`, and strict TypeScript checking. |
| `make test` | Run all Go/SQLite tests and frontend Vitest protocol tests. |
| `make build` | Build the Go executable and production frontend bundle. |
| `make dev-backend` | Run the API and initialize its database. |
| `make dev-frontend` | Run Vite's development server. |

Generated `upaste`, `web/dist`, dependencies, runtime databases, uploaded data, and local secrets are ignored. Do not commit them. Before committing run `make format`, `make check`, `make test`, and `make build`, then inspect the diff, staged files, and status for secrets or artifacts.

## Runtime configuration

Configuration precedence is CLI, then environment, then default:

| Purpose | Environment | CLI | Default |
|---|---|---|---|
| Application address | `UPASTE_ADDR` | `-addr` | `127.0.0.1:8080` |
| File listener address | `UPASTE_FILE_ADDR` | `-file-addr` | `127.0.0.1:8081` |
| Public file origin | `UPASTE_FILE_ORIGIN` | `-file-origin` | `http://127.0.0.1:8081` |
| Data directory | `UPASTE_DATA_DIR` | `-data-dir` | `./data` |

The data directory is resolved to an absolute clean path. The application creates `<data-dir>/upaste.db` and `<data-dir>/objects/`; newly created directories/database/object modes are `0700`/`0600`. File and app addresses must differ. File origin must be an absolute HTTP(S) origin without path, query, fragment, or userinfo; it is never inferred from Host or forwarded headers. Existing operator-managed permissions are preserved. Examples:

```sh
make dev-backend
mise exec -- go run ./cmd/upaste -addr 0.0.0.0:8080 -data-dir /srv/upaste
UPASTE_DATA_DIR=/tmp/upaste-dev make dev-backend
```

The loopback default is intentional. Binding externally requires explicit operator configuration and an appropriate trusted reverse proxy/firewall.

## Transfer deadlines

Servers keep 5-second header reads, 15-second ordinary read/write deadlines, 60-second idle connections, and 32 KiB headers. A valid multipart File create extends that request body's read deadline and eventual response write deadline to 10 minutes before multipart parsing; File GET/HEAD extends only that response write deadline to 10 minutes before delivery. This bounded exception is needed for the 64 MiB transfer limit and does not add rate limiting or make ordinary JSON slow-body requests long-lived.

## Database and migrations

`internal/database/migrations/*.sql` is embedded into the binary. Migrations are forward-only, monotonically versioned, and use SQLite `PRAGMA user_version`; add a migration rather than editing an already released one. Schema version 2 adds `standard_text_payloads`; version 3 adds `encrypted_text_payloads`; version 4 adds `file_payloads` and File invariant triggers. Migrations 0001–0003 remain immutable. The application refuses a database newer than its supported schema. Database tests use `t.TempDir()` and require no external service.

The pool limit is four open and four idle connections. Write transactions acquire SQLite's immediate lock so concurrent read-then-write owner operations wait under the existing five-second busy timeout instead of failing with a stale deferred snapshot. The concurrency regression runs four updates and four independent creates together. Connection hardening and its tests are described in [ADR 0008](adr/0008-sqlite-driver.md).

## Endpoint

```sh
curl -i http://127.0.0.1:8080/healthz
```

`GET /healthz` returns HTTP 200 and `application/json; charset=utf-8`. It is a lightweight liveness endpoint and does not query SQLite. Standard/Encrypted Text and Standard File routes and safe placeholder examples are documented in [API.md](API.md). The file listener is a separate origin and exposes only `/f/{id}`; configure reverse proxies with distinct origins rather than Host-based multiplexing. Request decoding uses Go JSON v2 strict defaults and explicit unknown-member rejection; responses retain the Phase 2 encoder for compatibility. Never put an owner token in a URL or logs; send it only in an Authorization bearer header.

## Go formatting scope

`scripts/gofmt` discovers Go files across the repository, including `internal/`. It excludes dependency directories (`vendor/` and `node_modules/`), directories named `generated`, and files carrying Go's standard `// Code generated ... DO NOT EDIT.` marker. Both `make check` and CI call its check mode, which lists unformatted files and exits non-zero; `make format` calls its write mode over the same file set.

## CI

CI runs project-wide formatting, vet, all Go tests/build, a frozen pnpm install, TypeScript checking, Vitest browser-crypto tests in Node's standards-compatible Web Crypto environment, and the frontend build. Pure-Go on-disk SQLite integration tests run without system packages. Action references are immutable SHAs annotated with their upstream major tag in the workflow.
