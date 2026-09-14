# ADR 0007: Resource identifiers and owner capabilities

- Status: Accepted
- Date: 2026-09-14

## Context

Public identifiers must resist practical enumeration without being mistaken for authorization. Anonymous management needs a separate capability with enough entropy that a stolen verifier database does not enable feasible offline recovery. A keyed HMAC verifier would add a deployment secret whose loss invalidates every management capability and whose rotation/recovery would add complexity.

## Decision

Generate each public Share ID from 16 bytes (128 bits) read from Go `crypto/rand`, encoded with base64url without padding. The result is 22 URL-safe characters. IDs are opaque, non-sequential, contain no timestamp or host information, and never grant management authority.

Generate each owner capability independently from 32 `crypto/rand` bytes (256 bits), encoded as unpadded base64url and prefixed with `up_o1_`. The canonical form is `up_o1_<base64url-32-random-bytes>`. The human-visible prefix identifies purpose, supports secret scanning and future format versions, and carries no entropy.

Persist only `SHA-256(canonical_owner_token)`. V1 has no server-side pepper or HMAC key. Future verification compares the computed and stored digest in constant time.

## Consequences

A 128-bit random ID is compact while enumeration remains infeasible when combined with rate controls. IDs stay opaque, so a future generator can increase entropy without changing existing IDs. The independent 256-bit token is the sole management security boundary; its uniform search space makes offline brute force infeasible even if SHA-256 verifiers leak.

Removing HMAC avoids key distribution, loss, rotation, and recovery failure modes. It does not protect weak or user-chosen tokens, so implementations must generate exactly the specified random token and never accept reduced-entropy owner credentials. Raw tokens remain unrecoverable server-side; loss by the owner means management access cannot be restored. A keyed verifier remains a compatible future migration if a concrete threat requirement justifies it.
