# Development

## Prerequisites and pinned tools

Install mise. The repository pins Go 1.27.1, Node.js 24.21.0, and pnpm 10.34.5 in `mise.toml`; `web/package.json` also pins pnpm. SQLite is supplied by the pure-Go `modernc.org/sqlite` dependency, so no system SQLite, GCC, or CGO toolchain is required. Browser crypto tests use Vitest 5.0.0 with Node's standards-compatible Web Crypto globals and need no DOM/browser service. Playwright is used for the real-browser suites.

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
| `make test` | Run all Go/SQLite tests and frontend Vitest tests. |
| `make build` | Full production build: TypeScript check, Vite build, clean embedding staging, then a single `-tags production` Go binary at `./upaste`. |
| `make prove-embedded` | `make build`, then run the isolated-binary proof (binary alone, fresh data dir, no `web/dist`). |
| `make e2e` | Existing 35-test Vite-backed real-browser suite. |
| `make prod-e2e` | Embedded-production real-browser suite against the built binary (no Vite), including the throttled slow-upload CSP/progress check. |
| `make dist VERSION=v0.1.0` | Build deterministic `linux/amd64` and `linux/arm64` release archives plus `SHA256SUMS` under `release/`. |
| `make verify-dist VERSION=v0.1.0` | Verify checksums, archive shape, embedded metadata, AArch64 identity, amd64 standalone runtime and restart persistence, and a deterministic rebuild. |
| `make deploy-check` | Validate the systemd unit and two-origin reverse-proxy examples, including `systemd-analyze verify` when available. |
| `make qualify-release VERSION=v0.0.0-test` | Complete pre-release gate: Go and race suites, repeated concurrency stress, deployment checks, deterministic amd64/arm64 packaging, packaged runtime verification, automated cold backup/restore proof, and shutdown-under-load proof. |
| `make fuzz` | Bounded local/pre-release Go fuzz campaign for the parser boundaries. Not part of every CI run. |
| `make benchmark` | Repeatable local performance baseline for release awareness. Informational only; not CI-gated. |
| `make e2e` / `pnpm --dir web e2e:public` / `pnpm --dir web e2e:cap` | Private Vite E2E, public Turnstile-mode E2E, and real Cap-widget E2E against a local deterministic Cap mock. |
| `make load-smoke` | Bounded concurrent read/create/upload/download load smoke that exercises backpressure and recovery. Local/pre-release, not every-push CI. |
| `make dev-backend` | Run the API and File listeners without the embedded frontend. |
| `make dev-frontend` | Run Vite's development server for the UI. |
| `make clean` | Remove generated binaries, frontend output, embedding staging, and release archives. |

Before committing run `make format`, `make check`, `make test`, and `make build`, then inspect the diff, staged files, and status for secrets or artifacts. Before a real release run `make qualify-release VERSION=...`; longer fuzz campaigns and machine-specific performance measurement remain explicit local/pre-release commands.

## Development and production frontend

Development and production frontend serving are deliberately different concerns:

- **Development:** run `make dev-backend` and `make dev-frontend`. The Go application listener serves `/api/v1/*`, `/raw/*`, and `/healthz`; Vite serves the React UI on its own port and proxies `/api` and `/raw` to the backend. The documented development state does not require generated production assets.
- **Production:** `make build` runs `tsc --noEmit && vite build` in `web/`, replaces the Git-ignored `internal/webapp/dist/` staging directory with the exact Vite output, and compiles `go build -tags production -o upaste ./cmd/upaste`. The resulting executable serves the embedded UI and needs no `web/dist`, Node, or copied assets at runtime.

The production `internal/webapp` implementation is behind the `production` build tag and uses `//go:embed all:dist`. The default `!production` implementation returns no embedded handler, so `go test ./...`, `make check`, and `make test` work from a clean checkout with no generated assets.

`scripts/build-production-binary.sh` is the canonical build implementation used by `make build` and CI. `scripts/prove-embedded-binary.sh` proves self-containment. `scripts/e2e-production-server.sh` starts the embedded binary for Playwright; when `UPASTE_BINARY` is set it uses that prebuilt binary instead of rebuilding.

Generated and ignored paths:

```text
upaste                        # production executable
web/dist/                     # Vite output
internal/webapp/dist/         # embedding staging, rebuilt on every production build
release/                      # deterministic release archives and checksums
web/playwright-report/        # Playwright reports
web/test-results/             # Playwright artifacts
data/                         # default runtime database/objects
```

These are never sources of truth and are safe to delete; `make clean` removes them.

## Build metadata and version command

