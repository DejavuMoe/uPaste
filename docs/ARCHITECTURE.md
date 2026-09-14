# Architecture

## Current Phase 4.1 implementation

uPaste is a small modular monolith. `cmd/upaste` parses configuration, prepares and migrates SQLite, then starts a Go `net/http` process using `http.ServeMux`, `slog`, bounded HTTP timeouts, a 32 KiB header limit, graceful SIGINT/SIGTERM shutdown, and loopback by default. `GET /healthz` remains a database-independent liveness check. Phase 4 adds Standard File multipart create/read/owner-expiration-update/delete behavior. Files are delivered only from a second listener; Standard raw remains available while Encrypted raw returns 409. `web/` remains an independently built shell without product UI, plus a React-independent Web Crypto protocol module.

The implemented internal packages are deliberately limited:

- `domain`: closed V1 classifications, privacy compatibility, and pure expiration semantics.
- `capability`: canonical random Share IDs and owner capabilities plus SHA-256 verification.
- `config`: CLI/environment/default resolution for address and data directory.
- `database`: filesystem preparation, hardened SQLite connections, and embedded forward migrations.
- `share`: concrete transactional Standard/Encrypted Text and Standard File behavior using an injected clock.
- `httpapi`: strict Go JSON v2 and multipart creation, bearer authorization, and application API responses.
- `fileapi`: isolated attachment-only File listener with a bounded response deadline.
- `objectstore`: `Store` staging/commit/open/delete boundary; `Local` is the current implementation.

There is no generic repository layer, cleanup/reconciliation worker, rate limiting, encrypted File behavior, encrypted product UI, or embedded frontend yet.

The browser module `web/src/crypto/encryptedText.ts` owns key generation, AES-256-GCM encryption/decryption, the binary plaintext envelope, and strict key-fragment/base64url handling. The HTTP server never decrypts and has no production AES key handling. API responses model Standard, Encrypted, and File payloads explicitly so irrelevant zero-valued fields are never serialized.

Ordinary application requests retain 15-second read/write deadlines. Valid bounded File multipart uploads extend their body read and eventual response write deadlines to 10 minutes; File GET/HEAD extend only their response write deadline to 10 minutes. Header reads remain 5 seconds, idle connections 60 seconds, and headers 32 KiB.

The app listener defaults to `127.0.0.1:8080`; the independent file listener defaults to `127.0.0.1:8081`; their equal-address configuration is rejected. Configured `file-origin`, default `http://127.0.0.1:8081`, is the only public File URL base. File delivery never branches on Host or forwarded headers.

## Persistence foundation

The database is `<data-dir>/upaste.db`; local File objects are `<data-dir>/objects/<first-two-hex>/<remaining-hex-key>`. New object directories/files use `0700`/`0600`; server-generated 16-byte hex object keys and filenames never become public filesystem paths. New data directories and database files are created with `0700` and `0600` permissions respectively; existing operator-managed permissions are not weakened. SQLite uses `modernc.org/sqlite` v1.58.0 (SQLite 3.53.4, `modernc.org/libc` v1.75.6) through `database/sql`, with no CGO or ORM.

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

Embedded, forward-only SQL migrations use `PRAGMA user_version`. Version 1 creates Share metadata and a partial expiration index; immutable migration 2 adds `standard_text_payloads`; migration 3 adds `encrypted_text_payloads`; migration 4 adds `file_payloads` and File payload/privacy triggers. Missing migrations run transactionally in ascending order, successful version advancement is committed with the migration, and databases newer than the binary are refused.

## Frozen V1 boundaries

A V1 Share owns exactly one `TEXT` or `FILE` payload. Text may be `STANDARD` or browser-encrypted; files are `STANDARD` only. Expiration is derived on every read from nullable `expires_at` and authoritative server time. Deletion removes active data rather than retaining a user-visible soft-delete row; no lifecycle-state column exists.

The metadata schema contains `id`, `payload_kind`, `privacy_mode`, `owner_token_verifier`, `created_at`, `updated_at`, and nullable `expires_at`. Timestamps are integer Unix milliseconds with UTC semantics. Payload columns are intentionally absent.

## Approved future architecture

The deployable shape remains one Go binary, one SQLite database, one local object store, and two listeners. A future S3-compatible `Store` implementation may replace Local storage when that boundary is needed. The production frontend bundle may later be embedded in the executable.

The same binary already exposes separate loopback listeners for the application/API and untrusted file origin, commonly `127.0.0.1:8080` and `127.0.0.1:8081`. Reverse-proxy routing must provide separate origins and must not rely only on `Host`. TLS normally terminates at a trusted reverse proxy.

## Not implemented in Phase 4

Encrypted File APIs/persistence, uploads beyond one-shot 64 MiB Standard Files, rate limiting, expiration/object reconciliation, Markdown rendering, syntax highlighting, Docker packaging, and production UI remain later work. Kubernetes, microservices, queues, Redis, GraphQL, gRPC, CQRS, and event sourcing are not part of the architecture.
