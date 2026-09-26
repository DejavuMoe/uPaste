# uPaste interface exploration

Status: **changes requested** for directions A, B and C. The user rejected all three on 2026-09-26. They remain as historical artifacts and must not be treated as implementation candidates. The current production interface and API remain the functional baseline. The three earlier uncommitted create-draft guard edits are preserved separately.

The rejection identifies a shared problem: generated concept images dictated three ornamental page silhouettes around essentially the same empty form. The next direction must be judged with realistic filled content and the link/token handoff, with no ornamental rail, settings dashboard, or creation wizard.

The user rejected the current frontend prototype and asked for a full redesign using the project-level `prototype-first-ui` workflow. The user specified **中文 / English switching** for the new interface. These explorations use the same real Share model and synthetic, invalid `.invalid` links and management tokens. [Current capabilities](../../docs/ui/capabilities.md) and [source roles](design-sources.json) define their evidence boundary.

## Active revision — one composer

**Approved by the user on 2026-09-26 for the creation and one-time credential handoff slice.** Viewer, management, public governance and Superadmin surfaces remain outside this prototype approval.

[Open V2 with synthetic filled content](prototype-v2.html?demo=filled) · [Open its empty state](prototype-v2.html) · [Review the one-time token handoff](prototype-v2.html?demo=result)

V2 is one editor flow. It removes the ornamental rail, inspector, wizard, display-serif heading, colored logo fragment and option-card matrix. Content starts immediately below a compact task heading; Format, Privacy and Expires remain visible, and Create follows conditional inputs and encryption guidance. File mode hides inapplicable Format and Privacy choices. The result separates the complete read link from the one-time management token, with Copy token as its decisive action. Language switching retains the current draft and selections.

V2 was authored directly in HTML/CSS/JavaScript from evidenced uPaste jobs. The three generated concept images below are excluded as V2 composition references. The [filled desktop](screenshots/v2-filled-desktop-zh.png), [empty mobile](screenshots/v2-empty-mobile.png), [file mobile](screenshots/v2-file-mobile.png) and [result mobile](screenshots/v2-result-mobile.png) captures show content density and the credential handoff. Normal creation, file selection, language switching and the synthetic encrypted result were exercised in a browser at 1440 × 900, 390 × 844 and 360 × 640.

This is a **creation and handoff visual revision**, not a complete V1 prototype. Its viewer and management surfaces are lightweight previews; full Markdown rendering/source switching, Standard-only Raw, direct management token entry, public challenge/retention and Superadmin remain to be designed after this direction is reviewed. It uses invalid `.invalid` links and a nonfunctional sample token, with no API requests or cryptography. The native `datetime-local` control uses the browser's locale for its internal date format, which may differ from the UI language selection.

## Rejected directions retained for comparison

| Direction | Information architecture | Best fit | Design risk to test |
|---|---|---|---|
| [A — Focus canvas](variant-a.html) | Content first; format, privacy, expiry and action share the editor's lower edge. | Frequent, direct paste and upload. | Settings must remain discoverable and security choices clear. |
| [B — Dispatch console](variant-b.html) | Desktop content/editor and persistent delivery inspector; mobile settings follow content with current selections at submission. | Deliberate privacy and expiration choices. | Inspector density on tablet and mobile. |
| [C — Guided handoff](variant-c.html) | Content → Share settings → Result in three focused steps. | First-time users and one-time token handoff. | Extra step for frequent users. |

Start with the [visual comparison](index.html), then open each live prototype. Try text creation, file selection, privacy and expiry changes, the result link and masked management token, then switch language. The prototypes simulate UI state; they do **not** create a server Share, encrypt bytes, transmit files, or prove deployment behavior.

## Shared boundaries

- One Share contains either Text or one Standard File. Text allows Plain, Source and Markdown; Encrypted applies only to Text.
- Text is limited to 1 MiB UTF-8; one File is limited to 64 MiB. A File is downloaded as an attachment and has no inline preview.
- The complete encrypted read link carries the decryption key in its fragment. The one-time management token is separate from that link and cannot be recovered after leaving the creation result.
- Public deployment challenge, finite retention, Superadmin, and exceptional viewer states must be designed against the actual interfaces before implementation. These initial directions focus on the private creation and handoff flow.
- No history, public feed, search, ordinary user accounts, encrypted files or multi-file shares are implied.

## Concept-to-prototype visual review

Generated concept images are inspiration only. Their incidental text is not product copy or functional authority. Each row records five comparison points at 1440 × 900, with the 390 × 844 continuation checked separately.

| Direction | Composition | Typography | Palette | Control hierarchy | Responsive continuation |
|---|---|---|---|---|---|
| A | Brand rail, centered editor and bottom controls follow the [concept](assets/concept-focus-canvas.png). | Serif heading and compact control text retain the editorial character. | Warm paper, ink and cobalt match the concept's restrained contrast. | Text/File sit on the canvas; settings and Create remain adjacent. | Rail becomes a compact header; Create is within the first mobile viewport. |
| B | Editor/inspector split follows the [concept](assets/concept-dispatch-console.png). | Precise utility type and strong headings remain readable. | Graphite and one cyan accent retain the concept's contrast without glow. | Format, privacy and expiry stay visible on desktop. | Inspector becomes a second section; submission summarizes current privacy and expiry. |
| C | Numbered left progress rail and focused work region follow the [concept](assets/concept-guided-handoff.png). | Large editorial heading and small step labels establish stage hierarchy. | Mineral white, charcoal and vermilion follow the concept. | Stage 1 contains content; stage 2 contains delivery settings; stage 3 handles the link and token. | Progress rail becomes horizontal; the next action remains visible at 390 px. |

The concepts are not approved specifications. Intentional changes in the rendered exploration serve the real product: A uses a three-value theme selector for System/Light/Dark; B shows all supported expiration presets and a current privacy/expiry summary in the mobile submission area; C adds the separate read and management actions to the result step. All three use the requested language switch rather than showing two languages in every field.

## Review and next gate

Directions A, B and C are rejected historical versions. V2's creation and handoff direction is approved. The next design-only slice should complete public-mode challenge and retention, viewer and management errors, and the conditional Superadmin surface. The project skill requires a separate design-only approval commit before production implementation of the approved slice.