`internal/buildinfo` is the only build-metadata boundary. Development builds
report `devel`, `unknown`, and `unknown`; release tooling injects deterministic
values with Go linker `-X` flags:

```text
github.com/DejavuMoe/uPaste/internal/buildinfo.Version
github.com/DejavuMoe/uPaste/internal/buildinfo.Commit
github.com/DejavuMoe/uPaste/internal/buildinfo.BuildDate
```

```sh
./upaste --version
./upaste -version
```

Both forms print the same stable output and return before configuration parsing,
SQLite initialization, data-directory creation, listener binding, or
maintenance. Unknown flags and combined invalid flags still fail normal CLI
parsing. There is no HTTP version endpoint.

## Release packaging

`scripts/package-release.sh` is the canonical packaging implementation behind
`make dist`. It validates `VERSION` as `vMAJOR.MINOR.PATCH` with optional
prerelease/build suffixes, resolves the exact commit, derives
`SOURCE_DATE_EPOCH` from that commit unless explicitly provided, builds the
frontend once, stages the embedding directory, then cross-compiles `linux/amd64`
and `linux/arm64` with `CGO_ENABLED=0`, `-tags production`, `-trimpath`, and
deterministic metadata.

Each archive has this shape:

```text
upaste-<version>-linux-<arch>/
  upaste
  README.md
  DEPLOYMENT.md
  BUILDINFO
  upaste.service
  upaste.env.example
  nginx.conf.example
  Caddyfile.example
```

`SHA256SUMS` covers both archives. Archives normalize ordering, ownership,
permissions, timestamps, and gzip headers, so the same inputs produce identical
amd64 bytes. `scripts/verify-release.sh` (and `make verify-dist`) checks the
checksums, archive shape, embedded version/commit strings, AArch64 ELF identity,
amd64 `--version`, embedded frontend and health endpoint, API and File-origin
create/download behavior, clean `SIGTERM` shutdown, restart persistence, and a
deterministic amd64 rebuild. It creates no tag and no GitHub Release.

The release workflow at `.github/workflows/release.yml` is draft-only and
triggers on `v*` tags or a manual dispatch. It validates tag/version/commit
integrity before creating a draft release. Ordinary master CI runs packaging
with a synthetic `v0.0.0-test` version and never publishes.

## Runtime configuration

Configuration precedence is CLI, then environment, then default:

| Purpose | Environment | CLI | Default |
|---|---|---|---|
| Application address | `UPASTE_ADDR` | `-addr` | `127.0.0.1:8080` |
| File listener address | `UPASTE_FILE_ADDR` | `-file-addr` | `127.0.0.1:8081` |
| Public file origin | `UPASTE_FILE_ORIGIN` | `-file-origin` | `http://127.0.0.1:8081` |
| Trusted proxy CIDRs | `UPASTE_TRUSTED_PROXY_CIDRS` | `-trusted-proxy-cidrs` | empty |
| Deployment mode | `UPASTE_DEPLOYMENT_MODE` | — | `private` |
| Public default TTL | `UPASTE_PUBLIC_DEFAULT_TTL` | — | `24h` |
| Public maximum TTL | `UPASTE_PUBLIC_MAX_TTL` | — | `168h` |
| Challenge provider | `UPASTE_CHALLENGE_PROVIDER` | — | empty (private mode) |
| Cap settings | `UPASTE_CAP_ENDPOINT`, `UPASTE_CAP_SITE_KEY`, `UPASTE_CAP_SECRET_KEY` | — | empty |
| Turnstile settings | `UPASTE_TURNSTILE_SITE_KEY`, `UPASTE_TURNSTILE_SECRET_KEY`, `UPASTE_TURNSTILE_HOSTNAME` | — | empty |
| Challenge timeout | `UPASTE_CHALLENGE_TIMEOUT` | — | `10s` |
| Superadmin token | `UPASTE_ADMIN_TOKEN` | — | empty (admin disabled) |
| Admin cookie Secure | `UPASTE_ADMIN_COOKIE_SECURE` | — | public forces true; private defaults false |
| Data directory | `UPASTE_DATA_DIR` | `-data-dir` | `./data` |

The data directory is resolved to an absolute clean path. The application creates `<data-dir>/upaste.db` and `<data-dir>/objects/`; newly created directories/database/object modes are `0700`/`0600`. File and application addresses must differ. File origin must be an absolute HTTP(S) origin without path, query, fragment, or userinfo; it is never inferred from Host or forwarded headers. Existing operator-managed permissions are preserved. Examples:

```sh
make dev-backend
make build && ./upaste
mise exec -- go run ./cmd/upaste -addr 0.0.0.0:8080 -data-dir /srv/upaste
UPASTE_DATA_DIR=/tmp/upaste-dev make dev-backend
```

