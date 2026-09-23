# uPaste HTTP API

The application API implements anonymous Standard Text, Encrypted Text, and Standard File Shares. It emits no permissive CORS headers; File bytes use the separately configured file origin. There is no listing or search endpoint.

## Common behavior

All `/api/v1/` responses, including errors, send:

```http
Cache-Control: no-store
X-Content-Type-Options: nosniff
```

Application errors use:

```json
{"error":{"code":"not_found","message":"share not found"}}
```

| Status | Code | Meaning |
|---|---|---|
| 400 | `invalid_request` | Malformed JSON, invalid enum/input, unknown field, or invalid expiration. |
| 401 | `unauthorized` | Missing, malformed, duplicate, or incorrect owner authorization. |
| 404 | `not_found` | Malformed ID or missing Share. |
| 409 | plain text | Raw view is unavailable for an active Encrypted Share. |
| 410 | `expired` | The Share reached its expiration. |
| 413 | `request_too_large` | JSON, multipart wire body, metadata, decoded text, or File exceeds its limit. |
| 415 | `unsupported_media_type` | Request is not identity-encoded JSON or multipart/form-data. |
| 422 | `unsupported_share_type` | A valid domain combination, currently `FILE + ENCRYPTED`, is not implemented. |
| 429 | `rate_limited` | Process-local IP-derived limiter rejected the request; `Retry-After` is set. |
| 500 | `internal_error` | An internal operation failed; implementation details are not exposed. |

API timestamps are RFC3339 UTC. Incoming timestamps accept RFC3339/RFC3339Nano and are normalized to UTC millisecond precision. Expiration is authoritative server time: `now >= expires_at` is expired and returns 410 without content. Expiration is synchronously authoritative for accessibility. Expired Shares cannot be read, updated, deleted, or revived through normal application operations. Phase 5 maintenance later physically purges expired metadata/payloads asynchronously. For expired File Shares, physical File-object cleanup remains subject to the existing maintenance/reconciliation rules.

## Runtime configuration

```http
GET /api/v1/config
```

Unauthenticated and non-secret. Private mode returns:

```json
{"deployment_mode":"private","admin_enabled":false}
```

