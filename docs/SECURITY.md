# Security invariants

These are mandatory design constraints. Later implementation must fail closed when a security-sensitive decision is unknown.

1. Resource IDs are identifiers, not secrets.
2. Read access and management access are separate concepts.
3. Management credentials never appear in query parameters, pathnames, logs, analytics, or Referer-visible URLs.
4. Management operations use `Authorization: Bearer <owner-token>`.
5. Raw owner tokens are never persisted server-side. V1 persists only `SHA-256(canonical_owner_token)` and compares valid-form candidates in constant time.
6. Standard content may be visible to the server.
7. Encrypted Text Share content is zero-knowledge with respect to the application server. V1 File Shares are Standard only; encrypted files are not implied by this design.
8. Encrypted text is encrypted in the browser. The server stores ciphertext and never receives plaintext. The decryption key lives in the URL fragment and is never intentionally transmitted to the server.
9. Markdown raw HTML is disabled.
10. Any future Markdown HTML output also passes through sanitization as defense in depth.
11. `/raw/{id}` is always inert plain text; Files are served only from the separate file origin as attachments. with `Content-Type: text/plain; charset=utf-8`, `X-Content-Type-Options: nosniff`, and a restrictive Content Security Policy.
12. Uploaded untrusted files are served from a separate web origin and listener from the application/API. Host-header checks alone are insufficient.
13. Active uploaded formats such as HTML, SVG, XML, and JavaScript never execute with application-origin authority.
14. Every File delivery is an attachment regardless of detected MIME type.
15. Filename extensions and browser-declared MIME types are not authoritative.
16. Logs never contain request bodies, Authorization values, passwords, owner tokens, encryption keys, or plaintext encrypted-share content.
17. No ordinary user authentication or account cookies exist. The only authentication cookie is the Superadmin session cookie described below; anonymous users remain capability-based.
18. The API is same-origin by default; permissive CORS is disabled.
19. Security-sensitive defaults fail closed.

## Identifier and capability primitives

- A public Share ID is generated from 16 independent `crypto/rand` bytes (128 bits) and encoded with unpadded base64url, producing 22 URL-safe characters. It is opaque and never sequential or derived from timestamps/hosts. It grants no authority.
- An owner capability is generated independently from 32 `crypto/rand` bytes (256 bits). Its canonical shape is `up_o1_<base64url-32-random-bytes>` with no base64 padding. The visible `up_o1_` purpose/version prefix is not secret; the random portion is the security boundary.
- V1 stores `SHA-256(canonical_owner_token)`, not the token. A server-side pepper/HMAC key is intentionally omitted: exhaustive attack against a uniform 256-bit token is infeasible, while another critical secret would create key-loss, rotation, and recovery failure modes. A keyed verifier can be introduced if evidence changes this tradeoff.
- Verification uses constant-time digest comparison after canonical candidate parsing. Share ID and owner token generation are independent.

Phase 2 uses these primitives for owner-authorized PATCH/DELETE. Only the creation response explicitly calls `OwnerToken.Reveal`; ordinary string formatting and structured `slog` values redact the type as `[REDACTED owner capability]`. Management accepts exactly one `Authorization: Bearer` header and never URL/body/cookie credentials. Persistence receives only the verifier. Valid-form candidates use constant-time digest comparison, and handlers/logs never echo candidates.

PATCH verifies the active Share and capability before body processing, then repeats existence, expiration, and capability checks inside the update transaction. This prevents unauthorized body-validation feedback without treating the preliminary check as mutation authorization.

Standard File upload is bounded to 64 MiB through streamed staging; the full multipart wire body is capped at 66 MiB. Valid multipart upload body reads and eventual responses, plus File GET/HEAD writes, use bounded 10-minute per-operation deadlines; ordinary API requests retain the server's 15-second deadline. Original filenames are sanitized metadata, never paths. The server sniffs at most 512 bytes with `http.DetectContentType`, computes SHA-256 while streaming, and every delivery is `Content-Disposition: attachment` regardless of MIME. No malware scanning is performed.

File URLs use explicitly configured `file-origin`; Host and forwarded headers are never trusted. The file listener exposes only `/f/{id}` and all responses carry no-store, nosniff, no-referrer, frame denial, and sandbox CSP. The application listener never serves File bytes.

SQLite connections enable foreign keys, WAL, immediate write transactions, a five-second busy timeout, NORMAL synchronization, defensive mode, and disabled double-quoted-string fallback on every physical connection. Immediate transactions avoid reproducible deferred-transaction `SQLITE_BUSY`/`SQLITE_BUSY_SNAPSHOT` failures in concurrent owner updates while retaining the four-connection pool.

