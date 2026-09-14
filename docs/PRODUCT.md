# Product

## Frozen V1 product model

The fundamental domain concept is a **Share**, not a Paste. A Share is the lifetime and authorization aggregate and contains exactly one payload:

- `TEXT`
- `FILE`

A V1 Share cannot mix text and files. V1 has no attachments, multi-file Shares, or Collection type. Adding Collections or attachments requires a new explicit architecture/domain decision.

A Text Share has format `PLAIN`, `SOURCE`, or `MARKDOWN` and privacy `STANDARD` or `ENCRYPTED`. `SOURCE` may later carry a syntax/language hint as presentation metadata, never as a security boundary. HTML supplied as text remains inert source/plain content and never becomes executable HTML.

A File Share supports `STANDARD` privacy only in V1. Encrypted File Shares are out of scope. Any later design must separately review streaming encryption, large-file memory behavior, authenticated chunking, resumability, integrity semantics, and key handling rather than generalizing the encrypted-text design.

The initial product has no accounts. Read and management capabilities are separate, and management authorization is capability-based.

There is no public feed, index, or search; no social functionality, comments, likes, or profiles; no code execution; no URL fetching or server-side previews; and no executable HTML preview.

## Conceptual lifecycle

```text
ACTIVE -> EXPIRED -> PURGED
ACTIVE -> DELETED -> PURGED
```

- **ACTIVE:** a persisted Share whose expiration has not been reached.
- **EXPIRED:** a persisted Share where `expires_at != NULL && now >= expires_at`, derived from authoritative server time. Every read rejects it without waiting for a worker to update a state value.
- **DELETED:** an owner-capability-authorized action makes the Share immediately inaccessible and, where practical, removes its active payload and active row in the operation. V1 has no recycle bin or 30-day soft delete.
- **PURGED:** expiration cleanup has physically removed remaining expired database/object data. Cleanup delay never restores access.

These names describe behavior, not a required database enum. No lifecycle `state` column should mirror them unless later implementation proves one necessary. Expiration belongs to the Share and governs its one payload. Operational backups may retain bytes according to deployment policy.

## Delivery phases

- **Current Phase 4:** Standard/Encrypted Text plus Standard File create, public metadata read, owner update/delete, expiration enforcement, and separated attachment-only File delivery. Raw plaintext remains available only for Standard Text; there is no product UI or rendering.
- **Approved future direction:** expiration/object reconciliation, product UI, and deployment hardening.
- **Out of scope now:** Encrypted Files, public listing/search, accounts, frontend product behavior, and production packaging. Features excluded above require an explicit later decision.
