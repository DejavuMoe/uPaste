# ADR 0009: Embedded forward-only migrations

- Status: Accepted; implemented through schema version 3
- Date: 2026-09-14

## Context

The single-binary deployment must carry the exact schema history it understands. uPaste needs ordered, auditable upgrades without another migration executable or framework. Downgrading application code over a newer schema can corrupt or misinterpret data.

## Decision

Store monotonically numbered SQL files under `internal/database/migrations/` and embed them with `embed.FS`. Use SQLite `PRAGMA user_version` as the one integer schema marker: zero means uninitialized, version 1 means `0001_shares.sql` completed, version 2 adds `0002_standard_text_payloads.sql`, and version 3 adds `0003_encrypted_text_payloads.sql` with payload/privacy triggers.

At startup, read the current version, refuse versions newer than the binary supports, and apply each missing migration in ascending order. Execute each migration and its `user_version` update in the same transaction. On statement or commit failure, return an error and do not advance the version. Reopening at the current version performs no migration work.

Migrations are forward-only. There are no down migrations, destructive resets, or automatic deletion/recreation of unknown databases. Released migration files are audit history and must not be rewritten; corrections use a new version.

## Consequences

The executable and schema history cannot drift during deployment, startup is sufficient to upgrade an older database, and `user_version` avoids a migration metadata table for this linear history. SQLite permits this DDL transactionally, and tests prove failed migration rollback, unchanged version, fresh 0→3 setup, safe 1→3 and real 2→3 upgrades with Standard data preservation, and idempotent reopen. Rollback to older application binaries requires restoring a compatible operational backup rather than automated down SQL.
