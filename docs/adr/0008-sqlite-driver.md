# ADR 0008: SQLite driver

- Status: Accepted direction; dependency deferred to Phase 1
- Date: 2026-09-14

## Context

uPaste targets a single ordinary Linux server and uses explicit SQL through `database/sql`. Requiring CGO would add GCC and a C runtime toolchain to ordinary builds and complicate cross-compilation and release packaging.

## Decision

Use `modernc.org/sqlite` for the first SQLite implementation. It integrates with `database/sql`, requires no CGO, simplifies Linux binary builds and cross-compilation, and fits the single-server self-hosted model. Keep explicit SQL migrations and do not introduce an ORM.

Phase 1 will pin and verify the exact dependency version. It must arrange for these settings on **every physical SQLite connection**, not execute them once on an arbitrary pooled connection:

```text
journal_mode = WAL
foreign_keys = ON
busy_timeout = 5000 ms
synchronous = NORMAL
```

Phase 1 will also verify whether the selected driver/version supports SQLite defensive mode and enable it when supported. Shared-cache mode stays disabled absent a demonstrated requirement. No speculative page-cache tuning is approved.

## Consequences

Builds avoid a host C compiler and are easier to reproduce and cross-compile. The project accepts a larger pure-Go dependency and must track its SQLite compatibility, behavior, and security updates. Connection initialization needs explicit tests because `database/sql` may create multiple physical connections. This ADR adds no dependency or database code in Phase 0.1.
