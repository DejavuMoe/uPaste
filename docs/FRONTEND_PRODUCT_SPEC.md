# Frontend product contract and UI/UX specification

Phase 6 freezes the product model, information architecture, screen contracts, wireframes, API mappings, and error handling for the upcoming Phase 7 production frontend. It changes no backend code, database schemas, or browser runtime code.

uPaste is a private, self-hosted sharing utility: the homepage is the functional tool itself, not a SaaS landing page or marketing showcase.

---

## 1. Product positioning and design philosophy

1. **Utility first**: uPaste is an editor and sharing utility. It has no marketing hero, feature cards, fake statistics, or promotional copy.
2. **Immediate utility**: Navigating to `/` presents the creation workspace immediately. Users can begin typing or drop a file without unnecessary interaction.
3. **Single content column**: Both creation and viewing interfaces focus on a primary content column sized for optimal reading and editing (approximately 1080–1180 px on desktop).
4. **No persistent chrome**: There is no navigation sidebar, breadcrumb hierarchy, or complex dashboard. Product navigation is minimal:
   - Header: `uPaste` brand at left, theme selector (Light / Dark / System) at right.
   - Contextual action: `New share` button in header when viewing or managing a Share.
   - Footer: Minimal or absent. Never displays commit hashes, protocol details, or framework marketing.

---

## 2. Information architecture and route table

### Frozen product routes

| Route | Purpose | Access model |
|---|---|---|
| `/` | Create Share (Text or File) | Public / Anonymous |
| `/s/:id` | Public Share viewer (Text, Encrypted, or File) | Public / Anonymous |
| `/manage/:id` | Share owner management (Update / Delete) | Capability-authorized (`OwnerToken`) |

### Backend and separate origin routes

| Route | Origin | Purpose |
|---|---|---|
| `/api/v1/*` | Application origin | REST API (JSON / multipart) |
| `/raw/:id` | Application origin | Inert plain-text download (Standard Text only) |
| `/f/:id` | File listener origin | Direct attachment file download |

### Omitted routes in V1

The following routes are explicitly forbidden and must not be implemented:
- `/history` (no local or server-side share history)
- `/feed` (no public listings)
- `/search` (no content indexing or discovery)
- `/account` / `/login` / `/register` (no user identity or authentication)
- `/dashboard` (no administration panels). Phase 11 later added one bounded Superadmin governance surface at `/admin`; it exists only when `UPASTE_ADMIN_TOKEN` is configured and is documented in [API.md](API.md), not in this frozen Phase 6 contract.

---

## 3. Browser titles policy

Browser document titles (`<title>`) must remain informative and restrained without leaking confidential metadata:

| Screen | Browser title |
|---|---|
| Create share | `New share · uPaste` |
| Text viewer (Standard or Encrypted) | `Share · uPaste` |
| File viewer | `<sanitized_filename> · uPaste` (e.g. `report.pdf · uPaste`) |
| Management view | `Manage share · uPaste` |
| Error views (404, 410, 429) | `uPaste` |

**Security rule**: The document title must **never** contain:
- Text content snippets or user drafts
- Owner tokens (`up_o1_...`)
- Decryption keys or fragment contents (`#up_e1_...`)

---

## 4. Creation specifications

The root route `/` presents a tabbed creation interface defaulting to the **Text** tab.

### 4.1 Text create workspace

```text
┌──────────────────────────────────────────────────────────────┐
│ New share                                                    │
│                                                              │
│ Text     File                                                │
│ ────                                                         │
│ Format         Privacy                 Expires               │
│ [Plain text ▾] [Standard  Encrypted]   [Never             ▾] │
│                                                              │
│ ┌──────────────────────────────────────────────────────────┐ │
│ │                                                          │ │
│ │                   text editor area                       │ │
│ │                                                          │ │
│ └──────────────────────────────────────────────────────────┘ │
│                                                              │
│ 2.4 KB / 1 MiB                                  Create share │
└──────────────────────────────────────────────────────────────┘
```