Text request JSON has a 2 MiB wire limit. Standard decoded content has a matching application/SQL range of 1–1,048,576 UTF-8 bytes; Encrypted payloads have a 12-byte nonce and 19–1,048,594-byte ciphertext/tag BLOB. Request decoding uses strict Go JSON v2 defaults plus unknown-member rejection: duplicate names, invalid UTF-8, unknown or incorrectly cased fields, malformed JSON, and trailing values fail closed. Public API responses are `no-store` and `nosniff`. `/raw/{id}` always emits exact inert `text/plain` bytes with `nosniff`, `no-store`, no-referrer, frame denial, and a restrictive sandboxed CSP; its errors receive the same headers. JSON uses normal escaping and Markdown is not rendered.

Encrypted Text protocol `UPASTE_AES_GCM_V1` is frozen by ADR 0010: browser Web Crypto AES-256-GCM, directly generated 32-byte key, fresh random 12-byte nonce per encryption, 128-bit tag, exact AAD `uPaste:encrypted-text:v1`, and a version/format/UTF-8 binary envelope. The key exists only in canonical `#up_e1_` URL-fragment form and is never intentionally sent to or stored by the server. The server stores and validates only protocol, nonce, and combined ciphertext/tag; it cannot validate plaintext format or meaning.

The fragment is a confidentiality capability, not management authority. The independent owner token authorizes PATCH/DELETE but cannot decrypt. Fragment exposure through complete-link sharing, browser history/sync, clipboard, screenshots, extensions, or page JavaScript compromises confidentiality. Phase 3 and Phase 6 frontend specifications ([ADR 0013](adr/0013-frontend-product-architecture.md), [FRONTEND_PRODUCT_SPEC](FRONTEND_PRODUCT_SPEC.md)) enforce zero persistence or transport of OwnerTokens in history.state, React Router location.state, localStorage, sessionStorage, IndexedDB, or cookies. Markdown rendering forbids raw HTML, executable URLs, active elements, and automatic external media.

Rate limits derive identity from the TCP peer unless that immediate peer matches explicitly configured trusted-proxy CIDRs; only then is X-Forwarded-For walked right-to-left. IPv4-mapped addresses are unmapped and IPv6 limiter keys aggregate to /64. Limiters are bounded in-memory process state, reset on restart, and complement rather than replace proxy/network controls.

Local objects use server-generated keys and restrictive `0700` object directories/`0600` files. Filesystem finalization and SQLite commit are compensated on ordinary failure, including a finalization error after publication, so committed Shares do not intentionally point at missing objects. A failed compensating delete is reported internally but remains a future reconciliation orphan. A crash after finalization before DB commit or a post-delete cleanup failure can also leave an unreachable orphan; Phase 5 removes only stale unreferenced regular objects/stages after a 30-minute grace. Unknown/symlink filesystem entries and referenced-but-missing objects are reported rather than automatically deleted. Missing objects for live metadata return 500.

Zero knowledge covers an honest server running the reviewed protocol: it does not receive plaintext or key. It does not protect against malicious modified JavaScript from the host, a compromised browser or extension, stolen fragments, traffic metadata, IP/timing/ciphertext-size observation, server-controlled expiration, or owner-capability misuse. Encrypted files require a separate design covering streaming, bounded memory, authenticated chunking, resumability, integrity, and key handling.

## Embedded production frontend boundary

The production React bundle is built by Vite and embedded in the Go application binary. `internal/webapp/dist/` is generated staging ignored by Git; the production build replaces it from scratch, so it is never source of truth and a clean checkout never embeds a stale bundle. The application listener serves only the frozen SPA entry shapes `/`, `/s/:id`, and `/manage/:id`, plus embedded static files. Unknown routes, missing `/assets/*` files, and `/f/*` return 404 rather than a blanket SPA fallback. Only GET and HEAD are accepted for frontend resources; other methods receive 405 on known routes/assets and 404 elsewhere.

`index.html` and SPA responses use `Cache-Control: no-store`; hashed Vite assets under `/assets/` use `Cache-Control: public, max-age=31536000, immutable`. Every embedded frontend response sets `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`, and this CSP:

```text
default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'
```

The policy has no `unsafe-eval`, no wildcard source, and no external script or font origin. `style-src 'self'` is deliberately kept strict; the dynamic upload progress indicator is qualified under a throttled multi-megabyte upload and must not be replaced with `unsafe-inline`. Embedded asset content types come from the generated extensions, never from request input or user-controlled filenames. Markdown external links remain ordinary navigations and do not gain script, frame, or form privileges. The separate File origin and attachment-only File semantics are unchanged.

## Native deployment and release integrity

Release binaries for `linux/amd64` and `linux/arm64` are built from the exact source revision with `CGO_ENABLED=0 -tags production -trimpath` and deterministic version/commit/build-date metadata injected by the linker. Release archives and `SHA256SUMS` are generated from normalized inputs; operators must verify the checksum before installing. There is no automatic updater: the application never fetches or executes updates.

