# ADR 0013: Frontend product architecture and implementation boundaries

- Status: Accepted; frozen in Phase 6 for Phase 7 implementation
- Date: 2026-09-14

## Context

With the server foundation completed in Phases 0–5.1 (supporting Standard/Encrypted Text and Standard File shares, dual-listener physical isolation, Go `os.Root` object storage, and process-local rate limiting), the project requires a formal frontend architecture before implementing the production React UI. 

Without a frozen architecture and interaction contract, frontend development risks "design-by-implementation," accidental API churn, secret leakage to browser persistent storage, adoption of heavy UI frameworks, and unvetted client dependencies.

## Decision

Freeze the frontend architecture, boundaries, and implementation strategy for Phase 7:

### 1. Technology stack
- **Core stack**: Retain React `19`, TypeScript `7`, and Vite `8` (`web/`). No re-platforming or meta-frameworks (e.g. Next.js, Remix, Astro).
- **Styling**: Global semantic CSS design tokens and plain CSS / CSS Modules. No heavy UI component libraries (MUI, Ant Design, Mantine, Chakra), no utility CSS frameworks (Tailwind), and no large uncurated component dumps (shadcn).
- **Routing**: A small, established, production-grade React client-side router will be pinned and adopted in Phase 7 to reliably manage navigation, history, and URL fragment preservation. Hand-rolling browser history and popstate handling is rejected.
- **State management**: React local component state, route parameters/state, and focused custom hooks. No global state libraries (Redux, Zustand, MobX, XState) or server-cache libraries (TanStack Query) are permitted in V1.
- **Networking**: Browser-native `fetch` and `AbortController`. No Axios or heavy networking clients. For File upload progress, a small focused `XMLHttpRequest` helper is permitted if required by browser progress event limitations.

### 2. Frozen routes and information architecture
- Only three product routes exist:
  - `/`: Create Share (Text or File workspace)
  - `/s/:id`: Public Share viewer
  - `/manage/:id`: Owner management (Update content, adjust expiration, permanent delete)
- Omitted features: No user accounts, login, session cookies, public listing/feed, content search, local share history, or administration dashboards.

### 3. Capability separation and secret non-persistence
- **OwnerToken non-persistence**: The owner token (`up_o1_...`) is displayed once upon creation and may be passed to `/manage/:id` exclusively in transient, volatile React router memory. It is **never** written to `localStorage`, `sessionStorage`, `document.cookie`, or `IndexedDB`, and never placed in URLs, query strings, or fragments.
- **Encrypted decryption key**: The AES-256-GCM key exists exclusively in the browser URL fragment (`#up_e1_<base64url>`) and volatile memory. It is never transmitted in API request headers or bodies, never stored in browser storage, and never logged.
- **Authority separation**: The decryption key grants confidentiality only. The owner token authorizes mutation and deletion but cannot decrypt ciphertext.

### 4. Separate File origin
- File downloads (`/f/:id`) remain strictly isolated on the independently configured File listener origin (e.g. `http://127.0.0.1:8081`). The frontend application never proxies file bytes through the main application origin.

### 5. Markdown rendering security boundary
- Client-side rendering only; raw HTML is disabled.
- Output must pass through an independent, audited HTML sanitizer before DOM insertion.
- Scripts, styles, iframes, objects, embeds, and executable URL schemes (`javascript:`, `data:`) are strictly forbidden.
- External images are not automatically embedded by default to prevent IP/metadata leakage to third parties. Safe links must include `rel="noopener noreferrer nofollow"`.
- Concrete safe rendering dependencies will be pinned in Phase 7; no dependencies are added in Phase 6.

### 6. Zero external telemetry and asset isolation
- Zero analytics, trackers, external logging beacons, or error-reporting SaaS SDKs.
- Zero remote fonts (system font stacks only) and zero external CDN script dependencies.

### 7. Phase 7 testing strategy
- **Unit tests**: Vitest for utility formatters, expiration calculations, envelope serialization/deserialization, and route helpers.
- **Component tests**: React Testing Library and `user-event` for user interactions, accessibility, keyboard navigation, and focus trapping.
- **Browser E2E tests**: Automated headless browser testing covering end-to-end flows: Standard Text creation/viewing, Encrypted Text client-side encryption/decryption, File upload and direct download, owner management (update and delete), and error states (404, 410, 429).
- **Visual QA contract**: Explicit visual inspection across 5 target viewports (360px, 390px, 768px, 1024px, 1440px) in both Light and Dark themes.

## Consequences

- **Invariants**: No production runtime code in `web/` or `cmd/`/`internal/` is modified in Phase 6. Database schema version remains 4. Existing tests continue to pass with 100% fidelity.
- **Implementation readiness**: Phase 7 can implement the complete React UI directly from this contract without improvising product rules, layout decisions, or security boundaries.
