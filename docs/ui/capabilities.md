# Current UI capabilities and states

This inventory describes the Windows working tree, including the current
uncommitted create-page draft-navigation guard. It is based on production source,
API handlers, and tests; this audit did not run the app or browser tests. Existing
design specifications describe earlier intent and are not used as proof of
current behavior.

## Users and primary jobs

- An anonymous creator publishes one text or one file payload, chooses an
  expiration, then saves a read link and a one-time management token.
- A recipient opens a read link to view text or file metadata and may download a
  file. An encrypted text recipient needs the complete URL fragment to decrypt.
- A management-token holder changes text or expiration, or permanently deletes
  a Share. Encrypted text editing also needs the decryption fragment; a File Share
  supports only expiration changes and deletion.
- When configured, one Superadmin signs in to inspect storage totals and Shares,
  delete one or up to 100 selected Shares, and run expired cleanup. There are no
  ordinary accounts. Evidence: `docs/PRODUCT.md`, `internal/domain/domain.go`,
  `web/src/features/{create,share,manage,admin}/`, `internal/adminapi/adminapi.go`.

## Surfaces

| Surface ID | Route/window/device | Purpose | Primary actions | Evidence/source files |
|---|---|---|---|---|
| `shell` | All SPA routes | Brand/home link and system/light/dark theme selection | Go home; change theme | `web/src/app/App.tsx`, `web/src/components/AppHeader.tsx`, `web/src/app/theme.tsx` |
| `create` | `/` | Create Text or File Share | Select tab, format/privacy/expiration, enter text or choose/drop one file, submit | `web/src/features/create/{CreatePage,TextCreateForm,FileCreateForm}.tsx` |
| `created` | `/`, after successful POST | Deliver the read link and owner capability once | Copy link/token, reveal token, open or manage Share, start new Share | `web/src/features/create/CreationSuccess.tsx` |
| `read` | `/s/:id` | Read Text/Markdown or File metadata | Copy text, switch Markdown view, wrap source lines, open standard-text Raw, download File | `web/src/features/share/{ShareRoute,TextViewer,MarkdownViewer,FileViewer}.tsx` |
| `manage` | `/manage/:id` | Owner-capability mutation | Enter token if absent, edit Text, change expiration, save, delete | `web/src/features/manage/ManageRoute.tsx` |
| `admin` | `/admin` when enabled | Superadmin governance | Sign in/out, filter/sort/lookup, inspect detail, select/delete, clean expired | `web/src/features/admin/AdminPage.tsx`, `internal/webapp/webapp.go` |
| `raw` | App origin `/raw/:id` | Inert plain-text response for Standard Text | Open from Standard Text viewer | `internal/httpapi/httpapi.go`, `web/src/features/share/{TextViewer,MarkdownViewer}.tsx` |
| `file-download` | Separate File origin `/f/:id` | Attachment response, not an app page | Download a Standard File | `internal/fileapi/fileapi.go`, `web/src/features/share/FileViewer.tsx` |

The embedded server admits only `/`, `/index.html`, `/s/:id`, `/manage/:id`,
and enabled `/admin` as SPA entry paths. The React wildcard navigates to `/`
only after the SPA has loaded; an unknown direct path receives server 404.
Evidence: `web/src/app/App.tsx`, `internal/webapp/webapp.go`.

## Domain objects and terminology

