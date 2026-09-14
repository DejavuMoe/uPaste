# Architecture

## Current Phase 0 implementation

uPaste is a small modular monolith. `cmd/upaste` is a Go `net/http` process using `http.ServeMux`, `slog`, bounded HTTP timeouts, graceful SIGINT/SIGTERM shutdown, and loopback address `127.0.0.1:8080` by default. Its only endpoint is `GET /healthz`. `web/` is an independently built React/TypeScript/Vite shell. There is no persistence, object storage, authorization, encryption, upload handling, or embedded frontend yet.

## Approved future architecture

The deployable shape is one Go binary, one SQLite database, and one data directory. Explicit SQL migrations and `database/sql` will be used without an ORM; SQLite will use WAL mode. Objects initially live on the local filesystem behind a meaningful storage interface because an S3-compatible implementation is expected later. The production frontend bundle will be embedded in the Go executable.

The same binary may internally expose two listeners:

- application/API: `127.0.0.1:8080`
- untrusted files: `127.0.0.1:8081`

A reverse proxy maps those listeners to separate origins such as `paste.example.com` and `files.example.com`. Security separation depends on listeners/origins, not only on `Host`. TLS normally terminates at Caddy, Nginx, or another trusted reverse proxy. Loopback remains the default bind address. An official Docker image is planned.

The API is same-origin by default. Anonymous management uses bearer capabilities. Encrypted text is encrypted/decrypted in the browser; only ciphertext reaches the server and the key remains in the URL fragment.

## Not designed or implemented in Phase 0

API schemas, database schema/migrations, object layout, retention jobs, crypto wire formats, token parameters, rate limits, upload limits, proxy-header configuration, Docker packaging, and production UI remain for later auditable phases. Kubernetes, microservices, queues, Redis, GraphQL, gRPC, CQRS, and event sourcing are not part of the architecture.
