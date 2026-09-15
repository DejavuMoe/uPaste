# Architecture

## Current implementation (through Phase 11)

uPaste is a small modular monolith. `cmd/upaste` parses configuration, prepares and migrates SQLite, opens local object storage, then starts two Go `net/http` processes using `http.ServeMux`, `slog`, bounded HTTP timeouts, a 32 KiB header limit, graceful SIGINT/SIGTERM shutdown, and loopback by default. `GET /healthz` remains a database-independent liveness check.

The application listener serves the embedded production React frontend, `/api/v1/*`, `/raw/*`, and `/healthz`. Public mode adds `GET /api/v1/config` for non-secret runtime policy, requires challenge verification only for `POST /api/v1/shares`, and exposes `/api/v1/admin/*` plus the `/admin` SPA route only when a Superadmin token is configured. Private mode remains the default and keeps anonymous creation without a challenge and optional/never expiration. The independent File listener serves only `/f/{id}` as an attachment. File bytes are never served from the application origin and `/f/*` returns 404 there.

The implemented internal packages are deliberately limited:

- `domain`: closed V1 classifications, privacy compatibility, and pure expiration semantics.
- `challenge`: Cap and Turnstile server-side challenge verification with fail-closed provider errors.
- `admin`: one Superadmin token verifier, bounded login limiter, and in-memory CSRF-bound sessions.
- `adminapi`: bounded governance API for summary, listing/inspection, delete, bulk delete, and expired cleanup.
- `capability`: canonical random Share IDs and owner capabilities plus SHA-256 verification.
- `config`: CLI/environment/default resolution for addresses, File origin, trusted proxies, and data directory.
- `database`: filesystem preparation, hardened SQLite connections, and embedded forward migrations.
- `share`: concrete transactional Standard/Encrypted Text and Standard File behavior using an injected clock.
- `httpapi`: strict Go JSON v2 and multipart creation, bearer authorization, and application API responses.
- `fileapi`: isolated attachment-only File listener with a bounded response deadline.
- `webapp`: embedded production frontend access, SPA route resolution, static asset delivery, cache policy, and frontend security headers.
- `objectstore`: `Store` staging/commit/open/delete boundary; `Local` is the current implementation.
- `maintenance`: deterministic purge/reconciliation pass plus one periodic worker.
- `abuse`: trusted-proxy client identity, bounded process-local limiters, and File gates.

There is no ordinary user/account/role system, encrypted File behavior, distributed limiter, or storage quota. The production React UI is implemented and embedded; development still serves the UI from the Vite development server. Public mode uses a challenge-gated anonymous creation path and creation-anchored finite retention; Superadmin governance supports inspect-and-remove without content editing or decryption.

The browser module `web/src/crypto/encryptedText.ts` owns key generation, AES-256-GCM encryption/decryption, the binary plaintext envelope, and strict key-fragment/base64url handling. The HTTP server never decrypts and has no production AES key handling. API responses model Standard, Encrypted, and File payloads explicitly so irrelevant zero-valued fields are never serialized.

Ordinary application requests retain 15-second read/write deadlines. Valid bounded File multipart uploads extend their body read and eventual response write deadlines to 10 minutes; File GET/HEAD extend only their response write deadline to 10 minutes. Header reads remain 5 seconds, idle connections 60 seconds, and headers 32 KiB.

The app listener defaults to `127.0.0.1:8080`; the independent File listener defaults to `127.0.0.1:8081`; their equal-address configuration is rejected. Configured `file-origin`, default `http://127.0.0.1:8081`, is the only public File URL base. File delivery never branches on Host or forwarded headers.

## Production frontend delivery

The Vite production bundle is compiled into the Go executable. `scripts/build-production-binary.sh` runs the TypeScript check and Vite build, replaces the Git-ignored staging directory `internal/webapp/dist/`, and compiles with `-tags production`. The production-tagged `internal/webapp` file owns `//go:embed all:dist`; the default `!production` build returns no embedded handler so a clean checkout can still run `go test ./...`, `make check`, and `make test` without generated assets. `make build` is the canonical single-binary build.

`cmd/upaste` composes request dispatch with explicit precedence:

1. `/healthz` reaches the liveness mux.
2. `/api/*`, `/raw`, and `/raw/*` reach the API handler.
3. The embedded `webapp` handler is consulted for everything else.
4. With no embedded frontend (development build), all non-API paths return 404 and Vite serves the UI.

The `webapp` handler serves `index.html` for exactly `/`, `/index.html`, `/s/:id`, and `/manage/:id`. Unknown routes, `/admin`, `/login`, `/dashboard`, missing `/assets/*` files, and `/f/*` remain 404. Request methods other than GET/HEAD receive 405 on known frontend routes/assets and 404 elsewhere.

`index.html` and SPA fallback responses are `Cache-Control: no-store`. Hashed Vite assets under `/assets/` receive `Cache-Control: public, max-age=31536000, immutable` with extension-appropriate MIME types. HTML, JavaScript, CSS, JSON, images, fonts, and text are served explicitly; unknown extensions fall back to the platform MIME database and then `application/octet-stream`.

Every embedded frontend response carries:

```http
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
X-Frame-Options: DENY
Content-Security-Policy: default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'
```

The private baseline policy uses no `unsafe-eval`, `unsafe-inline`, wildcard sources, external script hosts, or external font hosts. Public challenge modes extend it only with the exact configured Cap origin (plus `wasm-unsafe-eval`, a worker `blob:` allowance, and per-response nonces) or the canonical Cloudflare Turnstile origin. File downloads remain direct navigations to the separately configured File origin, which has its own attachment/CSP/no-store/no-referrer/nosniff/frame-denial policy.