#### Field specifications:

1. **Format selector**:
   - Options: `Plain text`, `Source`, `Markdown`.
   - Exact API mapping: `PLAIN`, `SOURCE`, `MARKDOWN`.
   - No language dropdown, no rich text, no WYSIWYG, and no executable HTML mode.
2. **Privacy selector**:
   - Options: `Standard` (default) vs `Encrypted`.
   - Clear segmented control.
   - When `Encrypted` is active, display a single concise explanatory note:
     > *Encrypted in this browser. Anyone with the complete link can read it. The server cannot recover a lost decryption key.*
3. **Expiration selector**:
   - Presets: `Never` (default, `expires_at: null`), `1 hour`, `1 day`, `7 days`, `30 days`, `Custom…`.
   - Selecting `Custom…` opens a compact local date/time picker.
   - Values are converted to normalized RFC3339 UTC timestamps on submission.
4. **Editor behavior and browser text semantics**:
   - The frontend does not trim or otherwise intentionally transform the textarea API value.
   - Browser text controls normalize line endings according to HTML textarea semantics. In normal modern browser textarea editing this means line endings are represented as LF (`\n`) in the API value. The frontend does not promise preservation of an originally supplied CRLF representation after that content is edited through the browser.
   - Server raw endpoint (`/raw/:id`) continues to return stored Standard Text bytes exactly; the browser editor operates on browser textarea string semantics.
   - Whitespace remains significant: the frontend does not trim leading or trailing whitespace, permits whitespace-only content, and preserves tabs and spaces present in the textarea API value. Zero-byte content remains invalid. No automatic formatting, smart quotes, or indentation rewriting.
   - Byte counting: calculated as `new TextEncoder().encode(value).byteLength` (or equivalent UTF-8 byte-length operation) against the strict 1 MiB (1,048,576 UTF-8 bytes) limit. JavaScript `value.length` must NOT be used for byte-limit calculation.
   - Displays real-time byte count against the 1 MiB limit (e.g. `2.4 KB / 1 MiB` or `1,048,000 / 1,048,576 B`).
   - Warns visually when byte count exceeds 90% (943,718 bytes) of the limit.
   - Disables submission if editor is empty (0 bytes) or exceeds 1,048,576 bytes.
   - Typography: System monospace font for `SOURCE`; clean system UI font for `PLAIN` and `MARKDOWN`.
   - Visible keyboard focus ring on `:focus-visible`.
5. **Submission action**:
   - Button labeled `Create share`.
   - Transitions to `Creating…` and disables input during active HTTP request.
   - Duplicate clicks are ignored while in flight.

### 4.2 File create workspace

```text
┌──────────────────────────────────────────────────────────────┐
│ New share                                                    │
│                                                              │
│ Text     File                                                │
│          ────                                                │
│ ┌──────────────────────────────────────────────────────────┐ │
│ │                                                          │ │
│ │                  Drop one file here                      │ │
│ │                          or                              │ │
│ │                     [Choose file]                        │ │
│ │                                                          │ │
│ │                  Maximum size: 64 MiB                    │ │
│ └──────────────────────────────────────────────────────────┘ │
│                                                              │
│ Expires                                                      │
│ [Never             ▾]                           Create share │
└──────────────────────────────────────────────────────────────┘
```

#### Specifications:
1. **Scope and limits**:
   - Exactly one file per Share.
   - Privacy mode is strictly `STANDARD` (encrypted file is not supported in V1).
   - Maximum file size is strictly 64 MiB (67,108,864 bytes). Files exceeding this are rejected immediately on selection.
2. **File selection**:
   - Drag-and-drop target with keyboard-accessible `Choose file` native `<input type="file">` button.
   - Upon selection, the drop zone collapses to a compact file row:
     ```text
     ┌────────────────────────────────────────────────────────┐
     │ 📄 report.pdf        8.4 MiB                   [Remove]│
     └────────────────────────────────────────────────────────┘
     ```
   - Clicking `Remove` clears the selection and restores the drop zone.
   - No inline thumbnail or browser preview is generated.
