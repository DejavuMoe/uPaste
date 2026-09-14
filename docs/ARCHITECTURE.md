# Architecture

## Current Phase 1 implementation

uPaste is a small modular monolith. `cmd/upaste` parses configuration, prepares and migrates SQLite, then starts a Go `net/http` process using `http.ServeMux`, `slog`, bounded HTTP timeouts, graceful SIGINT/SIGTERM shutdown, and loopback by default. Its only endpoint remains `GET /healthz`; this lightweight liveness check does not query the database. `web/` remains an independently built React/TypeScript/Vite shell.

The implemented internal packages are deliberately limited:

- `domain`: closed V1 classifications, privacy compatibility, and pure expiration semantics.
- `capability`: canonical random Share IDs and owner capabilities plus SHA-256 verification.
- `config`: CLI/environment/default resolution for address and data directory.
- `database`: filesystem preparation, hardened SQLite connections, and embedded forward migrations.

There is no Share CRUD, payload storage, authorization middleware, encryption implementation, upload handling, or embedded frontend yet.

## Persistence foundation

The database is `<data-dir>/upaste.db`. New data directories and database files are created with `0700` and `0600` permissions respectively; existing operator-managed permissions are not weakened. SQLite uses `modernc.org/sqlite` v1.58.0 (SQLite 3.53.4, `modernc.org/libc` v1.75.6) through `database/sql`, with no CGO or ORM.

Validated DSN parameters configure every physical connection:

```text
_busy_timeout=5000
_defensive=1
_dqs=0
_foreign_keys=ON
_journal_mode=WAL
_synchronous=NORMAL
```

The pool is conservatively bounded to four open and four idle connections without an arbitrary connection lifetime. This is an initial single-server value, not a scalability claim. Shared cache, OFD locking, loadable extensions, and speculative SQLite tuning are not enabled.

Embedded, forward-only SQL migrations use `PRAGMA user_version`. Version 1 creates only Share metadata and a partial expiration index. Missing migrations run transactionally in ascending order, successful version advancement is committed with the migration, and databases newer than the binary are refused.

## Frozen V1 boundaries

A V1 Share owns exactly one `TEXT` or `FILE` payload. Text may be `STANDARD` or browser-encrypted; files are `STANDARD` only. Expiration is derived on every read from nullable `expires_at` and authoritative server time. Deletion removes active data rather than retaining a user-visible soft-delete row; no lifecycle-state column exists.

The metadata schema contains `id`, `payload_kind`, `privacy_mode`, `owner_token_verifier`, `created_at`, `updated_at`, and nullable `expires_at`. Timestamps are integer Unix milliseconds with UTC semantics. Payload columns are intentionally absent.

## Approved future architecture

The deployable shape remains one Go binary, one SQLite database, and one data directory. Payload objects will initially use the local filesystem; an interface is deferred until the expected S3-compatible implementation creates a real second boundary. The production frontend bundle will eventually be embedded in the executable.

The same binary may later expose separate loopback listeners for the application/API and untrusted file origin, commonly `127.0.0.1:8080` and `127.0.0.1:8081`. Reverse-proxy routing must provide separate origins and must not rely only on `Host`. TLS normally terminates at a trusted reverse proxy.

## Not implemented in Phase 1

Share APIs, payload persistence, authorization middleware, uploads, encryption code, rate limiting, file listener, Markdown rendering, syntax highlighting, Docker packaging, and production UI remain later work. Kubernetes, microservices, queues, Redis, GraphQL, gRPC, CQRS, and event sourcing are not part of the architecture.