## Build metadata and release packaging

`internal/buildinfo` is the single build-metadata boundary. Development builds report `devel` / `unknown` / `unknown`; release builds inject `Version`, `Commit`, and a deterministic `BuildDate` with linker `-X` values. `upaste --version` prints that metadata and exits before configuration parsing, SQLite initialization, data-directory creation, listener binding, or maintenance.

`scripts/package-release.sh` builds the Vite bundle once, cleans and stages `internal/webapp/dist`, then cross-compiles `linux/amd64` and `linux/arm64` with `CGO_ENABLED=0 -tags production -trimpath`. It emits deterministic `tar.gz` archives plus `SHA256SUMS` under the Git-ignored `release/` directory. `scripts/verify-release.sh` checks checksums, archive shape and contents, embedded version/commit strings, AArch64 ELF identity, amd64 standalone runtime, restart persistence, and a deterministic rebuild.

The supported native deployment is one unprivileged `upaste` systemd service with loopback-only listeners, persistent state under `/var/lib/upaste`, and TLS plus two distinct public origins at a trusted reverse proxy. `docs/DEPLOYMENT.md` is the authoritative operator guide; `deploy/` contains the systemd, environment, Nginx, and Caddy examples.

## Persistence foundation

The database is `<data-dir>/upaste.db`; local File objects are `<data-dir>/objects/<first-two-hex>/<remaining-hex-key>`. Local storage opens this root once with Go `os.Root`; stage, finalization, reads, deletion, and reconciliation stay confined to that handle even if the pathname is later replaced. New object directories/files use `0700`/`0600`; server-generated 16-byte hex object keys and filenames never become public filesystem paths. New data directories and database files are created with `0700` and `0600` permissions respectively; existing operator-managed permissions are not weakened. SQLite uses `modernc.org/sqlite` v1.58.0 (SQLite 3.53.4, `modernc.org/libc` v1.75.6) through `database/sql`, with no CGO or ORM.

Validated DSN parameters configure every physical connection:

```text
_busy_timeout=5000
_defensive=1
_dqs=0
_foreign_keys=ON
_journal_mode=WAL
_synchronous=NORMAL
_txlock=immediate
```

The pool is conservatively bounded to four open and four idle connections without an arbitrary connection lifetime. Immediate write transactions were enabled after a four-owner-update/four-independent-create stress test repeatedly reproduced deferred-transaction `SQLITE_BUSY` and `SQLITE_BUSY_SNAPSHOT`; the same test passes repeatedly with immediate acquisition and the existing five-second busy timeout. The pool, WAL mode, and timeout remain unchanged. Shared cache, OFD locking, loadable extensions, and other speculative SQLite tuning are not enabled.

Embedded, forward-only SQL migrations use `PRAGMA user_version`. Version 1 creates Share metadata and a partial expiration index; immutable migration 2 adds `standard_text_payloads`; migration 3 adds `encrypted_text_payloads`; migration 4 adds `file_payloads` and File payload/privacy triggers; migration 5 adds admin listing indexes on `created_at` and `(payload_kind, privacy_mode, created_at)`. Missing migrations run transactionally in ascending order, successful version advancement is committed with the migration, and databases newer than the binary are refused.

## Frozen V1 boundaries

A V1 Share owns exactly one `TEXT` or `FILE` payload. Text may be `STANDARD` or browser-encrypted; files are `STANDARD` only. Expiration is derived on every read from nullable `expires_at` and authoritative server time. Deletion removes active data rather than retaining a user-visible soft-delete row; no lifecycle-state column exists.

The metadata schema contains `id`, `payload_kind`, `privacy_mode`, `owner_token_verifier`, `created_at`, `updated_at`, and nullable `expires_at`. Timestamps are integer Unix milliseconds with UTC semantics. Payload columns are intentionally absent.

## Maintenance and abuse controls

Maintenance runs once asynchronously after both listeners start and then every 15 minutes. It purges at most 2048 expired rows in 256-row transactions, then snapshots all remaining File references and scans Local objects once. Objects/stages require a 30-minute grace, longer than the 10-minute File deadline; unknown filesystem entries are reported, not removed.

Rate limits, trusted-proxy resolution, and File gates are process-local in-memory state. They complement trusted reverse-proxy and network controls; they are not distributed DDoS protection.

## Frontend product freeze (historical)

Phase 6 froze the frontend product architecture, routes (`/`, `/s/:id`, `/manage/:id`), secret non-persistence, and visual language ([ADR 0013](adr/0013-frontend-product-architecture.md) and [FRONTEND_PRODUCT_SPEC](FRONTEND_PRODUCT_SPEC.md)). Phase 7 implemented that contract and qualified it with unit, component, real-browser, and visual tests.

## Approved future architecture

The deployable shape remains one Go binary, one SQLite database, one local object store, and two listeners. A future S3-compatible `Store` implementation may replace Local storage when that boundary is needed. Reverse-proxy routing must provide separate origins and must not rely only on `Host`; TLS normally terminates at a trusted reverse proxy. Native systemd deployment and deterministic Linux release packaging are implemented; container packaging and actual public release publication remain future work.

## Not implemented

Encrypted File APIs/persistence, uploads beyond one-shot 64 MiB Standard Files, hard storage quotas, distributed abuse controls, Docker/container packaging, and automatic updates remain later work. Native systemd deployment and deterministic amd64/arm64 release packaging are implemented; no public version tag or GitHub Release exists yet. Kubernetes, microservices, queues, Redis, GraphQL, gRPC, CQRS, and event sourcing are not part of the architecture.
