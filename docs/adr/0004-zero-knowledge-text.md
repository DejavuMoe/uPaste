# ADR 0004: Zero-knowledge encrypted text

- Status: Accepted
- Date: 2026-09-14

## Context

Some users need the application server not to learn text plaintext. Sending either plaintext or the key to the server defeats that property.

## Decision

For Encrypted Text Shares, use browser Web Crypto AES-GCM. Upload ciphertext; place the decryption key in the URL fragment so normal HTTP requests do not transmit it. The server never intentionally receives plaintext or key. Standard text remains server-visible. V1 File Shares are Standard only; this decision does not generalize encryption to files.

## Consequences

The server cannot search, render, recover, or moderate encrypted plaintext. Key loss makes content unrecoverable. Ciphertext integrity is provided by AES-GCM, while metadata including size, timing, IP information, expiration, and access patterns remains visible. A malicious operator can change delivered JavaScript and capture future data, so this is not protection from an actively malicious server. Wire format and key derivation details require a later reviewed design. Encrypted files would require a separate review of streaming, large-file memory behavior, authenticated chunking, resumability, integrity semantics, and key handling.
