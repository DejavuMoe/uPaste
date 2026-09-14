# ADR 0006: SQLite and local filesystem persistence

- Status: Accepted; SQLite foundation implemented, object storage deferred
- Date: 2026-09-14

## Context

The primary deployment is one ordinary VPS. Operators should not need a database cluster or object service, while a future S3-compatible option should remain possible.

## Decision

Use SQLite in WAL mode through `database/sql` with explicit SQL migrations and no ORM; ADR 0008 selects the driver and connection policy. Store objects on the local filesystem initially. Introduce an object-storage interface only when storage is implemented because local and S3-compatible implementations are genuinely expected.

## Consequences

Deployment and backup can remain a binary, database, and data directory. SQL stays visible and controllable. SQLite write concurrency and single-host storage set scaling limits; multi-host operation requires a later architecture change. Phase 1 introduced connection, migration, and metadata-schema foundations; Phase 2 adds transactional Standard Text rows in SQLite. Object consistency, cleanup, backup handling, File payload storage, and its interface remain deferred until those operations exist.