Public mode returns `deployment_mode: "public"`, `admin_enabled: true`, the
configured `retention` object (`default_seconds`, `max_seconds`), and the
non-secret `challenge` object (`provider`, `site_key`, plus Cap's `api_endpoint`
or Turnstile's `hostname`). Challenge secrets, the Superadmin token, owner
capabilities, and Share content are never included. The frontend uses this
endpoint to enable challenge-aware creation and public retention limits; if it
cannot be loaded, the frontend keeps private-mode defaults and surfaces the
configuration error instead of assuming public policy.

## Rate limits

Current self-hosted defaults use trusted-proxy-aware client IP identity: create 10/minute (burst 5), owner mutation 30/minute (burst 10), public reads 120/minute (burst 60), and a process-wide 1000/minute burst-200 limiter. File uploads/downloads also have non-blocking process-wide gates of 4/32. API limits return the normal `429 rate_limited` envelope and `Retry-After`; File-origin limits return plain-text 429 with the same header and File security headers. Limits are in-memory, reset on restart, and are not distributed or DDoS protection.

Expired Shares become inaccessible synchronously but physical DB/object cleanup is asynchronous; there is no maintenance endpoint.

## Create Text

```http
POST /api/v1/shares
Content-Type: application/json
Content-Encoding: identity
```

`Content-Encoding` may be omitted. Other encodings are rejected; the server does not decompress requests. The JSON body has a 2 MiB wire limit.

```json
{
  "payload_kind": "TEXT",
  "privacy_mode": "STANDARD",
  "text": {
    "format": "PLAIN",
    "content": "hello"
  },
  "expires_at": null
}
```

Every field is required. Text format is `PLAIN`, `SOURCE`, or `MARKDOWN`. Content is preserved without trimming and must contain 1–1,048,576 UTF-8 bytes; whitespace-only content is valid. The wire limit is independent because JSON escaping and structure also consume bytes. `expires_at` is either explicit `null` or a timestamp strictly after server time once normalized. Request JSON uses Go's strict JSON v2 semantics: duplicate or unknown object members, invalid UTF-8, malformed input, trailing values, and incorrectly cased field names are rejected with the generic `400 invalid_request` response.

Success is `201 Created`:

```http
Location: /api/v1/shares/<id>
Cache-Control: no-store
X-Content-Type-Options: nosniff
Content-Type: application/json; charset=utf-8
```

```json
{
  "share": {
    "id": "<22-character-share-id>",
    "payload_kind": "TEXT",
    "privacy_mode": "STANDARD",
    "text": {"format": "PLAIN", "content": "hello"},
    "created_at": "2026-09-14T03:00:00Z",
    "updated_at": "2026-09-14T03:00:00Z",
    "expires_at": null
  },
  "owner_token": "<returned-once-owner-capability>"
}
```

This is the only response that reveals the owner token. The `Location` contains only the public Share ID. The server persists only the token's SHA-256 verifier.

For `TEXT + STANDARD`, `text` is required and `encrypted_text` is rejected. For `TEXT + ENCRYPTED`, `encrypted_text` is required and `text` is rejected. Supplying both or neither returns 400. File combinations remain unimplemented and return 422.

### Encrypted Text

The browser encrypts before POST and sends only:

```json
{
  "payload_kind": "TEXT",
  "privacy_mode": "ENCRYPTED",
  "encrypted_text": {
    "protocol": "UPASTE_AES_GCM_V1",
    "nonce": "<16-character-unpadded-base64url>",
    "ciphertext": "<canonical-unpadded-base64url-ciphertext-and-tag>"
  },
  "expires_at": null
}
```

The protocol is AES-256-GCM with a fresh 12-byte nonce, 128-bit tag, and exact AAD `uPaste:encrypted-text:v1`. The API decodes nonce and ciphertext to BLOBs. Ciphertext includes the tag and must be 19–1,048,594 bytes; malformed/noncanonical encoding returns 400 and oversized ciphertext returns 413. The maximum canonical request is about 1.4 MB and fits the unchanged 2 MiB wire ceiling.

Success is 201 with ordinary metadata, `privacy_mode: "ENCRYPTED"`, and the same `encrypted_text` object. It contains no `text`, format, AES key, key fragment, or verifier. The owner capability is returned once exactly as for Standard creation.

The browser-generated 32-byte AES key exists only in the canonical fragment `#up_e1_<43-character-unpadded-base64url>`, producing links such as `https://paste.example/p/<id>#up_e1_<key-placeholder>`. URL fragments are not normally sent in HTTP requests, and the client library never places this key in API JSON, query parameters, cookies, or browser storage.

### Create Standard File

Use the same resource with a streamed multipart body:

```http
POST /api/v1/shares
Content-Type: multipart/form-data; boundary=...
```

It has exactly `metadata` and `file` parts in either order. Metadata is an `application/json` part with strict JSON-v2 semantics:

```json
{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}
```

The file part requires a filename. The full wire body is capped at 66 MiB, metadata at 64 KiB, and decoded file bytes at 1–67,108,864 bytes. `FILE + ENCRYPTED` returns 422; encrypted Files are not implemented. Unknown/duplicate parts, duplicate or unknown metadata fields, malformed disposition/JSON, and invalid field casing return 400.

Filenames are metadata only: path components (including legacy backslash paths) are reduced to a basename; empty, NUL, CR/LF/control-character, invalid UTF-8, and >255-byte names are rejected. The server generates an opaque storage key and never exposes it. It sniffs up to 512 bytes with `http.DetectContentType`, streams the object while computing SHA-256, and returns 201 with:

```json
{"share":{"id":"<id>","payload_kind":"FILE","privacy_mode":"STANDARD","file":{"filename":"report.pdf","size":123,"media_type":"application/pdf","sha256":"<64-lowercase-hex>","download_url":"https://files.example/f/<id>"},"created_at":"...","updated_at":"...","expires_at":null},"owner_token":"<returned-once-owner-capability>"}
```

`download_url` is constructed only from configured file origin, never Host or forwarded headers. It exposes no storage key/path or declared client MIME.

## Read

```http
GET /api/v1/shares/{id}
```

An active Share returns `200` with `{"share": ...}` and never includes owner token/verifier data. Standard responses contain only `text`; Encrypted responses contain only protocol, nonce, and ciphertext under `encrypted_text`. Encrypted responses contain no plaintext format or key. A malformed or missing ID returns 404. An expired Share returns 410 without its payload.

## Raw text

```http
GET /raw/{id}
```

An active Share returns the exact stored bytes without an appended newline. `PLAIN`, `SOURCE`, and `MARKDOWN` are all served as inert text; HTML, SVG, Markdown, and JavaScript-shaped content is never interpreted.

Every raw response, including errors, sends:

```http
Content-Type: text/plain; charset=utf-8
X-Content-Type-Options: nosniff
Cache-Control: no-store
Referrer-Policy: no-referrer
X-Frame-Options: DENY
Content-Security-Policy: default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; sandbox
```

Malformed/missing IDs return 404 and expired Shares return 410. An active Encrypted Share returns `409 Conflict` with `raw view unavailable for encrypted share`; expiration takes precedence and still returns 410. The raw route never accepts a key and never returns ciphertext as plaintext. Error bodies never contain stored text.

## Owner authorization

`PATCH` and `DELETE` accept the owner capability only as:

```http
Authorization: Bearer <owner-token>
```

The Bearer scheme is case-insensitive; the token is case-sensitive. Missing, malformed, duplicate, and incorrect Authorization headers all return 401 with `WWW-Authenticate: Bearer`. Tokens in query strings, paths, bodies, or cookies are ignored and never authorize an operation. Management credentials must not be placed in URLs.

The owner capability and AES fragment have independent powers: the fragment decrypts ciphertext but cannot PATCH or DELETE; the owner capability can replace or delete ciphertext but cannot decrypt it. An `up_e1_` key is never accepted as owner authorization.

For PATCH, the server parses the route and Authorization header, then runs a metadata-only check that verifies the Share exists, is active, and the owner capability matches before reading or validating the body. An active Share with a wrong valid capability therefore returns 401 even when its body is invalid. The transactional update repeats existence, expiration, and capability checks before mutation. A syntactically present Bearer credential for an expired Share may receive 410 before digest comparison because expiration is public state; missing or malformed Authorization still returns 401 immediately.

## Update

```http
PATCH /api/v1/shares/{id}
Authorization: Bearer <owner-token>
Content-Type: application/json
```

Replace text, change expiration, or do both:

```json
{
  "text": {"format": "SOURCE", "content": "package main"},
  "expires_at": "2026-09-20T00:00:00Z"
}
```

`text`, when present, requires both format and content and uses the create limits. An omitted `expires_at` remains unchanged; explicit `"expires_at": null` clears automatic expiration. A supplied timestamp must be strictly future. At least one mutable field is required. IDs, type/privacy, creation time, and owner capability are immutable and rejected as unknown fields.

For an Encrypted Share, replace ciphertext locally encrypted with the existing fragment key and a fresh nonce:

```json
{
  "encrypted_text": {
    "protocol": "UPASTE_AES_GCM_V1",
    "nonce": "<fresh-16-character-nonce>",
    "ciphertext": "<replacement-ciphertext-and-tag>"
  },
  "expires_at": null
}
```

Standard Shares reject `encrypted_text`; Encrypted Shares reject `text`. Expiration-only PATCH works for both. Privacy is immutable; changing privacy requires creating a new Share. The server cannot prove nonce uniqueness, so the official browser helper generates a fresh random nonce for every encryption.

Success returns `200` and `{"share": ...}` with server-clock `updated_at`, but never returns the owner token or AES key. Metadata and the appropriate payload change commit atomically.

For File Shares PATCH supports expiration only. `text`, `encrypted_text`, filename, MIME, content, kind, and privacy changes are rejected. Replace content by deleting and creating a new Share.

## File delivery origin

The application listener intentionally does not serve `/f/*`. The separately configured file listener serves only:

```http
GET /f/{id}
HEAD /f/{id}
```

Active Files use the server-sniffed `Content-Type`, strong ETag `"<64-lowercase-hex-sha256>"`, and `Content-Disposition: attachment` for every MIME type. `http.ServeContent` provides normal single-range support: valid ranges return 206 and invalid ranges 416. HEAD has identical metadata/security headers and no body.

All `/f/*` responses, including errors, use `nosniff`, `no-store`, no-referrer, frame denial, and the restrictive sandboxed CSP. No file is inline, no CORS is enabled, and the upload-declared MIME never controls delivery. Malformed/missing/Text/Encrypted IDs return 404; expired File Shares return 410. An existing File Share with a missing object returns 500 because storage is inconsistent.

## Delete

```http
DELETE /api/v1/shares/{id}
Authorization: Bearer <owner-token>
```

An active authorized Share is physically removed and returns `204 No Content`. Standard or Encrypted payload is removed by foreign-key cascade. Later reads return 404, and a second delete returns 404. Expired Shares return 410 and are later purged asynchronously by maintenance. There is no soft delete or recycle bin.

## Superadmin administration

`/admin` and `/api/v1/admin/*` do not exist unless `UPASTE_ADMIN_TOKEN` is configured; without it they return 404. The single high-entropy `up_a1_` capability is exchanged for an in-memory session:

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/v1/admin/session` | `{"token":"up_a1_..."}`; sets a host-only HttpOnly `SameSite=Strict` session cookie (`Secure` in public mode) and returns `{"authenticated":true,"csrf":"...","expires_at":"..."}` |
| `GET` | `/api/v1/admin/session` | Returns the current session and CSRF token, or 401 |
| `DELETE` | `/api/v1/admin/session` | Ends the session; requires `X-uPaste-CSRF` |
| `GET` | `/api/v1/admin/summary` | Aggregate counts: `active`, `expired`, `text`, `file`, `encrypted`, `file_bytes` |
| `GET` | `/api/v1/admin/shares` | Metadata-only listing with `limit` (1–100, default 50), `kind`, `privacy`, `lifecycle` (`active`/`expired`), `sort` (`newest`/`oldest`), `id`, and `cursor`; returns `{"shares":[...],"next_cursor":"..."}` |
| `GET` | `/api/v1/admin/shares/{id}` | On-demand detail: server-visible Text content, File metadata, or encrypted metadata only |
| `DELETE` | `/api/v1/admin/shares/{id}` | Removes the Share and its payload; requires `X-uPaste-CSRF` |
| `POST` | `/api/v1/admin/shares/bulk-delete` | `{"ids":[...]}` with 1–100 IDs; requires `X-uPaste-CSRF`; returns `{"deleted":n,"failed":[...]}` |
| `POST` | `/api/v1/admin/cleanup/expired` | Runs the established expired purge/reconciliation pass; requires `X-uPaste-CSRF`; returns `{"purged":n}` |

Listing rows contain `id`, `payload_kind`, `privacy_mode`, `state`, `created_at`, `updated_at`, `expires_at`, `payload_bytes`, and lightweight File metadata only. Detail responses never include storage keys, owner verifiers, or encryption keys; an Encrypted Share reports `protocol`, `nonce`, and `ciphertext_bytes` with a notice, never plaintext or ciphertext bytes. Superadmin cannot edit content, recover OwnerTokens, or decrypt encrypted Shares.

Unauthenticated requests return 401 `unauthorized`; state-changing requests without a valid session-bound `X-uPaste-CSRF` header return 403 `csrf_failed`. Login attempts are rate limited per client identity and return 429 with `Retry-After`. Admin responses send `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, and `Referrer-Policy: no-referrer`, and never add CORS headers.

## Methods

- `/api/v1/shares`: `POST`
- `/api/v1/shares/{id}`: `GET, PATCH, DELETE`
- `/api/v1/config`: `GET`
- `/api/v1/admin/*`: `GET, POST, DELETE` when `UPASTE_ADMIN_TOKEN` is configured, otherwise 404
- `/raw/{id}`: `GET`
- `/healthz`: `GET` lightweight liveness

Other methods return 405 with `Allow`. API method errors use JSON; raw method errors stay inert plain text. No `OPTIONS` or CORS behavior is added.
