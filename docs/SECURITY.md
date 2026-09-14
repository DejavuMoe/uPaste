# Security invariants

These are mandatory design constraints. Later implementation must fail closed when a security-sensitive decision is unknown.

1. Resource IDs are identifiers, not secrets.
2. Read access and management access are separate concepts.
3. Management credentials never appear in query parameters, pathnames, logs, analytics, or Referer-visible URLs.
4. Management operations use `Authorization: Bearer <owner-token>`.
5. Raw owner tokens are never persisted server-side; a keyed cryptographic hash/HMAC verifier is planned.
6. Standard content may be visible to the server.
7. Encrypted content is zero-knowledge with respect to the application server.
8. Encrypted text is encrypted in the browser. The server stores ciphertext and never receives plaintext. The decryption key lives in the URL fragment and is never intentionally transmitted to the server.
9. Markdown raw HTML is disabled.
10. Any future Markdown HTML output also passes through sanitization as defense in depth.
11. `/raw/:id` is always inert plain text with `Content-Type: text/plain; charset=utf-8`, `X-Content-Type-Options: nosniff`, and a restrictive Content Security Policy.
12. Uploaded untrusted files are served from a separate web origin and listener from the application/API. Host-header checks alone are insufficient.
13. Active uploaded formats such as HTML, SVG, XML, and JavaScript never execute with application-origin authority.
14. File delivery defaults to attachment unless the detected MIME type is explicitly classified safe for inline delivery.
15. Filename extensions and browser-declared MIME types are not authoritative.
16. Logs never contain request bodies, Authorization values, passwords, owner tokens, encryption keys, or plaintext encrypted-share content.
17. No authentication cookies are planned for the initial anonymous capability model.
18. The API is same-origin by default; permissive CORS is disabled.
19. Security-sensitive defaults fail closed.

Future resource IDs and owner tokens must be generated with a cryptographically secure RNG. Content encryption will use Web Crypto AES-GCM. Exact encodings, sizes, HMAC construction, CSP, and upload policy will be specified and tested before those features ship.

## Repository secret policy

This public repository must never contain real credentials, tokens, private keys, databases, uploaded content, `.env` files, secrets, or private IDE state. Examples, if introduced, use conspicuously fake values. Before every commit, inspect staged changes and scan for accidental secrets. Runtime secrets come from deployment configuration outside Git; they must not be printed.
