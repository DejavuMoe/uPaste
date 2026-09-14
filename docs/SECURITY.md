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
11. `/raw/{id}` is always inert plain text with `Content-Type: text/plain; charset=utf-8`, `X-Content-Type-Options: nosniff`, and a restrictive Content Security Policy.
12. Uploaded untrusted files are served from a separate web origin and listener from the application/API. Host-header checks alone are insufficient.
13. Active uploaded formats such as HTML, SVG, XML, and JavaScript never execute with application-origin authority.
14. File delivery defaults to attachment unless the detected MIME type is explicitly classified safe for inline delivery.
15. Filename extensions and browser-declared MIME types are not authoritative.
16. Logs never contain request bodies, Authorization values, passwords, owner tokens, encryption keys, or plaintext encrypted-share content.
17. No authentication cookies are planned for the initial anonymous capability model.
18. The API is same-origin by default; permissive CORS is disabled.
19. Security-sensitive defaults fail closed.

## Identifier and capability primitives

- A public Share ID is generated from 16 independent `crypto/rand` bytes (128 bits) and encoded with unpadded base64url, producing 22 URL-safe characters. It is opaque and never sequential or derived from timestamps/hosts. It grants no authority.
- An owner capability is generated independently from 32 `crypto/rand` bytes (256 bits). Its canonical shape is `up_o1_<base64url-32-random-bytes>` with no base64 padding. The visible `up_o1_` purpose/version prefix is not secret; the random portion is the security boundary.
- V1 stores `SHA-256(canonical_owner_token)`, not the token. A server-side pepper/HMAC key is intentionally omitted: exhaustive attack against a uniform 256-bit token is infeasible, while another critical secret would create key-loss, rotation, and recovery failure modes. A keyed verifier can be introduced if evidence changes this tradeoff.
- Verification uses constant-time digest comparison after canonical candidate parsing. Share ID and owner token generation are independent.

Phase 2 uses these primitives for owner-authorized PATCH/DELETE. Only the creation response reveals the token; management accepts exactly one `Authorization: Bearer` header and never URL/body/cookie credentials. Persistence receives only the verifier. Valid-form candidates use constant-time digest comparison, and handlers/logs never echo candidates.

SQLite connections enable foreign keys, WAL, a five-second busy timeout, NORMAL synchronization, defensive mode, and disabled double-quoted-string fallback on every physical connection. The metadata schema constrains owner verifiers to 32-byte BLOBs.

Standard Text request JSON has a 2 MiB wire limit and decoded content has a matching application/SQL range of 1–1,048,576 UTF-8 bytes. Public API responses are `no-store` and `nosniff`. `/raw/{id}` always emits exact inert `text/plain` bytes with `nosniff`, `no-store`, no-referrer, frame denial, and a restrictive sandboxed CSP; its errors receive the same headers. JSON uses normal escaping and Markdown is not rendered.

Content encryption will use Web Crypto AES-GCM. Exact encrypted-text wire encoding, CSP, and upload policy still require later review. Encrypted files require a separate design covering streaming, bounded memory, authenticated chunking, resumability, integrity, and key handling.

## Repository secret policy

This public repository must never contain real credentials, tokens, private keys, databases, uploaded content, `.env` files, secrets, or private IDE state. Examples, if introduced, use conspicuously fake values. Before every commit, inspect staged changes and scan for accidental secrets. Runtime secrets come from deployment configuration outside Git; they must not be printed.