| Object/concept | User-visible fields/values | Allowed actions | Permission/ownership rules | Evidence |
|---|---|---|---|---|
| Share | Link/ID, `TEXT` or `FILE`, `STANDARD` or `ENCRYPTED`, creation and optional expiration; API also carries `updated_at` | Create, read, update, delete | Read by ID; update/delete require the Share's bearer management token; expired Shares reject reads | `web/src/app/types.ts`, `internal/domain/domain.go`, `internal/httpapi/httpapi.go`, `AdminPage.tsx` |
| Text payload | `PLAIN`, `SOURCE`, `MARKDOWN`; content up to 1 MiB UTF-8 | Edit, copy, view; Raw only when Standard | Standard content is server-visible; Encrypted content is encrypted/decrypted in the browser | `web/src/features/create/TextCreateForm.tsx`, `web/src/crypto/encryptedText.ts`, `web/src/features/share/` |
| File payload | Recipient sees filename, size, MIME, expiration and download; admin detail also shows SHA-256; 1 byte–64 MiB | Upload once, view metadata, download | Standard only; management cannot replace/edit file bytes | `web/src/features/create/FileCreateForm.tsx`, `web/src/features/share/FileViewer.tsx`, `web/src/features/admin/AdminPage.tsx`, `internal/domain/domain.go` |
| Read link / decryption key | `/s/:id`; encrypted link adds `#up_e1_…` | Copy/open | Complete encrypted link grants decryption; fragment is not sent to the Share API | `web/src/features/create/CreationSuccess.tsx`, `web/src/crypto/encryptedText.ts`, `web/e2e/encrypted-management.spec.ts` |
| Management token | `up_o1_…`, masked at creation until revealed | Copy, enter, use for mutation | Shown in creation result once; retained only in React memory for live navigation, lost on reload | `web/src/features/create/CreationSuccess.tsx`, `web/src/app/ownerCapabilities.tsx`, `web/e2e/management.spec.ts` |
| Expiration | Never, 1 hour, 1 day, 7 days, 30 days, custom date/time as available | Set at creation; mutate with owner token | Public mode requires finite expiration, bounded by policy and anchored to creation; private mode permits Never | `web/src/components/ExpirationField.tsx`, `web/src/features/manage/ManageRoute.tsx`, `internal/config/config.go`, `internal/share/service.go` |
| Superadmin session | Admin token sign-in; summary and metadata listing | Inspect, delete, clean expired, log out | Available only when enabled; HttpOnly cookie and session-bound CSRF header; encrypted plaintext and owner token are unavailable | `web/src/app/adminApi.ts`, `web/src/features/admin/AdminPage.tsx`, `internal/adminapi/adminapi.go` |

A Share contains exactly one Text or File payload. Its lifecycle is derived from
server time: active, expired, then purged by maintenance, or deleted immediately.
There is no public listing/search or account-based ownership. Evidence:
`internal/domain/domain.go`, `internal/share/service.go`, `docs/PRODUCT.md`.

## State matrix

| Surface/flow | Ready | Loading | Empty | Error | Disabled | Permission/offline | Progress/cancel/retry | Platform-specific |
|---|---|---|---|---|---|---|---|---|
| Runtime config | `GET /api/v1/config` supplies deployment mode, optional retention/challenge, admin availability | Provider tracks `loading`; forms do not wait for it | Initial private defaults until response | Provider tracks `error`; initial failure keeps private defaults, later reload failure keeps prior config; no dedicated error view | — | No dedicated offline state | `reload()` exists in context but no visible control calls it | Browser fetch (`web/src/app/config.tsx`) |
| Text create | Format/privacy/expiry, editor and byte count | `Creating…`; editor/options disabled | Empty text disables submit | Inline alert; 429 wait text; draft retained | >1 MiB, invalid expiry, pending POST, or unsolved public challenge | Public Cap/Turnstile gate; no dedicated offline screen | Request aborts on unmount; failed submit can be retried; no visible cancel control | Browser Web Crypto for Encrypted Text (`TextCreateForm.tsx`, `ChallengeGate.tsx`) |
| File create | Choose/drop one File, remove it, set expiry | `Preparing…`/`Uploading…` | No file: drop zone, submit disabled | Zero/oversize file rejected; upload error alert and selection retained | Missing/invalid file, invalid expiry, pending upload, or unsolved challenge | Same public challenge; no dedicated offline screen | XHR byte/percent progress when measurable; unmount aborts; no visible upload cancel | File input and drag/drop (`FileCreateForm.tsx`, `api.ts`) |
| Creation result | Read link and masked owner token | — | — | Copy failure falls back to browser prompt | — | Token cannot be recovered from the screen after leaving/reload | Copy feedback; Open/Manage/New actions | Clipboard API with fallback (`CreationSuccess.tsx`, `CopyButton.tsx`) |
| Read Share | Text, rendered/source Markdown, or File metadata; no automatic File-byte fetch | `Loading share…`; encrypted text then `Decrypting…` | No separate blank-share state | 404, 410, 429, network/server, missing/wrong key have distinct screens | Raw absent for encrypted; unsafe File URL shows unavailable | Complete fragment required to decrypt; network error has retry | 429/network/server error offers Try again; fetch aborts on unmount | File download leaves SPA for separate origin (`ShareRoute.tsx`, `ShareErrorState.tsx`, viewers) |
| Manage Share | Owner token gate, editable Text or File metadata with expiry/delete | `Loading share…`; `Saving…`/`Deleting…` | Empty Text edit cannot save | 404/410/server terminal states; save errors in status; 401 forgets token but keeps draft | Save blocked unless changed and valid; File/content-without-key editing absent | Bearer token required for mutation; no dedicated offline screen | Delete and dirty navigation have confirmation dialogs; failed save can retry | Ctrl/Cmd+Enter saves from Text editor (`ManageRoute.tsx`) |
| Superadmin | Summary, filters, exact ID, paged table and detail | Initial config default shows unavailable; when enabled, login form shows during session check; `busy` state; detail dialog shows loading | Empty list renders table with no rows or empty-state text | Login/list/detail/mutation errors shown; partial bulk failure reported | Disabled deployment shows unavailable page; sign-in disables empty token | Session cookie and CSRF required; no dedicated offline screen | Confirm/cancel for delete, bulk delete, cleanup; Load more | Browser-only admin SPA (`AdminPage.tsx`, `adminApi.ts`, `config.tsx`) |

