# Frontend interaction specification

This document specifies the exact interaction contracts, state transitions, secret handling rules, and behavioral details for the uPaste frontend.

---

## 1. Comprehensive state matrix

The following matrix formally defines the 21 mandatory application states specified in Phase 6:

| # | State Name | Input Data | Visible Actions | Secret Material Present | Server Operations | Navigation Behavior | Error Behavior |
|---|---|---|---|---|---|---|---|
| 1 | **Create Text Standard** | Content (1–1,048,576 B UTF-8), format (`PLAIN`, `SOURCE`, `MARKDOWN`), expiration (nullable RFC3339) | Tab switch, format select, privacy select, expiration select, `Create share` | None | `POST /api/v1/shares` with JSON payload | Stays on `/`; transitions to State 4 on success | 400/413/429 displayed inline; draft preserved |
| 2 | **Create Text Encrypted** | Plaintext (1–1,048,576 B), format, expiration | Tab switch, format select, privacy select, expiration select, `Create share` | Plaintext in memory; ephemeral 32-byte AES key generated via Web Crypto | `POST /api/v1/shares` with JSON containing ciphertext and nonce | Stays on `/`; transitions to State 4 with complete fragment URL | Web Crypto error or 400/413/429 shown inline; draft preserved |
| 3 | **Create File** | Single file (binary, ≤ 64 MiB), expiration | Drag-and-drop zone, `Choose file`, `Remove`, expiration select, `Create share` | None | `POST /api/v1/shares` via multipart/form-data | Stays on `/`; transitions to State 4 on success | File size > 64 MiB rejected locally; upload failure shown inline |
| 4 | **Creation Success** | Created Share ID, optional `#up_e1_` key, `OwnerToken` | `Copy link`, `Reveal token`, `Copy token`, `Open share`, `Manage share`, `New share` | `OwnerToken` in volatile memory; `#up_e1_` in link if encrypted | None | `Open share` -> `/s/:id`; `Manage share` -> `/manage/:id`; `New share` -> reset `/` | Clipboard failure falls back to manual text selection |
| 5 | **View Standard Plain** | Share ID | `New share`, `Copy content`, `Raw` | None | `GET /api/v1/shares/:id` | Stays on `/s/:id` | 404/410 transition to respective error states |
| 6 | **View Standard Source** | Share ID | `New share`, `Copy content`, `Raw`, line wrapping toggle | None | `GET /api/v1/shares/:id` | Stays on `/s/:id` | 404/410 transition to respective error states |
| 7 | **View Standard Markdown** | Share ID | `New share`, `Copy content`, `Raw`, `Rendered / Source` toggle | None | `GET /api/v1/shares/:id` | Stays on `/s/:id` | Sanitizer catches unsafe elements; 404/410 handled |
| 8 | **View Encrypted Plain** | Share ID + `#up_e1_<key>` in URL fragment | `New share`, `Copy content` (`Raw` strictly hidden) | Decryption key in URL fragment and memory; decrypted plaintext in memory | `GET /api/v1/shares/:id` (server receives only metadata and ciphertext) | Stays on `/s/:id#up_e1_...` | Decryption failure transitions to State 11 |
| 9 | **View Encrypted Source** | Share ID + `#up_e1_<key>` in URL fragment | `New share`, `Copy content`, line wrapping toggle | Decryption key in URL fragment; decrypted plaintext in memory | `GET /api/v1/shares/:id` | Stays on `/s/:id#up_e1_...` | Decryption failure transitions to State 11 |
| 10 | **View Encrypted Markdown** | Share ID + `#up_e1_<key>` in URL fragment | `New share`, `Copy content`, `Rendered / Source` toggle | Decryption key in URL fragment; decrypted plaintext in memory | `GET /api/v1/shares/:id` | Stays on `/s/:id#up_e1_...` | Decryption failure transitions to State 11 |
| 11 | **View Encrypted without key** | Share ID (no URL fragment) | `New share` | None | `GET /api/v1/shares/:id` | Stays on `/s/:id` | Explanatory message: "Decryption key missing" |
| 12 | **View Encrypted wrong key** | Share ID + invalid `#up_e1_<key>` or tampered ciphertext | `New share` | Invalid key candidate in memory | `GET /api/v1/shares/:id` | Stays on `/s/:id#up_e1_...` | Explanatory message: "Unable to decrypt this share" |
| 13 | **View File** | Share ID | `New share`, `Download file` | None | `GET /api/v1/shares/:id` | Stays on `/s/:id`; download triggers navigation to `download_url` | 404/410 handled; download network error handled |
| 14 | **Manage Standard** | Share ID + `OwnerToken` | Format selector, expiration selector, editor, `Save changes`, `Delete share`, `Cancel` | `OwnerToken` in volatile memory only | `PATCH /api/v1/shares/:id`, `DELETE /api/v1/shares/:id` | Stays on `/manage/:id`; delete navigates to State 21 | 401 invalid token prompt; 410 expired error; 429 rate limit |
| 15 | **Manage Encrypted with key** | Share ID + `OwnerToken` + `#up_e1_<key>` | Format selector, expiration selector, editor, `Save changes`, `Delete share`, `Cancel` | `OwnerToken` in volatile memory; decryption key in URL fragment and memory | `PATCH /api/v1/shares/:id` (re-encrypts with same key + fresh nonce) | Stays on `/manage/:id#up_e1_...`; delete navigates to State 21 | Decryption error or 401/429 handled |
| 16 | **Manage Encrypted without key** | Share ID + `OwnerToken` (no key fragment) | Expiration selector, `Update expiration`, `Delete share`, `Cancel` (content editor disabled) | `OwnerToken` in volatile memory | `PATCH /api/v1/shares/:id` (expiration only), `DELETE /api/v1/shares/:id` | Stays on `/manage/:id`; delete navigates to State 21 | 401/410/429 handled |
| 17 | **Manage File** | Share ID + `OwnerToken` | Expiration selector, `Update expiration`, `Delete share`, `Cancel` (no file replace) | `OwnerToken` in volatile memory | `PATCH /api/v1/shares/:id` (expiration only), `DELETE /api/v1/shares/:id` | Stays on `/manage/:id`; delete navigates to State 21 | 401/410/429 handled |
| 18 | **Not found (404)** | Share ID | `Create new share` | None | None (result of 404 response) | Generic 404 view; clicking action routes to `/` | None |
| 19 | **Expired (410)** | Share ID | `Create new share` | None | None (result of 410 response) | Generic 410 view; clicking action routes to `/` | None |
| 20 | **Rate limited (429)** | Any active operation | `Retry` (after delay), dismiss | In-flight request data retained in memory | None until retry period expires | Retains current route and draft state | Countdown timer based on `Retry-After` header |
| 21 | **Deleted** | Share ID (recently deleted) | `Create new share` | None (token and content cleared) | None (result of successful 204 DELETE) | Stays on post-deletion confirmation screen; action routes to `/` | None |