The loopback default is intentional. Binding externally requires explicit operator configuration and an appropriate trusted reverse proxy/firewall.

## Runtime routes and headers

The application listener serves the embedded frontend at `/`, `/s/:id`, and `/manage/:id`; it also serves `/api/v1/*`, `/raw/*`, and `/healthz`. `/f/*` is intentionally 404 on the application listener. Unknown frontend paths and missing `/assets/*` files return 404 rather than a blanket SPA fallback. `GET` and `HEAD` are supported for frontend resources; other methods receive 405 on known routes/assets and 404 elsewhere.

`index.html` and SPA fallback responses use `Cache-Control: no-store`. Hashed `/assets/*` files use `Cache-Control: public, max-age=31536000, immutable`. Embedded frontend responses set `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`, and a strict CSP documented in [ARCHITECTURE.md](ARCHITECTURE.md) and [SECURITY.md](SECURITY.md).

The File listener remains a separate origin and exposes only `/f/{id}` with attachment-only delivery and its own security headers. Reverse proxies must be configured with distinct origins rather than Host-based multiplexing.

## Transfer deadlines

Servers keep 5-second header reads, 15-second ordinary read/write deadlines, 60-second idle connections, and 32 KiB headers. A valid multipart File create extends that request body's read deadline and eventual response write deadline to 10 minutes before multipart parsing; File GET/HEAD extends only that response write deadline to 10 minutes before delivery. This bounded exception is needed for the 64 MiB transfer limit and does not add rate limiting or make ordinary JSON slow-body requests long-lived.

Trusted proxy CIDRs are comma-separated and empty by default: forwarded headers are ignored unless the immediate TCP peer matches one. Maintenance runs once after listener startup and every 15 minutes; it purges expired rows in bounded batches and reconciles stale Local objects after a 30-minute grace. It has no HTTP/admin endpoint.

## Database and migrations

`internal/database/migrations/*.sql` is embedded into the binary. Migrations are forward-only, monotonically versioned, and use SQLite `PRAGMA user_version`; add a migration rather than editing an already released one. Schema version 2 adds `standard_text_payloads`; version 3 adds `encrypted_text_payloads`; version 4 adds `file_payloads` and File invariant triggers. Migrations 0001–0003 remain immutable. The application refuses a database newer than its supported schema. Database tests use `t.TempDir()` and require no external service.

The pool limit is four open and four idle connections. Write transactions acquire SQLite's immediate lock so concurrent read-then-write owner operations wait under the existing five-second busy timeout instead of failing with a stale deferred snapshot. The concurrency regression runs four updates and four independent creates together. Connection hardening and its tests are described in [ADR 0008](adr/0008-sqlite-driver.md).

## Endpoint

```sh
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/
```

`GET /healthz` returns HTTP 200 and `application/json; charset=utf-8`. It is a lightweight liveness endpoint and does not query SQLite. Standard/Encrypted Text and Standard File routes and safe placeholder examples are documented in [API.md](API.md). Request decoding uses Go JSON v2 strict defaults and explicit unknown-member rejection; responses retain the Phase 2 encoder for compatibility. Never put an owner token in a URL or logs; send it only in an Authorization bearer header.

## Go formatting scope

`scripts/gofmt` discovers Go files across the repository, including `internal/`. It excludes dependency directories (`vendor/` and `node_modules/`), directories named `generated`, and files carrying Go's standard `// Code generated ... DO NOT EDIT.` marker. Both `make check` and CI call its check mode, which lists unformatted files and exits non-zero; `make format` calls its write mode over the same file set.

## CI

CI has five jobs. `backend` runs formatting, `go vet`, all Go tests, and a default-tag build. `frontend` runs a frozen pnpm install, TypeScript checking, Vitest, and the Vite production build. `e2e` runs the existing Vite-backed real-browser suite. `production` installs Chromium, builds and stages the production frontend, runs production-tagged Go vet/tests, builds the single self-contained binary, runs the isolated-binary proof, and runs the embedded production E2E suite plus the public Turnstile-mode and real-Cap-widget suites. `qualify` runs `scripts/qualify-release.sh` with the synthetic `v0.0.0-test` version: full Go suite, race suite, repeated concurrency stress, deployment/systemd checks, deterministic amd64/arm64 package build and verification, backup/restore proof, and shutdown-under-load proof. It never publishes a release. Longer fuzz campaigns (`make fuzz`) and performance baselines (`make benchmark`) are intentionally manual/pre-release rather than every-push CI. Action references are immutable SHAs annotated with their upstream major tag in the workflow.
