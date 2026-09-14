# Domain model

## Share

`Share` is the aggregate and lifetime boundary.

- **Content kind:** Text or File.
- **Initial text format:** plain text, source code, or Markdown.
- **Privacy:** Standard or Encrypted.
- **Lifecycle:** ACTIVE to EXPIRED to PURGED, or ACTIVE to DELETED to PURGED.
- **Expiration:** one optional Share-level `expires_at`; attachments inherit it.

A public resource ID identifies a Share but grants no management authority. Read policy and management capability are distinct concepts. The anonymous owner receives a high-entropy owner capability token; future management requests send it as `Authorization: Bearer <owner-token>`.

EXPIRED and DELETED Shares are immediately unavailable regardless of whether asynchronous physical purge succeeded. DELETED content is removed from active storage; deployment backup retention is separate operational policy.

## Current implementation

No domain types or operations exist in Phase 0. This document freezes vocabulary and constraints for later schema and API work, not an implemented data model.
