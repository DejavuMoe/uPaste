# Domain model

## Share

`Share` is the V1 lifetime and authorization aggregate. It owns exactly one payload of kind `TEXT` or `FILE`; mixed payloads, attachments, multiple files, and Collections do not exist in V1.

A Text payload has format `PLAIN`, `SOURCE`, or `MARKDOWN` and privacy `STANDARD` or `ENCRYPTED`. A future source-language hint is presentation metadata, not a security boundary. Text containing HTML remains inert text. A File payload supports only `STANDARD`; encrypted files require a separate future design review.

## Conceptual lifecycle

- **ACTIVE:** persisted and either `expires_at IS NULL` or authoritative server `now < expires_at`.
- **EXPIRED:** persisted with `expires_at IS NOT NULL` and authoritative server `now >= expires_at`.
- **DELETED:** owner-authorized deletion has made the Share inaccessible and removes the active payload and row where practical.
- **PURGED:** cleanup has physically removed data remaining after expiration.

Every future read path derives expiration and rejects expired content immediately. It must not wait for cleanup or an asynchronous state transition. Deletion has no user-visible recycle bin or fixed soft-delete period. Backups are an operational retention boundary. These lifecycle terms are conceptual and do not require a persistent `state` column; deleted rows are not intended to remain in the active Share table.

## Identifiers and management authority

A public Share ID is a 128-bit `crypto/rand` value encoded as unpadded base64url: 16 bytes become 22 URL-safe characters. It is opaque, non-sequential, contains no timestamp or host data, and grants no management authority.

The owner capability is independently generated from 32 `crypto/rand` bytes and has canonical form `up_o1_<unpadded-base64url>`. Its 256-bit random portion carries management authority. Management requests send it only as `Authorization: Bearer <owner-token>`. Persistence stores only `SHA-256(canonical_owner_token)`, and verification uses constant-time comparison. An Encrypted Text AES fragment key is a separate confidentiality capability: it decrypts but cannot manage, while the owner capability manages but cannot decrypt.

## Initial persistence constraints

The `shares` metadata table stores `id`, `payload_kind`, `privacy_mode`, `owner_token_verifier`, `created_at`, `updated_at`, and nullable `expires_at`. Timestamps are integer Unix milliseconds with UTC semantics; `expires_at = NULL` means no automatic expiration, subject to instance policy.

Standard plaintext is stored separately in `standard_text_payloads`, one row per Share, with format and 1–1,048,576 UTF-8 content bytes. Encrypted Text is stored separately in `encrypted_text_payloads` as protocol, 12-byte nonce, and 19–1,048,594-byte combined ciphertext/tag BLOBs. It stores no key, plaintext, or format. Standard Files are stored outside SQLite under server-generated opaque object keys; `file_payloads` stores only key, sanitized original filename, 1–67,108,864 byte size, server-sniffed media type, and SHA-256 digest. It stores no path or bytes. Migration triggers enforce each payload table matches its parent kind/privacy.

## Current implementation

Phase 4 retains the closed domain primitives and caller-clocked expiration helper where `now == expires_at` is expired. It implements transactional Standard/Encrypted Text and Standard File creation, explicit one-of payload modeling, active JSON reads, owner-capability mutation, physical cascade deletion, constrained payload tables, and separated File delivery. Encrypted raw plaintext is unavailable because the server has no key; File raw is unavailable because raw means Text plaintext. Expired rows remain physically present but cannot be read, updated, deleted, or revived through application operations. There is still no persisted lifecycle state, Encrypted File payload, listing, or cleanup/reconciliation operation.