3. **Upload progress states**:
   - On submission, transitions to `Preparing…` then `Uploading…`.
   - Displays real browser upload progress if available via the underlying transport. Fake progress or estimated percentages are forbidden.

### 4.3 Creation success state

Upon successful creation, the application remains on `/` and renders an explicit confirmation view rather than immediately redirecting. This guarantees the user has the opportunity to copy the management token:

```text
┌──────────────────────────────────────────────────────────────┐
│ Share created                                                │
│                                                              │
│ Share link                                                   │
│ ┌──────────────────────────────────────────────┬───────────┐ │
│ │ https://upaste.example/s/2bF9a...            │   [Copy]  │ │
│ └──────────────────────────────────────────────┴───────────┘ │
│                                                              │
│ Management token                                             │
│ ┌──────────────────────────────────────────────┬───────────┐ │
│ │ up_o1_•••••••••••••••••••••••••••••••••••••• │[Reveal][Copy│
│ └──────────────────────────────────────────────┴───────────┘ │
│ ⚠️ Save this token now. It is shown once and is required to  │
│    modify or delete this share. uPaste does not store it.    │
│                                                              │
│ [Open share]          [Manage share]             [New share] │
└──────────────────────────────────────────────────────────────┘
```

#### Behavior rules:
- **Share link**:
  - Standard Text / File: `https://<domain>/s/<id>`
  - Encrypted Text: `https://<domain>/s/<id>#up_e1_<base64url_key>`
  - Copying an encrypted share link copies the complete URL including the `#up_e1_` fragment.
- **Management token**:
  - Masked by default (`up_o1_••••`).
  - `Reveal` toggles plaintext visibility.
  - `Copy` copies `up_o1_<token>` to clipboard with inline `Copied` feedback.
  - Never concatenated with the decryption key.
- **Navigation actions**:
  - `Open share`: Navigates to `/s/:id` (preserving fragment for encrypted shares).
  - `Manage share`: Navigates to `/manage/:id` using top-level application in-memory React state / Context or a purpose-built in-memory capability store. `OwnerToken` MUST NOT be stored in or transported through `history.state`, React Router `location.state`, URL state, browser storage, cookies, or any browser-persisted navigation mechanism.
  - `New share`: Resets creation form to blank state.

---

## 5. Viewer specifications

Route: `/s/:id`

The viewer loads metadata from `GET /api/v1/shares/:id`.

### 5.1 Standard Text viewer

```text
┌──────────────────────────────────────────────────────────────┐
│ uPaste                                             New share │
│                                                              │
│ Plain text · 12.4 KB · Expires in 2 days        [Copy] [Raw] │
│ ──────────────────────────────────────────────────────────── │
│ 1 │ package main                                             │
│ 2 │                                                          │
│ 3 │ func main() {                                            │
│ 4 │     println("Hello, uPaste")                             │
│ 5 │ }                                                        │
└──────────────────────────────────────────────────────────────┘
```

- **Metadata row**: Displays format badge (`Plain text`, `Source`, `Markdown`), payload size in human-readable units, and relative/absolute expiration time.
- **Actions**:
  - `Copy`: Copies raw text content to system clipboard with transient inline feedback.
  - `Raw`: Direct hyperlink to `/raw/:id` (opens in new tab or direct navigation).
- **Format presentation**:
  - `PLAIN`: Clean serif/sans typography, whitespace-preserving (`white-space: pre-wrap`).
  - `SOURCE`: Monospace font (`white-space: pre` or toggleable `pre-wrap`), optional line numbers, horizontal scroll inside content container.
  - `MARKDOWN`: Dual-mode toggle (`Rendered` vs `Source`).

### 5.2 Markdown viewer security policy

