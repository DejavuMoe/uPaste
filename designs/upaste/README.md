# uPaste interface exploration

Status: **V3 needs review**. Directions A, B and C were rejected on 2026-09-26 and remain historical artifacts. V2 creation and credential handoff were approved, implemented and pushed as `fe7a5dd` and `1dfb045`. V3 extends that direction without changing production code.

The rejection identified a shared problem: generated concept images dictated ornamental page silhouettes around the same empty form. V2 established the content-first direction; this iteration checks the remaining product workflows against that visual language.

The user specified **中文 / English switching**. These explorations use the real Share capability model and synthetic, invalid `.invalid` links and tokens. [Current capabilities](../../docs/ui/capabilities.md) and [source roles](design-sources.json) define their evidence boundary.

## V3 review — reading, control and conditional deployments

V3 is **design-only and needs review**. Its three design assets extend the approved V2 type, spacing and restrained control hierarchy. They are local simulations; no Share API, file origin, challenge service, or admin API is called. Cross-page links open fixed review fixtures rather than carrying typed prototype content. The mock Raw action opens a generated `text/plain` tab and the mock file action downloads synthetic bytes.

| Surface | Review the ordinary task | Review constraints and failures |
|---|---|---|
| Recipient and owner | [Markdown reading](prototype-v3.html?screen=read&kind=markdown), [file reading](prototype-v3.html?screen=read&kind=file), [management token entry](prototype-v3.html?screen=manage&kind=plain&token=missing) | [Source wrapping](prototype-v3.html?screen=read&kind=source), [missing decryption key](prototype-v3.html?screen=read&kind=encrypted&state=missing-key), [owner without key](prototype-v3.html?screen=manage&kind=encrypted&key=missing&token=present), [public retention limit](prototype-v3.html?screen=manage&kind=encrypted&key=missing&token=present&public=1) |
| Public creation | [Public Text creation](prototype-v3-public.html?demo=public), [private deployment contrast](prototype-v3-public.html?demo=private) | [Expired challenge](prototype-v3-public.html?demo=public&challenge=expired), [configuration failure](prototype-v3-public.html?demo=public&config=error), [submission failure](prototype-v3-public.html?demo=public&submit=once-error) |
| Superadmin, when enabled | [Token entry](prototype-v3-admin.html), [metadata overview](prototype-v3-admin.html?state=dashboard) | [Share detail](prototype-v3-admin.html?state=detail), [partial bulk failure](prototype-v3-admin.html?state=partial), [unavailable state](prototype-v3-admin.html?state=unavailable) |

The viewer keeps content first. Standard Text supports Copy and Raw; Source supports wrapping; Markdown has rendered/source views; File has metadata and a download action without inline preview. Encrypted Text needs the complete read-link fragment, and its key is never shown in the prototype DOM. Management begins at the token gate after a reload or direct entry. Without a valid decryption fragment, the token holder can change expiry or delete but cannot edit encrypted content. Public retention is finite and limited from Share creation time. The Superadmin preview is available only in its configured deployment; an actual disabled server returns 404 at `/admin`, rather than serving this prototype's unavailable panel.

The public prototype uses a simple local verification control to demonstrate widget states; it is not Cap or Turnstile. Its visible configuration-failure retry and submission block are proposed UI behavior: the current production config provider starts from private defaults and does not expose this dedicated screen. Admin empty/error guidance and cleanup count are likewise reviewable proposals, not claims about the current page.

Browser QA exercised 57 V3 states from 320–1440px across English/Chinese, light/dark, creation/result, reading, management, public challenge, admin overview and errors. The repository retains three aggregated rendered-DOM captures and 24 representative screenshots; detailed per-state captures were kept as local QA artifacts outside the repository. The three prototypes remain independent HTML entry points for review; V2's approved creation prototype remains intact. Approval of these surfaces has **not** been recorded, so production implementation is outside this iteration.

## Approved V2 — one composer

**Approved by the user on 2026-09-26 for the creation and one-time credential handoff slice.** Viewer, management, public governance and Superadmin surfaces remain outside this prototype approval.

[Open V2 with synthetic filled content](prototype-v2.html?demo=filled) · [Open its empty state](prototype-v2.html) · [Review the one-time token handoff](prototype-v2.html?demo=result)

V2 is one editor flow. It removes the ornamental rail, inspector, wizard, display-serif heading, colored logo fragment and option-card matrix. Content starts immediately below a compact task heading; Format, Privacy and Expires remain visible, and Create follows conditional inputs and encryption guidance. File mode hides inapplicable Format and Privacy choices. The result separates the complete read link from the one-time management token, with Copy token as its decisive action. Language switching retains the current draft and selections.

V2 was authored directly in HTML/CSS/JavaScript from evidenced uPaste jobs. The three generated concept images below are excluded as V2 composition references. The [filled desktop](screenshots/v2-filled-desktop-zh.png), [empty mobile](screenshots/v2-empty-mobile.png), [file mobile](screenshots/v2-file-mobile.png) and [result mobile](screenshots/v2-result-mobile.png) captures show content density and the credential handoff. Normal creation, file selection, language switching and the synthetic encrypted result were exercised in a browser at 1440 × 900, 390 × 844 and 360 × 640.

This approved slice covers creation and handoff. Its viewer and management surfaces are lightweight previews; V3 above expands those flows. V2 uses invalid `.invalid` links and a nonfunctional sample token, with no API requests or cryptography. The native `datetime-local` control uses the browser's locale for its internal date format, which may differ from the UI language selection.

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

Directions A, B and C are rejected historical versions. V2's creation and handoff direction is approved and implemented. V3 is ready for review; the project skill requires an explicit design approval and a separate design-only approval commit before any V3 production implementation.
