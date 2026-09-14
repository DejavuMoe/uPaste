# ADR 0002: Standard-library HTTP

- Status: Accepted
- Date: 2026-09-14

## Context

The initial API is small. Framework routing, middleware, and binding features are not yet needed, while every framework expands upgrade and security surface.

## Decision

Use `net/http`, `http.ServeMux`, and `log/slog`. Add focused local helpers only when repeated code proves a need. Do not use Gin, Fiber, Echo, or Chi now.

## Consequences

Routing and lifecycle behavior remain explicit and dependency-free, and modern ServeMux supports method-aware patterns. The project must implement its own future middleware composition and validation conventions. A framework may be reconsidered only if concrete API complexity makes stdlib code less reliable or maintainable; migration cost is accepted.