## System interfaces

| UI action | API/IPC/command/event | Payload/result | Failure behavior | Source/tests |
|---|---|---|---|---|
| Load runtime policy | `GET /api/v1/config` | Mode, admin enabled, optional retention/challenge; no secrets | Provider keeps private defaults and sets error on failure | `web/src/app/config.tsx`, `internal/httpapi/httpapi.go` |
| Create Text | `POST /api/v1/shares` JSON; optional `X-uPaste-Challenge` | Standard `{text:{format,content}}` or Encrypted `{encrypted_text:{protocol,nonce,ciphertext}}`, `expires_at`; returns Share and owner token | API error, rate wait, or client encryption error shown inline | `web/src/app/api.ts`, `TextCreateForm.tsx`, `internal/httpapi/httpapi.go` |
| Create File | `POST /api/v1/shares` multipart `metadata` + `file`; optional challenge header | One Standard File and expiration; returns Share and owner token | XHR/network/server error shown inline; upload progress observed | `web/src/app/api.ts`, `FileCreateForm.tsx`, `web/e2e/file-management.spec.ts` |
| Read Share | `GET /api/v1/shares/:id` | Metadata plus Standard Text, encrypted envelope, or File metadata/download URL | 404/410/429/network/server mapping in viewer | `web/src/app/api.ts`, `ShareRoute.tsx`, `internal/httpapi/httpapi.go` |
| Open Raw / download | `GET /raw/:id` on app origin; `GET`/`HEAD /f/:id` on File origin | Inert text; attachment bytes with filename, size, ETag | Raw unavailable for encrypted, File 404/410/429 possible | `internal/httpapi/httpapi.go`, `internal/fileapi/fileapi.go`, `web/e2e/file-management.spec.ts` |
| Save / delete as owner | `PATCH` or `DELETE /api/v1/shares/:id` with bearer token | Changed Text/encrypted envelope and/or `expires_at`; delete returns 204 | 401 forgets token, 404/410 terminal, other errors retain draft | `web/src/app/api.ts`, `ManageRoute.tsx`, `web/e2e/{management,encrypted-management,file-management}.spec.ts` |
| Admin sign in/out | `POST`/`GET`/`DELETE /api/v1/admin/session` | Token login; session/CSRF response; HttpOnly cookie | Invalid token/rate limit; expired session appears signed out on reload | `web/src/app/adminApi.ts`, `internal/adminapi/adminapi.go` |
| Admin inspect/mutate | `GET /summary`, `GET /shares`, `GET`/`DELETE /shares/:id`, `POST /shares/bulk-delete`, `POST /cleanup/expired` under `/api/v1/admin` | Counts, metadata list, on-demand detail, deleted/failed IDs, purged count | Inline errors; partial bulk result retains failed visible IDs | `web/src/app/adminApi.ts`, `AdminPage.tsx`, `internal/adminapi/adminapi.go` |
| Theme | React state, `localStorage['upaste.theme']`, `prefers-color-scheme` media query | System/light/dark; sets `html[data-theme]` | Storage failures are ignored and selection still works in memory | `web/src/app/theme.tsx`, `web/src/components/AppHeader.tsx` |

## Interaction and accessibility behavior

- Keyboard/focus: Text/File tabs support Left/Right arrows, Home, End, and
  roving `tabIndex`. Format and Privacy are button-based radio groups with click
  handlers; no arrow-key handler is present. The management Text editor handles
  Ctrl/Cmd+Enter. Dialogs use a portal, `aria-modal`, focus containment, Escape,
  background `inert`/`aria-hidden`, and focus restoration. Evidence:
  `web/src/components/{Tabs,Dialog}.tsx`, `TextCreateForm.tsx`, `ManageRoute.tsx`,
  `web/src/components/Dialog.test.tsx`.
