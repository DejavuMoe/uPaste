# ADR 0011: Local File object storage

- Status: Accepted; implemented in Phase 4
- Date: 2026-09-14

## Context

Standard Files are too large for SQLite and must be delivered from an origin separate from application/API authority. Local filesystem objects and SQLite metadata cannot form one native transaction.

## Decision

Store File bytes under `<data-dir>/objects/` through a narrow staging/commit/open/delete storage boundary. A server-generated 16-random-byte lowercase-hex key is sharded by its first two characters; filenames never form paths. Objects are staged with `0600`, streamed with a 64 MiB limit while SHA-256 and MIME sniff bytes are computed, fsynced, then atomically linked into the shard. New roots/shards use `0700`.

Create inserts Share metadata in an immediate SQLite transaction, finalizes the staged object, inserts File metadata, then commits. On ordinary error it rolls back and compensates by removing the staged/final object. Thus a committed File Share is not intentionally left pointing at an unfinalized object. A process crash after finalization but before DB commit can leave an unreachable orphan object.

DELETE commits removal of the authoritative Share row first, then removes the object best-effort. A cleanup failure is safely logged and still returns 204: the Share remains inaccessible and an orphan awaits future reconciliation. No cleanup worker is introduced in Phase 4.

The same process runs distinct application/API and file-delivery listeners. The file listener serves only `/f/{id}` from configured `file-origin`, never based on Host or forwarded headers. Every File is a Content-Disposition attachment regardless of detected MIME.

## Consequences

SQLite remains metadata-only and a future S3 backend can implement the same narrow storage boundary. Crash/delete orphans consume disk but cannot be reached through the product because delivery resolves database metadata first. Operators need later reconciliation/GC tooling. Missing objects for an existing File Share return 500 rather than a misleading 404. File delivery is intentionally download-only; no inline preview, malware detection, encrypted File protocol, replacement PATCH, or resumable upload exists.