Phase 7 implements client-side Markdown rendering adhering to the following strict invariants:
1. **Client-side only**: Parsing and rendering take place entirely within the viewer browser.
2. **Raw HTML disabled**: HTML tags within Markdown source are escaped and never rendered as live DOM elements.
3. **Independent sanitization**: Output is sanitized with an audited HTML sanitizer before insertion into the DOM.
4. **Active content forbidden**: `<script>`, `<style>`, `<iframe>`, `<object>`, `<embed>`, `<form>`, `<svg>`, and `<base>` elements are unconditionally stripped.
5. **Executable URLs blocked**: Links with `javascript:`, `data:`, `vbscript:`, or custom URL schemes are forbidden. Only `http:`, `https:`, and `mailto:` are permitted.
6. **No automatic external media**: External image embedding (`![]()`) is disabled by default to prevent viewer IP address and user-agent leakage to third parties. Images may render as safe outbound links.
7. **Safe outbound links**: All generated links must have `target="_blank" rel="noopener noreferrer nofollow"`.
8. **View mode switch**: Always provide a toggle between `Rendered` view and raw `Source` view.

### 5.3 Encrypted Text viewer

Encrypted Text shares are identified by `privacy_mode: "ENCRYPTED"`.

- **Decryption sequence**:
  1. Fetch encrypted metadata, nonce, and ciphertext from `GET /api/v1/shares/:id`.
  2. Read decryption key from browser URL fragment (`window.location.hash`).
  3. Decrypt using Web Crypto `UPASTE_AES_GCM_V1` helper with AAD `uPaste:encrypted-text:v1`.
  4. Parse decrypted binary envelope: verify byte 0 is `0x01`, extract format (`0x01` PLAIN, `0x02` SOURCE, `0x03` MARKDOWN), and decode remaining bytes as UTF-8.
  5. Render using the standard text viewer component.
- **No Raw button**: The `Raw` button is strictly omitted for Encrypted shares.
- **Missing key state**:
  If the URL fragment `#up_e1_...` is absent, display:
  ```text
  ┌────────────────────────────────────────────────────────────┐
  │ 🔒 Decryption key missing                                  │
  │                                                            │
  │ This encrypted share cannot be read without the complete   │
  │ link containing the decryption key.                        │
  │                                                            │
  │                                                [New share] │
  └────────────────────────────────────────────────────────────┘
  ```
- **Invalid key / Tampered ciphertext state**:
  If Web Crypto decryption fails:
  ```text
  ┌────────────────────────────────────────────────────────────┐
  │ ⚠️ Unable to decrypt this share                             │
  │                                                            │
  │ The link may be incomplete, or the encrypted data may have │
  │ been modified.                                             │
  │                                                            │
  │                                                [New share] │
  └────────────────────────────────────────────────────────────┘
  ```
  *Rule*: Never display Web Crypto error stack traces or cryptographic primitives in the UI.

### 5.4 File viewer

When `payload_kind` is `FILE`:

```text
┌──────────────────────────────────────────────────────────────┐
│ uPaste                                             New share │
│                                                              │
│ 📄 financial-report-q3.pdf                                   │
│                                                              │
│ Size: 8.4 MiB                                                │
│ MIME type: application/pdf                                   │
│ Expires: in 5 days                                           │
│                                                              │
│ [Download file]                                              │
└──────────────────────────────────────────────────────────────┘
```

- Displays original sanitized filename, formatted byte size, detected MIME type, and expiration.
- **Download action**: A standard hyperlink pointing directly to `download_url` on the separate File listener origin (e.g. `http://127.0.0.1:8081/f/:id`).
- **Security constraints**:
  - No inline PDF, image, video, or audio preview.
  - No iframe embedding.
  - File bytes are never fetched into browser JavaScript memory via `fetch` or `XMLHttpRequest`.
  - Download is triggered as a standard browser attachment download.

---

## 6. Management specifications

Route: `/manage/:id`

Management allows the creator to edit content (Text only), adjust expiration, or permanently delete the Share using their `OwnerToken`.

