# Phase 2 HTTP API

The Phase 2 API implements anonymous `STANDARD` Text Shares only. It is same-origin and emits no permissive CORS headers. There is no listing or search endpoint.

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
| 410 | `expired` | The Share reached its expiration. |
| 413 | `request_too_large` | JSON wire body or decoded text exceeds its limit. |
| 415 | `unsupported_media_type` | Request is not identity-encoded JSON. |
| 422 | `unsupported_share_type` | A valid domain combination is not implemented in Phase 2. |
| 500 | `internal_error` | An internal operation failed; implementation details are not exposed. |

API timestamps are RFC3339 UTC. Incoming timestamps accept RFC3339/RFC3339Nano and are normalized to UTC millisecond precision. Expiration is authoritative server time: `now >= expires_at` is expired and returns 410 without content. Expired Shares cannot be updated, deleted, or revived and await a future cleanup mechanism.

## Create Standard Text

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

Every field is required. Text format is `PLAIN`, `SOURCE`, or `MARKDOWN`. Content is preserved without trimming and must contain 1–1,048,576 UTF-8 bytes; whitespace-only content is valid. The wire limit is independent because JSON escaping and structure also consume bytes. `expires_at` is either explicit `null` or a timestamp strictly after server time once normalized. Unknown fields and trailing JSON values are rejected.

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

Valid but unsupported Phase 2 HTTP types return 422: `TEXT + ENCRYPTED`, `FILE + STANDARD`, and `FILE + ENCRYPTED`. Their future domain status does not imply an implemented endpoint.

## Read

```http
GET /api/v1/shares/{id}
```

An active Share returns `200` with `{"share": ...}` and never includes owner token/verifier data. A malformed or missing ID returns 404. An expired Share returns 410 without its content.

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

Malformed/missing IDs return 404 and expired Shares return 410. Error bodies never contain stored text.

## Owner authorization

`PATCH` and `DELETE` accept the owner capability only as:

```http
Authorization: Bearer <owner-token>
```

The Bearer scheme is case-insensitive; the token is case-sensitive. Missing, malformed, duplicate, and incorrect Authorization headers all return 401 with `WWW-Authenticate: Bearer`. Tokens in query strings, paths, bodies, or cookies are ignored and never authorize an operation. Management credentials must not be placed in URLs.

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

Success returns `200` and `{"share": ...}` with server-clock `updated_at`, but never returns the owner token. Metadata and payload changes commit atomically.

## Delete

```http
DELETE /api/v1/shares/{id}
Authorization: Bearer <owner-token>
```

An active authorized Share is physically removed and returns `204 No Content`. The payload is removed by foreign-key cascade. Later reads return 404, and a second delete returns 404. Expired Shares return 410 and remain for future cleanup. There is no soft delete or recycle bin.

## Methods

- `/api/v1/shares`: `POST`
- `/api/v1/shares/{id}`: `GET, PATCH, DELETE`
- `/raw/{id}`: `GET`
- `/healthz`: `GET` lightweight liveness

Other methods return 405 with `Allow`. API method errors use JSON; raw method errors stay inert plain text. No `OPTIONS` or CORS behavior is added.
