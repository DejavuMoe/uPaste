# ADR 0008: SQLite driver

- Status: Accepted; implemented in Phase 1
- Date: 2026-09-14

## Context

uPaste targets a single ordinary Linux server and uses explicit SQL through `database/sql`. Requiring CGO would add GCC and a C runtime toolchain to ordinary builds and complicate cross-compilation and release packaging.

## Decision

Use `modernc.org/sqlite` with `database/sql`, explicit SQL migrations, and no ORM. Phase 1 pins v1.58.0, the latest stable tag returned by Go module tooling at implementation time. Although upstream's changelog described v1.59.0, the tag was not published (`go list -m modernc.org/sqlite@latest` returned v1.58.0 and the module proxy reported v1.59.0 as an unknown revision), so no untagged code was selected.

v1.58.0 bundles SQLite 3.53.4 and requires `modernc.org/libc` v1.75.6 exactly; the resolved dependency graph matches that requirement. It supports the validated shorthand and security DSN parameters used here.

Every physical connection is opened with this policy:

```text
_busy_timeout=5000
_defensive=1
_dqs=0
_foreign_keys=ON
_journal_mode=WAL
_synchronous=NORMAL
_txlock=immediate
```

The driver applies these parameters when each connection opens. Integration tests hold multiple pooled physical connections concurrently and verify their effective PRAGMAs, disabled double-quoted-string fallback, and defensive mode behavior rather than inspecting only the DSN.

Phase 2.1 adds immediate write transactions after a synchronized stress test with four owner PATCH operations and four independent creates repeatedly produced SQLite error 5 (`SQLITE_BUSY`) and extended error 517 (`SQLITE_BUSY_SNAPSHOT`) under deferred transactions. Immediate acquisition makes contenders wait under the existing busy timeout before any transaction reads; the same repeated test succeeds without changing WAL, timeout, or pool size.

The `database/sql` pool allows at most four open and four idle connections, with no arbitrary connection lifetime. Shared cache, loadable extensions, writable schema, mmap/cache/auto-vacuum tuning, retries, application-wide mutexes, and OFD locking are not enabled. The pool size is a conservative single-server default, not a benchmark-derived scalability claim.

## Consequences

Builds require no CGO, host C compiler, or system SQLite and are easier to reproduce and cross-compile. The project accepts a sizable pure-Go dependency graph and must track its SQLite compatibility and security updates. Connection-level hardening is explicit and tested. OFD locking may be reviewed separately after its portability and operational behavior have sufficient evidence.