### 6.1 Authentication and token entry

- If the user navigated directly from the creation success screen, `OwnerToken` is passed via top-level application in-memory React state / Context or an equally scoped non-persistent module memory capability store. It MUST NOT use `history.state` or React Router `location.state`.
- If the user reloads `/manage/:id`, closes the tab/window, opens `/manage/:id` in a new tab, or navigates directly, `OwnerToken` is absent from memory, prompting the token entry modal.
- A browser Back/Forward navigation MUST NOT recover `OwnerToken` from History API serialized state.
- The view presents a token entry prompt:
  ```text
  ┌────────────────────────────────────────────────────────────┐
  │ Management token required                                  │
  │                                                            │
  │ Enter the management token issued when this share was      │
  │ created.                                                   │
  │                                                            │
  │ [ up_o1_•••••••••••••••••••••••••••••••••••••• ][Reveal]   │
  │                                                            │
  │ [Continue]                                     [Cancel]    │
  └────────────────────────────────────────────────────────────┘
  ```
- Token input is masked by default with a `Reveal` toggle.
- Submitted token is retained **strictly in application process memory** (top-level state / context). It is never stored in or transported through `history.state`, React Router `location.state`, `localStorage`, `sessionStorage`, `cookies`, `IndexedDB`, `Cache Storage`, service workers, or URL parameters/fragments.
- Cleanup: On successful Share deletion, the associated `OwnerToken` is immediately cleared from application memory. Full page reload or tab close clears it naturally.

### 6.2 Capabilities matrix by payload kind

| Capability | Standard Text | Encrypted Text (with key) | Encrypted Text (no key) | File |
|---|---|---|---|---|
| Edit content | Allowed | Allowed (re-encrypts with same key + new nonce) | Forbidden (read-only) | Forbidden (no file replacement) |
| Change format | Allowed | Allowed | Forbidden | N/A |
| Change expiration | Allowed | Allowed | Allowed | Allowed |
| Delete share | Allowed | Allowed | Allowed | Allowed |

### 6.3 Dirty-state and expiration-only updates
- The frontend distinguishes between:
  - Content unchanged + expiration changed
  - Content actually edited
- When only expiration is modified, the frontend sends `PATCH /api/v1/shares/:id` with `expires_at` only. It must NOT re-send the text payload merely because the management view was opened. This avoids accidental newline normalization when content was not edited.
- For Encrypted Text, expiration-only updates send `expires_at` without re-encrypting or sending a new ciphertext and nonce.
- If existing fetched Standard Text contains CRLF, loading it into the editable textarea applies browser text-control newline normalization. If the user then modifies and saves that text, the frontend does not guarantee restoration of the original CRLF byte representation.

### 6.4 Encrypted management URL rules
- Encrypted shares may be managed via `/manage/:id#up_e1_<key>`. The URL fragment carries the decryption key across management views.
- Navigating between `/s/:id#up_e1_...` and `/manage/:id#up_e1_...` must preserve the URL fragment.
- The `OwnerToken` must **never** appear in the URL fragment or query parameters.

### 6.5 Delete interaction

Deletion is immediate and permanent:
1. User clicks `Delete share` (styled as a danger button).
2. A focused confirmation modal dialog appears:
   ```text
   ┌────────────────────────────────────────────────────────┐
   │ Delete this share?                                     │
   │                                                        │
   │ Anyone with the link will immediately lose access.     │
   │ This action cannot be undone.                          │
   │                                                        │
   │ [Cancel]                                [Delete share] │
   └────────────────────────────────────────────────────────┘
   ```
3. Confirming sends `DELETE /api/v1/shares/:id` with `Authorization: Bearer <owner_token>`.
4. Browser `window.confirm()` must not be used.
5. On success, transitions to a completed state:
   ```text
   ┌────────────────────────────────────────────────────────┐
   │ Share deleted                                          │
   │                                                        │
   │ This share has been permanently removed.               │
   │                                                        │
   │ [Create new share]                                     │
   └────────────────────────────────────────────────────────┘
   ```

