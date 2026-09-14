# ADR 0006: SQLite and local filesystem persistence

- Status: Accepted direction; not implemented
- Date: 2026-09-14

## Context

The primary deployment is one ordinary VPS. Operators should not need a database cluster or object service, while a future S3-compatible option should remain possible.

## Decision

Use SQLite in WAL mode through `database/sql` with explicit SQL migrations and no ORM. Store objects on the local filesystem initially. Introduce an object-storage interface only when storage is implemented because local and S3-compatible implementations are genuinely expected.

## Consequences

Deployment and backup can remain a binary, database, and data directory. SQL stays visible and controllable. SQLite write concurrency and single-host storage set scaling limits; multi-host operation requires a later architecture change. Database and object updates require explicit consistency, cleanup, and backup handling. No repository layer or speculative storage implementation is created in Phase 0.