- Draft navigation: Text and File create forms stay mounted while tabs switch.
  A nonempty text draft or selected file triggers `beforeunload` protection and,
  in the current uncommitted working tree, a React Router leave dialog for
  internal navigation/Back. Keep editing preserves both drafts and the selected
  tab. Manage likewise blocks leaving when content/expiry is dirty. Evidence:
  `web/src/features/create/CreatePage.tsx`, `CreatePage.test.tsx`,
  `web/e2e/management.spec.ts`, `ManageRoute.tsx`.
- Pointer/touch/drag/drop: File creation accepts a file picker or drag/drop;
  when multiple files are dropped, it selects the first and shows a notice.
  Source lines may scroll horizontally; Markdown tables and code blocks are
  contained. A coarse-pointer CSS rule raises button/selector minimum height
  to 40px. Evidence: `FileCreateForm.tsx`, viewers, `web/src/style.css`.
- Markdown reading: Rendered GFM is sanitized, raw HTML is skipped, images
  become labeled links/text instead of loading inline, unsafe links lose their
  destination, and task-list checkboxes are disabled. Source view exposes the
  original text. Evidence: `web/src/features/share/MarkdownViewer.tsx`,
  `MarkdownViewer.test.tsx`.
- Announcements and labels: Text byte counter and creation result use polite
  live regions; File upload, challenge status, admin notice, and errors expose
  status/alert roles. File metadata uses a definition list. Evidence: feature
  components above and `web/src/features/challenge/ChallengeGate.tsx`.
- Reduced motion/high contrast/localization: CSS honors
  `prefers-reduced-motion`; theme supports system/light/dark. No explicit
  `forced-colors` rules or localization mechanism was found in `web/src`;
  user-visible strings are English. Actual high-contrast behavior is unknown.

## Platform, viewport, and runtime constraints

- Web SPA built with React/TypeScript; production assets are embedded in one Go
  binary. No desktop/mobile native shell is present. The app and attachment-only
  File listeners are distinct; defaults are loopback `127.0.0.1:8080` and
  `127.0.0.1:8081`. Evidence: `web/package.json`, `cmd/upaste/main.go`,
  `internal/webapp/webapp.go`, `internal/config/config.go`.
- Source CSS has a 320px minimum page width, a 1140px main content maximum,
  layout changes at 768px and 480px, and coarse-pointer/reduced-motion rules.
  The existing Playwright visual suite names 360, 390, 768, 1024, and 1440px in
  both themes; the suite was not rerun in this audit. Evidence:
  `web/src/style.css`, `web/e2e/visual-qa.spec.ts`.
- Public deployments require a configured challenge and finite retention;
  private deployment is the default. Public UI supports Cap or Cloudflare
  Turnstile. Expiration and challenge limits come from `/api/v1/config`.
  Evidence: `internal/config/config.go`, `web/src/app/config.tsx`,
  `web/src/features/challenge/ChallengeGate.tsx`.

## Content authority currently evidenced

- Domain terminology: Share, Text, File, Standard, Encrypted, Plain text,
  Source, Markdown, Expires, Management token, Superadmin. The code owns current
  functional values; current English UI wording is not an approved redesign
  copy contract. Evidence: `internal/domain/domain.go`, `web/src/app/types.ts`,
  feature components.
- Existing state/error semantics: expired, not found, missing/wrong decryption
  key, rate limit, network/server failure, invalid expiration/file size, pending
  upload/save/delete, and permission failure. Evidence: `ShareErrorState.tsx`,
  create forms, `ManageRoute.tsx`, `AdminPage.tsx`.
- Explicitly approved marketing/content copy: none evidenced.

## Known unknowns

- No fresh runtime, accessibility-tree, browser, touch-device, zoom, or
  high-contrast check was performed for this inventory. Tests listed above are
  existing coverage, not a claim that they passed in this audit.
- The three create-page draft guard edits are uncommitted at audit time;
  committed CI results for an earlier SHA do not validate this working tree.
- Deployment mode, challenge provider, admin availability, real content, and
  network conditions on any target installation are unknown.
- Focus destination after creation success and behavior on browsers without
  required Web Crypto features have not been verified here; no explicit focus
  move or compatibility fallback is present in the inspected components.