### 6.6 Unsaved draft protection
If the user modifies content in the editor and attempts internal navigation, prompt the user with a non-blocking warning modal before discarding changes. Drafts are not persisted to browser storage.

---

## 7. Wireframes

### Wireframe 1: Text create desktop
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                             [Theme: System ▾]   │
│                                                                        │
│ New share                                                              │
│                                                                        │
│ [Text]  File                                                           │
│ ──────                                                                 │
│ Format              Privacy                    Expires                 │
│ [Plain text      ▾] [Standard  Encrypted]      [Never               ▾] │
│                                                                        │
│ ┌────────────────────────────────────────────────────────────────────┐ │
│ │1 | package main                                                    │ │
│ │2 |                                                                 │ │
│ │3 | func main() {                                                   │ │
│ │4 |     println("Hello world")                                      │ │
│ │5 | }                                                               │ │
│ │                                                                    │ │
│ └────────────────────────────────────────────────────────────────────┘ │
│                                                                        │
│ 78 B / 1 MiB                                              [Create share│
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 2: File create desktop
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                             [Theme: System ▾]   │
│                                                                        │
│ New share                                                              │
│                                                                        │
│ Text   [File]                                                          │
│        ──────                                                          │
│ ┌────────────────────────────────────────────────────────────────────┐ │
│ │                                                                    │ │
│ │                        Drop one file here                          │ │
│ │                                or                                  │ │
│ │                           [Choose file]                            │ │
│ │                                                                    │ │
│ │                       Maximum size: 64 MiB                         │ │
│ └────────────────────────────────────────────────────────────────────┘ │
│                                                                        │
│ Expires                                                                │
│ [Never               ▾]                                   [Create share│
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 3: Mobile create (390 px)
```text
┌─────────────────────────────────────┐
│ uPaste                      [Theme] │
│                                     │
│ New share                           │
│ [Text]  File                        │
│ ──────                              │
│ Format: [Plain text ▾]              │
│ Privacy: [Standard | Encrypted]     │
│ Expires: [Never ▾]                  │
│                                     │
│ ┌─────────────────────────────────┐ │
│ │text editor content...           │ │
│ │                                 │ │
│ │                                 │ │
│ └─────────────────────────────────┘ │
│ 24 B / 1 MiB                        │
│                                     │
│ [Create share                     ] │
└─────────────────────────────────────┘
```

### Wireframe 4: Creation success
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                             [Theme: System ▾]   │
│                                                                        │
│ Share created                                                          │
│                                                                        │
│ Share link                                                             │
│ ┌───────────────────────────────────────────────────────┬────────────┐ │
│ │ https://upaste.example/s/2bF9a0Z...                   │   [Copy]   │ │
│ └───────────────────────────────────────────────────────┴────────────┘ │
│                                                                        │
│ Management token                                                      │
│ ┌───────────────────────────────────────────────────────┬────────────┐ │
│ │ up_o1_••••••••••••••••••••••••••••••••••••••••••••••• │[Rev] [Copy]│ │
│ └───────────────────────────────────────────────────────┴────────────┘ │
│ ⚠️ This token is required to edit or delete this share. Save it now.   │
│                                                                        │
│ [Open share]                 [Manage share]                [New share] │
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 5: Standard Text viewer
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                                       New share │
│                                                                        │
│ Source · 4.2 KB · Expires in 6 days                       [Copy] [Raw] │
│ ────────────────────────────────────────────────────────────────────── │
│ 1 │ import "fmt"                                                       │
│ 2 │                                                                    │
│ 3 │ func main() { fmt.Println("uPaste") }                              │
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 6: Encrypted viewer missing key
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                                       New share │
│                                                                        │
│ 🔒 Decryption key missing                                              │
│                                                                        │
│ This encrypted share cannot be read without the complete link          │
│ containing the decryption key.                                         │
│                                                                        │
│                                                            [New share] │
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 7: File viewer
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                                       New share │
│                                                                        │
│ 📄 quarterly-results.pdf                                               │
│                                                                        │
│ Size: 14.2 MiB                                                         │
│ MIME: application/pdf                                                  │
│ Expires: Never                                                         │
│                                                                        │
│ [Download file                                                       ] │
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 8: Management token entry
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                                       New share │
│                                                                        │
│ Management token required                                              │
│                                                                        │
│ This credential is required to modify or delete this share.            │
│                                                                        │
│ ┌───────────────────────────────────────────────────────┬────────────┐ │
│ │ up_o1_••••••••••••••••••••••••••••••••••••••••••••••• │  [Reveal]  │ │
│ └───────────────────────────────────────────────────────┴────────────┘ │
│                                                                        │
│ [Continue]                                                    [Cancel] │
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 9: Standard Text management
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                                       New share │
│                                                                        │
│ Manage share: 2bF9a0Z...                                               │
│                                                                        │
│ Format: [Plain text ▾]        Expires: [7 days ▾]                      │
│                                                                        │
│ ┌────────────────────────────────────────────────────────────────────┐ │
│ │editable text content...                                            │ │
│ └────────────────────────────────────────────────────────────────────┘ │
│                                                                        │
│ [Save changes]                [Delete share]                  [Cancel] │
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 10: File management
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                                       New share │
│                                                                        │
│ Manage file: quarterly-results.pdf                                     │
│ Size: 14.2 MiB (File content cannot be modified)                       │
│                                                                        │
│ Expires: [30 days ▾]                                                   │
│                                                                        │
│ [Update expiration]           [Delete share]                  [Cancel] │
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 11: 404 Not found
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                                       New share │
│                                                                        │
│ Share not found                                                        │
│                                                                        │
│ The link may be incorrect, or the share may have been deleted.         │
│                                                                        │
│ [Create new share                                                    ] │
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 12: 410 Expired
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                                       New share │
│                                                                        │
│ This share has expired                                                 │
│                                                                        │
│ Expired shares are no longer accessible and cannot be recovered.       │
│                                                                        │
│ [Create new share                                                    ] │
└────────────────────────────────────────────────────────────────────────┘
```

### Wireframe 13: 429 Rate limited
```text
┌────────────────────────────────────────────────────────────────────────┐
│ uPaste                                                       New share │
│                                                                        │
│ Too many requests                                                      │
│                                                                        │
│ Please wait approximately 60 seconds before trying again.              │
│                                                                        │
│ (Your draft content has been preserved in this window.)                │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 8. Long content and truncation behaviors

1. **Text payload width**:
   - The editor and viewer container have a max-width of 1180px.
   - Long lines in `PLAIN` format automatically soft-wrap (`word-break: break-word; white-space: pre-wrap;`).
   - Long lines in `SOURCE` format do not force the main layout to stretch horizontally. The code container horizontally scrolls internally (`overflow-x: auto`).
2. **Markdown content**:
   - Wide tables and fenced code blocks scroll horizontally within the rendered article surface without breaking page bounds.
3. **File names**:
   - Extremely long filenames (e.g. 200+ characters) truncate with an ellipsis in the middle or end, while the full original filename is always accessible via `title` attribute and accessible text.

---

## 9. UI to API mapping matrix

This matrix records the frozen Phase 6 intent for each user action. The implemented wire shapes are authoritative in [API.md](API.md) where they differ (for example `text: {format, content}`, `encrypted_text: {protocol, nonce, ciphertext}`, and the multipart `metadata` part).

| User action | HTTP method | Endpoint | Headers | Request body | Success status | UI transition |
|---|---|---|---|---|---|---|
| Create Standard Text | `POST` | `/api/v1/shares` | `Content-Type: application/json` | `{"payload_kind":"TEXT","privacy_mode":"STANDARD","text_format":"...","content":"...","expires_at":...}` | `201 Created` | Render creation success view with link and token |
| Create Encrypted Text | `POST` | `/api/v1/shares` | `Content-Type: application/json` | `{"payload_kind":"TEXT","privacy_mode":"ENCRYPTED","protocol":"UPASTE_AES_GCM_V1","nonce":"...","ciphertext":"...","expires_at":...}` | `201 Created` | Assemble URL with `#up_e1_` and render success view |
| Create Standard File | `POST` | `/api/v1/shares` | `multipart/form-data` | `file` (binary), `expires_at` (text) | `201 Created` | Render creation success view with download link and token |
| Load Share Viewer | `GET` | `/api/v1/shares/:id` | None | None | `200 OK` | Render viewer (or initiate local decryption if encrypted) |
| Download Raw Text | `GET` | `/raw/:id` | None | None | `200 OK` | Direct browser plain-text view |
| Download File | `GET` | `/f/:id` (File origin) | None | None | `200 OK` / `206 Partial` | Browser native attachment download |
| Update Text Content | `PATCH` | `/api/v1/shares/:id` | `Authorization: Bearer <token>`<br>`Content-Type: application/json` | `{"content":"...","text_format":"..."}` | `200 OK` | Show inline save confirmation |
| Update Encrypted Content | `PATCH` | `/api/v1/shares/:id` | `Authorization: Bearer <token>`<br>`Content-Type: application/json` | `{"protocol":"UPASTE_AES_GCM_V1","nonce":"...","ciphertext":"..."}` | `200 OK` | Show inline save confirmation |
| Update Expiration | `PATCH` | `/api/v1/shares/:id` | `Authorization: Bearer <token>`<br>`Content-Type: application/json` | `{"expires_at":...}` | `200 OK` | Show inline save confirmation |
| Delete Share | `DELETE` | `/api/v1/shares/:id` | `Authorization: Bearer <token>` | None | `204 No Content` | Render "Share deleted" confirmation screen |

---

## 10. Error mapping matrix

| HTTP status / Condition | Backend error code | User-facing title | User-facing message | UI presentation |
|---|---|---|---|---|
| `400 Bad Request` | `invalid_request` | Invalid submission | Specific field error returned by server. | Inline form field error message. Form draft retained. |
| `401 Unauthorized` | `unauthorized` | Invalid management token | The token provided is incorrect or has been revoked. | Inline error on token entry dialog. Prompt remains visible. |
| `404 Not Found` | `not_found` | Share not found | The link may be incorrect, or the share may have been deleted. | Full-page 404 state. Offers "New share" action. |
| `409 Conflict` | `encrypted_raw_unavailable` | Raw text unavailable | Encrypted shares cannot be downloaded as raw plain text. | Not triggered by UI (Raw button is hidden for encrypted shares). |
| `410 Gone` | `expired` | This share has expired | Expired shares are no longer available. | Full-page 410 state. Offers "New share" action. |
| `413 Payload Too Large` | `request_too_large` | Content too large | Payload exceeds limit (1 MiB text, 64 MiB file). | Inline form error. User draft retained. |
| `415 Unsupported Media Type`| `unsupported_media_type` | Unsupported request format | Expected application/json or multipart/form-data. | Inline error. |
| `422 Unprocessable Entity`| `unsupported_share_type` | Unsupported share option | Selected configuration is not supported. | Inline error. |
| `429 Too Many Requests` | `rate_limited` | Too many requests | Try again in about [X] seconds (from Retry-After). | Non-destructive modal/banner. Draft retained. |
| `500 Internal Error` | `internal_error` | Service error | An unexpected server error occurred. Please try again. | Friendly error banner with "Retry" action. |
| Network failure / Offline | `FETCH_ERROR` | Could not reach server | Network connection unavailable or server unreachable. | Inline warning banner with "Try again". Draft retained. |
| Decryption failure | `DECRYPT_ERROR` | Unable to decrypt this share | Link may be incomplete or encrypted content modified. | In-place error message within viewer shell. |