---

## 2. Secret lifecycle and zero-knowledge interaction flow

### 2.1 OwnerToken handling
1. **Generation**: Generated on the backend during creation using 32 random bytes (`up_o1_<43-char-base64url>`).
2. **Delivery**: Returned once in the JSON response of `POST /api/v1/shares`.
3. **Display**: Displayed on the creation confirmation screen masked by default (`up_o1_••••••••••••••••••••••••••••••••••••••`). The user can click `Reveal` to unmask or `Copy` to copy it to clipboard.
4. **Volatile in-memory transport**:
   - When the user clicks `Manage share` immediately after creation, `OwnerToken` is passed via React router state (`history.state`).
   - If the user reloads `/manage/:id` or opens it in a new tab, `OwnerToken` is absent from memory, prompting the token input modal.
5. **Strict non-persistence rule**:
   - `OwnerToken` is **never** written to `localStorage`, `sessionStorage`, `document.cookie`, or `IndexedDB`.
   - `OwnerToken` is **never** placed in the URL query string, pathname, or fragment.

### 2.2 Encrypted Text decryption key handling
1. **Generation**: Generated client-side using `window.crypto.getRandomValues(new Uint8Array(32))`.
2. **Zero-transmission**: The raw key is never sent to the backend. The API request contains only the ciphertext, the 12-byte random nonce, and metadata.
3. **URL fragment channel**:
   - The key is represented as `#up_e1_<43-char-base64url>`.
   - The browser URL fragment is never sent in HTTP request headers.
   - When navigating from creation success to `/s/:id#up_e1_...` or `/manage/:id#up_e1_...`, the fragment is strictly preserved.
4. **Re-encryption on update**:
   - When updating an encrypted Share on `/manage/:id#up_e1_...`, the application reuses the existing AES-256-GCM key from the fragment.
   - A fresh, independent 12-byte nonce is generated via `crypto.getRandomValues`.
   - The new ciphertext and fresh nonce are submitted via `PATCH /api/v1/shares/:id`.
   - The shared URL fragment remains identical, so existing recipients can decrypt the updated content without receiving a new link.

---

## 3. Ephemeral clipboard feedback