The supported systemd unit runs as an unprivileged `upaste` user, loads `/etc/upaste/upaste.env`, writes only `/var/lib/upaste`, uses `UMask=0077`, and applies hardening including `NoNewPrivileges=true`, `ProtectSystem=strict`, `ProtectHome=true`, empty capability sets, and restricted address families. Both listeners remain loopback-only; TLS terminates at a trusted reverse proxy with two distinct public origins. `UPASTE_TRUSTED_PROXY_CIDRS` must contain only the directly connected proxy, never `0.0.0.0/0` or `::/0`.

Backups must capture the complete data tree (`upaste.db`, any SQLite side files, and `objects/`) in a cold, consistent state. Database migrations are forward-only, so binary-only rollback is safe only when no schema advancement has occurred; otherwise restore the pre-upgrade backup. No license has been selected or implied by the release tooling.

## Pre-release qualification gate

Before any real version tag, `make qualify-release VERSION=v0.0.0-test` must pass. It runs the full Go suite under the race detector, repeated stress on `share`, `database`, `maintenance`, `abuse`, `httpapi`, `fileapi`, and `objectstore`, deployment and systemd checks, deterministic amd64/arm64 package builds, packaged private- and public-mode runtime verification (shipped-file equality with the claimed commit, challenge verification, creation-anchored retention, Superadmin CSRF-governed deletion, restart persistence, and deterministic rebuilds), an automated cold backup/restore proof, and a shutdown-under-load restart proof. Ordinary CI runs the same qualification with a synthetic non-release version and never publishes. Packaging refuses a working tree with uncommitted tracked changes so an artifact cannot claim a commit it was not built from.

Additional explicit local/pre-release checks:

```sh
make fuzz        # bounded parser fuzz campaigns (capability, XFF, base64url, filename, CIDR)
make benchmark   # informational local performance baseline, not a CI threshold
make load-smoke  # bounded concurrent load exercising rate limits, gates, and recovery
```

The security suite also locks capability/Share-ID/path attack matrices, JSON and multipart adversarial behavior, interrupted-upload cleanup, raw/File-origin attachment semantics, browser encrypted-text tamper rejection, Markdown sanitization, trusted-proxy spoofing resistance, rate-limit and concurrency-gate release paths, SQLite integrity, migration compatibility, and object-store confinement.

Release archives include a deterministic `BUILDINFO` file (version, commit, build date, Go version, target) in addition to embedded `upaste --version` metadata and `SHA256SUMS`. No license has been selected or implied. No real version tag or GitHub Release is created by the qualification gate.

## Public abuse controls and Superadmin governance

Public mode requires exactly one server-side challenge verifier (Cap or Turnstile) and a high-entropy Superadmin token; startup fails closed if either is missing. Challenge tokens travel only in the transient `X-uPaste-Challenge` request header for `POST /api/v1/shares`, never in JSON, multipart metadata, URLs, cookies, history, or browser storage. Provider verification is server-side, bounded by a timeout, and fails closed on provider errors; the existing IP/global rate limiters run before challenge verification. Public Shares always expire and expiration is anchored to `created_at + max_ttl`, so repeated OwnerToken PATCH requests cannot create permanent anonymous storage.

Superadmin is one independently generated `up_a1_<256-bit unpadded base64url>` capability. Only its SHA-256 verifier is retained at startup. Admin sessions are in-memory, restart-invalidated, at most 8 hours, use a host-only HttpOnly `SameSite=Strict` cookie (Secure in public mode), and require a session-bound `X-uPaste-CSRF` header for logout and every destructive action. Admin APIs send no CORS headers. Admin listing is metadata-only: the paginated query computes payload byte counts and lightweight File metadata without selecting Text/BLOB content, ciphertext, nonce, storage keys, hashes, or verifiers; payload content is fetched on demand through the detail endpoint. Admin can inspect server-visible detail, delete, bulk-delete, and run the established expired-cleanup pass. Admin cannot edit or replace content, recover OwnerTokens, or decrypt encrypted Shares; encrypted content remains ciphertext-only and the UI exposes only protocol, nonce, and ciphertext size.

CSP stays provider-aware but fail-closed: private mode keeps the strict baseline; Cap adds its configured external origin only to `connect-src`, plus `'wasm-unsafe-eval'`, a worker `blob:` allowance, and per-response nonces; Turnstile adds only the canonical Cloudflare challenge origin to its exact script/frame/connect directives. No mode uses `unsafe-inline`, `unsafe-eval`, or wildcard sources.

## Repository secret policy

This public repository must never contain real credentials, tokens, private keys, databases, uploaded content, `.env` files, secrets, or private IDE state. Examples, if introduced, use conspicuously fake values. Before every commit, inspect staged changes and scan for accidental secrets. Runtime secrets come from deployment configuration outside Git; they must not be printed.
