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

- **Server Foundation (Phases 1–5.1, complete):** Standard/Encrypted Text plus Standard File create, public metadata read, owner update/delete, expiration enforcement, separated attachment-only File delivery, local object storage, maintenance purge/reconciliation, and process-local rate limiting.
- **Phase 6 (complete):** Frontend product contract, wireframes, interaction state matrix, visual design system, and implementation architecture frozen ([FRONTEND_PRODUCT_SPEC](FRONTEND_PRODUCT_SPEC.md), [FRONTEND_INTERACTION_SPEC](FRONTEND_INTERACTION_SPEC.md), [FRONTEND_VISUAL_SPEC](FRONTEND_VISUAL_SPEC.md), and [ADR 0013](adr/0013-frontend-product-architecture.md)).
- **Phase 7 (complete):** Production React UI implementation with unit, component, real-browser E2E, and 5-viewport × 2-theme visual qualification.
- **Phase 8 (complete):** Production frontend embedding and integration: the Vite bundle is embedded in the Go executable, the application listener serves the SPA with strict routing/cache/security headers, and the self-contained binary is qualified without Vite or external frontend files.
- **Phase 9 (complete):** Production release and native deployment tooling: deterministic `linux/amd64`/`linux/arm64` archives with SHA-256 checksums and version/commit metadata, hardened systemd deployment, two-origin Nginx/Caddy examples, backup/restore/upgrade/rollback guidance, and draft-only GitHub release automation. No public version tag or GitHub Release was created.
- **Phase 10 (complete):** Pre-release security, reliability, resource, concurrency, fuzz, recovery, and packaging qualification with a durable release gate. No real version tag or GitHub Release was created; the first release remains an explicit human decision.
- **Phase 11 (complete):** Public abuse protection and Superadmin governance: private/public deployment modes, Cap or Turnstile challenge-gated anonymous creation, creation-anchored public retention, a non-secret runtime config endpoint, and one high-entropy Superadmin with in-memory CSRF-protected sessions for bounded inspection and deletion. No ordinary user accounts were added.
- **Phase 12 (complete):** V1 first-release readiness and release-candidate closure inside the frozen scope: the packaged artifact is verified for exact archive contents, shipped-file equality with the claimed commit, private- and public-mode runtime contracts (challenge verification, creation-anchored retention, Superadmin governance), and deterministic amd64/arm64 rebuilds; release packaging additionally refuses any source state that could change the build while claiming HEAD (modified tracked entries and untracked non-ignored files) and requires the packaged commit to equal the checked-out HEAD. No version was selected, no License was chosen, and no tag or GitHub Release was created; first release publication remains an explicit human decision.
- **Future direction:** Public release publication, container packaging, further deployment hardening, and any explicitly approved scope expansion.
- **Out of scope now:** Encrypted Files, public listing/search, accounts, malware scanning, hard storage quotas, distributed rate limiting, Docker/container packaging, and automatic updates. Features excluded above require an explicit later decision.
