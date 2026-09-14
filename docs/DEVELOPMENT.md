# Development

## Prerequisites and pinned tools

Install mise. The repository pins Go 1.27.1, Node.js 24.21.0, and pnpm 10.34.5 in `mise.toml`; `web/package.json` also pins pnpm. SQLite is supplied by the pure-Go `modernc.org/sqlite` dependency, so no system SQLite, GCC, or CGO toolchain is required.

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
| `make test` | Run all Go tests, including on-disk SQLite integration tests. |
| `make build` | Build the Go executable and production frontend bundle. |
| `make dev-backend` | Run the API and initialize its database. |
| `make dev-frontend` | Run Vite's development server. |

Generated `upaste`, `web/dist`, dependencies, runtime databases, uploaded data, and local secrets are ignored. Do not commit them. Before committing run `make format`, `make check`, `make test`, and `make build`, then inspect the diff, staged files, and status for secrets or artifacts.

## Runtime configuration

Configuration precedence is CLI, then environment, then default:

| Purpose | Environment | CLI | Default |
|---|---|---|---|
| Application address | `UPASTE_ADDR` | `-addr` | `127.0.0.1:8080` |
| Data directory | `UPASTE_DATA_DIR` | `-data-dir` | `./data` |

The data directory is resolved to an absolute clean path. The application creates `<data-dir>/upaste.db`; newly created directory and database modes are `0700` and `0600`. Existing operator-managed permissions are preserved. Examples:

```sh
make dev-backend
mise exec -- go run ./cmd/upaste -addr 0.0.0.0:8080 -data-dir /srv/upaste
UPASTE_DATA_DIR=/tmp/upaste-dev make dev-backend
```

The loopback default is intentional. Binding externally requires explicit operator configuration and an appropriate trusted reverse proxy/firewall.

## Database and migrations

`internal/database/migrations/*.sql` is embedded into the binary. Migrations are forward-only, monotonically versioned, and use SQLite `PRAGMA user_version`; add a migration rather than editing an already released one. The application refuses a database newer than its supported schema. Database tests use `t.TempDir()` and require no external service.

The initial pool limit is four open and four idle connections. This is a conservative starting point, not benchmark-derived tuning. Connection hardening and its tests are described in [ADR 0008](adr/0008-sqlite-driver.md).

## Endpoint

```sh
curl -i http://127.0.0.1:8080/healthz
```

`GET /healthz` returns HTTP 200 and `application/json; charset=utf-8`. It is a lightweight liveness endpoint and does not query SQLite. No product HTTP endpoint exists in Phase 1.

## Go formatting scope

`scripts/gofmt` discovers Go files across the repository, including `internal/`. It excludes dependency directories (`vendor/` and `node_modules/`), directories named `generated`, and files carrying Go's standard `// Code generated ... DO NOT EDIT.` marker. Both `make check` and CI call its check mode, which lists unformatted files and exits non-zero; `make format` calls its write mode over the same file set.

## CI

CI runs project-wide formatting, vet, all Go tests/build, a frozen pnpm install, TypeScript checking, and the frontend build. Pure-Go on-disk SQLite integration tests run without system packages. Action references are immutable SHAs annotated with their upstream major tag in the workflow.
