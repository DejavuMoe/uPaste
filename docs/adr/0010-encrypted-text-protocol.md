# ADR 0010: Encrypted Text Protocol V1

- Status: Accepted; implemented in Phase 3
- Date: 2026-09-14

## Context

Encrypted Text must remain interoperable across browser releases while preventing the honest application server from receiving plaintext or a decryption key. Cryptographic parameters, encodings, and the plaintext representation therefore cannot be implementation details that drift independently.

## Decision

Freeze protocol `UPASTE_AES_GCM_V1` as AES-256-GCM using a directly generated 32-byte browser Web Crypto key, a fresh random 12-byte nonce for every encryption, an explicit 128-bit authentication tag, and the exact UTF-8 additional authenticated data `uPaste:encrypted-text:v1`. No password derivation, server secret, server-generated key, compression, or nonce derivation is used.

The plaintext is a binary envelope: byte 0 is `0x01`; byte 1 is `0x01` for PLAIN, `0x02` for SOURCE, or `0x03` for MARKDOWN; remaining bytes are 1–1,048,576 bytes of unnormalized UTF-8 content. The envelope is therefore 3–1,048,578 bytes and Web Crypto's combined ciphertext plus 16-byte tag is 19–1,048,594 bytes.

The API stores the nonce and combined ciphertext/tag as decoded BLOBs and never receives the envelope, plaintext format, or key. API nonce and ciphertext use canonical unpadded base64url. The browser key URL fragment is exactly `#up_e1_<43-character-unpadded-base64url-of-32-bytes>`. It contains only the AES key; the independent owner capability remains an Authorization bearer credential. The fragment is never intentionally placed in an HTTP request or browser storage.

The browser uses `TextEncoder` and fatal UTF-8 `TextDecoder`, performs no Unicode normalization, imports the raw key as non-extractable, and generates each key and nonce with `crypto.getRandomValues`. Updating normally reuses the fragment key with a fresh nonce so existing shared URLs remain usable. Re-keying changes the usable URL and is not a Phase 3 product feature.

## Consequences

An honest server can persist and serve authenticated ciphertext but cannot search, render, moderate, recover, or validate its plaintext. Losing the fragment makes content unrecoverable. The AES key grants confidentiality access only; it grants no PATCH or DELETE authority. The owner capability permits ciphertext replacement/deletion but does not decrypt.

The fragment remains a high-value secret exposed to anyone with the complete shared URL and potentially to browser history, clipboard, screenshots, sync, extensions, or page JavaScript. This protocol does not protect against a malicious host changing delivered JavaScript, a compromised browser, extensions with page access, traffic metadata, ciphertext size, timing, IP visibility, server-controlled expiration, or owner-capability misuse. Any incompatible parameter, envelope, AAD, or encoding change requires a new protocol version.
