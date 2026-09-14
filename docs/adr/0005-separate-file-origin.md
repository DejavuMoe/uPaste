# ADR 0005: Separate origin for untrusted files

- Status: Accepted
- Date: 2026-09-14

## Context

User files can contain active HTML, SVG, XML, or JavaScript. MIME sniffing and configuration mistakes can let such files execute with application-origin authority.

## Decision

Serve untrusted files from a separate web origin backed by a separate internal listener. A typical mapping is app/API on `127.0.0.1:8080` and files on `127.0.0.1:8081`. Do not depend only on `Host`. Default downloads to attachment unless byte-informed classification explicitly allows safe inline delivery, and send `nosniff` plus restrictive policy headers.

## Consequences

Origin isolation limits compromise of app state even when a browser executes a file. Deployment needs two proxy routes, DNS/TLS for two origins, and tests preventing listener mix-ups. The same binary can still serve both listeners. Inline convenience is deliberately reduced for safety.
