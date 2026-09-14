# Product

## Frozen product model

The fundamental domain concept is a **Share**, not a Paste. A Share contains text or a file. Initial text formats are plain text, source code, and Markdown. Privacy is either Standard or Encrypted.

The initial product has no accounts. Read and management capabilities are separate, and management authorization is capability-based.

There is no public feed, index, or search; no social functionality, comments, likes, or profiles; no code execution; no URL fetching or server-side previews; and no executable HTML preview. HTML submitted as text remains source text.

## Lifecycle

A Share follows exactly one of these paths:

```text
ACTIVE -> EXPIRED -> PURGED
ACTIVE -> DELETED -> PURGED
```

At `expires_at`, content becomes inaccessible immediately. Deletion likewise makes it immediately inaccessible and removes it from active database/storage. Physical cleanup may follow asynchronously. Backup retention is a deployment concern documented separately. Expiration belongs to the Share; attachments inherit that lifetime and have no independent expiration.

## Delivery phases

- **Current Phase 0:** health endpoint, frontend shell, tooling, CI, and design documentation.
- **Approved future direction:** Share behavior above, SQLite and filesystem persistence, capability management, encrypted text, and separated file delivery.
- **Out of scope now:** every product operation and production deployment package. Features excluded above are not planned unless a later decision explicitly changes scope.
