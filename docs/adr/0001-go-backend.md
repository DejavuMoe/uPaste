# ADR 0001: Go server runtime

- Status: Accepted
- Date: 2026-09-14

## Context

uPaste targets a normal Linux VPS and should deploy as a single, low-operational-overhead application binary. The server handles HTTP, SQLite, and filesystem I/O rather than computation requiring another ecosystem.

## Decision

Use the latest stable Go 1.27 patch line for the backend. Node.js remains frontend build tooling only. Do not use Bun, Deno, or Rust for the server.

## Consequences

Go provides a static-style single-binary deployment, mature HTTP/concurrency tooling, predictable resource use, and straightforward cross-compilation. One backend language reduces build and operations surface. Node-family runtimes could share TypeScript with the frontend but would add a production runtime and dependency surface. Rust can provide tighter low-level control but costs implementation complexity and slower iteration without a demonstrated need. The team accepts Go's garbage collector and separate Go/TypeScript codebases.
