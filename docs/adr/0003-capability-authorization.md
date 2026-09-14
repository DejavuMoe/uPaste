# ADR 0003: Anonymous capability-based management

- Status: Accepted
- Date: 2026-09-14

## Context

The initial product has no accounts but must distinguish viewing a Share from managing it. Public IDs are routinely exposed and cannot safely carry owner authority.

## Decision

Issue a high-entropy owner capability at Share creation. Send it for management only as `Authorization: Bearer <owner-token>`. Never put it in URLs or persist it raw. Store a verifier derived with a server-keyed cryptographic hash/HMAC and compare safely. Resource IDs grant no management access.

## Consequences

Anonymous ownership stays simple and avoids passwords, sessions, and recovery data. Losing the token means losing management access; theft grants its authority, and account-style recovery/auditing is unavailable. HTTPS, strict logging, sufficient entropy, secret-key operations, and careful client storage are mandatory. Exact token and verifier formats remain a future security decision.
