# Phase 3 HTTP API

The API implements anonymous Standard and zero-knowledge Encrypted Text Shares. It is same-origin and emits no permissive CORS headers. There is no listing or search endpoint.

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
| 413 | `request_too_large` | JSON wire body or decoded text exceeds its limit. |
| 415 | `unsupported_media_type` | Request is not identity-encoded JSON. |
| 422 | `unsupported_share_type` | A valid domain combination, currently File Shares, is not implemented. |
| 500 | `internal_error` | An internal operation failed; implementation details are not exposed. |

API timestamps are RFC3339 UTC. Incoming timestamps accept RFC3339/RFC3339Nano and are normalized to UTC millisecond precision. Expiration is authoritative server time: `now >= expires_at` is expired and returns 410 without content. Expired Shares cannot be updated, deleted, or revived and await a future cleanup mechanism.

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

## Delete

```http
DELETE /api/v1/shares/{id}
Authorization: Bearer <owner-token>
```

An active authorized Share is physically removed and returns `204 No Content`. Standard or Encrypted payload is removed by foreign-key cascade. Later reads return 404, and a second delete returns 404. Expired Shares return 410 and remain for future cleanup. There is no soft delete or recycle bin.

## Methods

- `/api/v1/shares`: `POST`
- `/api/v1/shares/{id}`: `GET, PATCH, DELETE`
- `/raw/{id}`: `GET`
- `/healthz`: `GET` lightweight liveness

Other methods return 405 with `Allow`. API method errors use JSON; raw method errors stay inert plain text. No `OPTIONS` or CORS behavior is added.