- **Interaction**:
  - Clicking any `Copy` button (Share link, OwnerToken, or Content) triggers `navigator.clipboard.writeText(...)`.
  - Upon promise resolution, the button label temporarily changes to `Copied` (with an optional checkmark icon) for **1,500 ms**.
  - After 1,500 ms, the label smoothly reverts to `Copy`.
- **No toast notification stack**:
  - Global floating toast containers, banners, and snackbars are explicitly forbidden for clipboard actions. All feedback must remain localized and inline within the button itself.
- **Fallback for denied clipboard permissions**:
  - If `navigator.clipboard.writeText()` rejects (e.g. non-HTTPS, browser permission denied), the UI must automatically select the text inside the input or show a small inline tooltip: `Press Ctrl+C to copy`.

---

## 4. File drag-and-drop interaction

1. **Drop target state**:
   - `dragenter` / `dragover`: Container border transitions to accent color, background subtly highlights. Default browser file opening behavior is prevented via `e.preventDefault()`.
   - `dragleave` / `drop`: Visual highlight clears.
2. **File validation**:
   - The drop handler inspects `e.dataTransfer.files`:
     - If multiple files are dropped, only the first file is accepted, and an inline message notes: *Only one file per share is supported.*
     - If the file exceeds 64 MiB (67,108,864 bytes), the file is rejected immediately, and an inline error is displayed: *File exceeds maximum limit of 64 MiB.*
3. **File removal**:
   - When a file is selected, the drop zone displays filename, formatted size, and a `Remove` button.
   - Clicking `Remove` resets the file input and restores the empty drop zone.
4. **Keyboard accessibility**:
   - The drop zone includes a native `<input type="file" id="file-input" hidden>` coupled with a focusable `<label for="file-input">` button styled as `Choose file`. Pressing `Enter` or `Space` on the button opens the OS file picker dialog.

---

## 5. Dirty state and unsaved draft protection

1. **Detection**:
   - An editor session is marked dirty when current text differs from initial text (or when a file has been selected).
2. **Internal route change**:
   - If the user attempts to navigate away (e.g. clicking `New share` or the header brand) while dirty, a confirmation dialog appears:
     > *You have unsaved changes. Are you sure you want to discard your draft?*
     > `[Discard and leave]` `[Keep editing]`
3. **External browser navigation / reload**:
   - Hook `window.addEventListener("beforeunload", handler)` when dirty.
   - Call `e.preventDefault()` to trigger the standard browser exit confirmation.
4. **Drafts not persisted**:
   - In accordance with the security model, unsaved drafts are never saved to `localStorage` or `sessionStorage`.

---

## 6. Focus management and dialog accessibility

1. **Modal dialog behavior**:
   - When the Delete confirmation dialog or Token entry modal opens:
     - Focus moves immediately to the primary interactive element (e.g. `Cancel` button or token input).
     - Tab key navigation is trapped within the dialog container (`focus-trap`).
     - Content outside the dialog is marked `aria-hidden="true"` or `inert`.
2. **Escape key dismissal**:
   - Pressing `Escape` closes non-destructive dialogs (e.g. Token entry or Discard draft) and restores focus to the triggering button.
3. **Keyboard shortcuts**:
   - `Ctrl+Enter` (Windows/Linux) or `Cmd+Enter` (macOS) inside the text editor triggers `Create share` or `Save changes`.
4. **Live announcements**:
   - Asynchronous status changes (e.g. `Creating…`, `Copied`, `Decryption failed`) utilize an `aria-live="polite"` container so screen readers announce state transitions without interrupting user focus.

---

## 7. Expiration control interaction

- **Preset selector**:
  - Standard select or segmented dropdown displaying: `Never`, `1 hour`, `1 day`, `7 days`, `30 days`, `Custom…`.
- **Custom date/time selection**:
  - Selecting `Custom…` reveals a compact date-time picker input (`<input type="datetime-local">`).
  - Validation rules:
    - Must be a valid date in the future (minimum 1 minute ahead).
    - Selecting a past time shows an immediate inline validation error: *Expiration date must be in the future.*
    - On form submission, the local timestamp is parsed and formatted as an RFC3339 UTC string (e.g. `2026-09-21T12:00:00Z`).

---

## 8. Markdown Source / Rendered view switching

- For shares with `text_format: "MARKDOWN"`:
  - The viewer toolbar includes a segmented switch: `[Rendered | Source]`.
  - Default view is `Rendered`.
  - Switching to `Source` renders raw Markdown text in monospace with line preservation (`white-space: pre-wrap`).
  - Switching view modes is purely local client-side state and does not trigger server requests.
