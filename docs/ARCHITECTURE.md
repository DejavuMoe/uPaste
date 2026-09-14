# Architecture

## Current Phase 0.1 implementation

uPaste is a small modular monolith. `cmd/upaste` is a Go `net/http` process using `http.ServeMux`, `slog`, bounded HTTP timeouts, graceful SIGINT/SIGTERM shutdown, and loopback address `127.0.0.1:8080` by default. Its only endpoint is `GET /healthz`. `web/` is an independently built React/TypeScript/Vite shell. There is no persistence, object storage, authorization, encryption, upload handling, or embedded frontend yet.

## Approved future architecture

The deployable shape is one Go binary, one SQLite database, and one data directory. SQLite will use `modernc.org/sqlite` through `database/sql`, WAL mode, and explicit SQL migrations without an ORM. Objects initially live on the local filesystem behind an interface when the expected S3-compatible second implementation is introduced. The production frontend bundle will be embedded in the Go executable.

A V1 Share owns exactly one `TEXT` or `FILE` payload. Text may be `STANDARD` or browser-encrypted; files are `STANDARD` only. Lifecycle names are conceptual: expiration is derived on every read from nullable `expires_at` and authoritative server time, while deletion removes active data rather than retaining a user-visible soft-delete row.

Initial Share metadata will represent `id`, `payload_kind`, `privacy_mode`, `owner_token_verifier`, `created_at`, `updated_at`, and nullable `expires_at`. Timestamps use integer Unix milliseconds with UTC semantics. No lifecycle state or payload columns are frozen for Phase 1.

The same binary may internally expose two listeners:

- application/API: `127.0.0.1:8080`
- untrusted files: `127.0.0.1:8081`

A reverse proxy maps those listeners to separate origins such as `paste.example.com` and `files.example.com`. Security separation depends on listeners/origins, not only on `Host`. TLS normally terminates at Caddy, Nginx, or another trusted reverse proxy. Loopback remains the default bind address. An official Docker image is planned.

The API is same-origin by default. Anonymous management uses bearer capabilities. Encrypted text is encrypted/decrypted in the browser; only ciphertext reaches the server and the key remains in the URL fragment.

## Not designed or implemented in Phase 0.1

Database schema/migrations, payload storage, Share APIs, authorization middleware, object storage, uploads, encryption code, rate limiting, file listener, Markdown rendering, syntax highlighting, Docker packaging, and production UI remain for later auditable phases. Kubernetes, microservices, queues, Redis, GraphQL, gRPC, CQRS, and event sourcing are not part of the architecture.
