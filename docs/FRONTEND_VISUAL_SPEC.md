# Frontend visual specification

This document establishes the visual design language, design tokens, typography, spacing, component styling, responsive behavior, and anti-AI-slop constraints for the uPaste frontend.

---

## 1. Anti-AI-slop design constraints

To prevent generic, over-decorated, or faux-modern interfaces, the following rules are frozen as hard design boundaries:

### Explicitly forbidden elements:
- ❌ **No gradients**: No multi-color background gradients, rainbow text, or gradient buttons.
- ❌ **No glowing borders / drop shadows**: No neon glow effects, colored drop-shadow halos, or futuristic borders.
- ❌ **No glassmorphism / excessive backdrop filters**: No frosted-glass panels or blurred semi-transparent cards stacked over each other.
- ❌ **No decorative floating blobs / mesh gradients**: No blurred geometric blobs drifting in the background.
- ❌ **No marketing headlines**: No "Share Code with Superpowers", "Next-Gen Pastebin", or marketing banners.
- ❌ **No fake testimonials or statistics**: No "Loved by 10,000+ developers" or fake uptime widgets.
- ❌ **No security scorecards**: No "Military-grade encryption" badges or generic shield/lock clip art.
- ❌ **No feature-card grids**: The homepage is the tool itself, not a grid of "Fast / Secure / Simple" feature cards.
- ❌ **No oversized pill radiuses**: Border radii must remain modest (max 12px); no 24px–32px pill-shaped buttons or input fields.
- ❌ **No oversized decorative icons**: Icons should be utility-sized (16px–20px); no giant 64px illustrative icons.
- ❌ **No ornamental sidebars or dashboards**: uPaste has no multi-page navigation or persistent toolbars.
- ❌ **No animated typewriter effects**: Content appears instantly without artificial typing animations.
- ❌ **No marketing badges**: No "Powered by AI", "Built with React", or commit hashes in the UI chrome.
- ❌ **No empty-space filling**: Empty space is normal and intentional. Do not add decorative dividers or illustrations to fill space.

---

## 2. Visual direction and design philosophy

- **Character**: Quiet, neutral, precise, technical utility. It should feel like a high-performance text editor or command-line companion, not an IDE and not a marketing website.
- **Surfaces**: Crisp neutral layers with 1px solid borders.
- **Contrast**: High legibility for long text reading. Avoid pure black (`#000000`) and pure white (`#ffffff`) surfaces to reduce eye fatigue.
- **Color palette**: Dominated by neutrals (grays/slates) with exactly **one** restrained accent color (a muted technical blue/indigo) used sparingly for primary actions and active focus indicators.

---

## 3. Typography system

All typography uses system fonts. Remote font fetching (such as Google Fonts or Adobe Fonts) is **strictly forbidden**.

### Font stacks:

```css
/* UI / Body font stack */
--font-sans: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont,
             "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;

/* Code / Monospace font stack */
--font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
             "Liberation Mono", "Courier New", monospace;
```

*Note*: If `Inter` is locally installed on the client operating system, it is rendered; otherwise, the client seamlessly falls back to system UI fonts.

### Typographic scale:

| Token | Size | Line height | Weight | Use case |
|---|---|---|---|---|
| `--text-xs` | 12px (0.75rem) | 16px (1.33) | 400 / 500 | Metadata, timestamps, byte counters, badges |
| `--text-sm` | 14px (0.875rem) | 20px (1.43) | 400 / 500 | Labels, secondary buttons, helper text, table rows |
| `--text-base` | 16px (1.0rem) | 24px (1.50) | 400 / 500 | Default body text, primary inputs, editor content |
| `--text-lg` | 18px (1.125rem) | 28px (1.55) | 500 / 600 | Section headers, card titles |
| `--text-xl` | 20px (1.25rem) | 28px (1.40) | 600 | Dialog headers, page titles (`New share`) |

---

## 4. Semantic design tokens

### Light mode

```css
:root, [data-theme="light"] {
  /* Surfaces */
  --color-canvas: #f8fafc;           /* Slate 50: Page background */
  --color-surface: #ffffff;          /* Pure white: Content containers, inputs */
  --color-surface-elevated: #ffffff; /* Dialogs, dropdown menus */
  --color-surface-sunken: #f1f5f9;   /* Slate 100: Code block backgrounds, drop zone */

  /* Borders */
  --color-border-subtle: #e2e8f0;    /* Slate 200: Soft dividers */
  --color-border-default: #cbd5e1;   /* Slate 300: Standard 1px input and card borders */
  --color-border-strong: #94a3b8;    /* Slate 400: Active/hover borders */

  /* Text */
  --color-text-primary: #0f172a;     /* Slate 900: High-contrast primary text */
  --color-text-secondary: #475569;   /* Slate 600: Secondary text, descriptions */
  --color-text-muted: #64748b;       /* Slate 500: Placeholders, metadata, timestamps */
  --color-text-disabled: #94a3b8;    /* Slate 400: Disabled controls */

  /* Accent (Restrained Blue) */
  --color-accent-default: #2563eb;   /* Blue 600: Primary buttons, active states */
  --color-accent-hover: #1d4ed8;     /* Blue 700: Button hover */
  --color-accent-subtle: #eff6ff;    /* Blue 50: Selection highlight, active tab */
  --color-accent-text: #ffffff;      /* Contrast text on accent */

  /* Semantic Feedback */
  --color-danger-default: #dc2626;   /* Red 600: Delete action, errors */
  --color-danger-hover: #b91c1c;     /* Red 700: Danger hover */
  --color-danger-subtle: #fef2f2;    /* Red 50: Error container background */
  --color-danger-text: #991b1b;      /* Red 800: Error text */

  --color-warning-default: #d97706;  /* Amber 600: Quota warning */
  --color-warning-subtle: #fffbeb;   /* Amber 50: Warning background */

  /* Focus & Selection */
  --color-focus-ring: #3b82f6;       /* Blue 500: 2px accessible focus ring */
  --color-selection-bg: #bfdbfe;     /* Blue 200: Text selection highlight */

  /* Code */
  --color-code-bg: #f8fafc;
  --color-code-text: #0f172a;
}
```

### Dark mode

```css
[data-theme="dark"] {
  /* Surfaces */
  --color-canvas: #0b0f17;           /* Off-black slate: Page background */
  --color-surface: #131b26;          /* Dark slate: Content containers, inputs */
  --color-surface-elevated: #1a2332; /* Dialogs, dropdown menus */
  --color-surface-sunken: #0e141d;   /* Sunken code blocks, drop zone */

  /* Borders */
  --color-border-subtle: #1e293b;    /* Slate 800: Soft dividers */
  --color-border-default: #334155;   /* Slate 700: Standard 1px borders */
  --color-border-strong: #475569;    /* Slate 600: Active/hover borders */

  /* Text */
  --color-text-primary: #f1f5f9;     /* Slate 100: High-contrast primary text */
  --color-text-secondary: #cbd5e1;   /* Slate 300: Secondary text */
  --color-text-muted: #94a3b8;       /* Slate 400: Metadata, placeholders */
  --color-text-disabled: #64748b;    /* Slate 500: Disabled controls */

  /* Accent (Restrained Blue) */
  --color-accent-default: #3b82f6;   /* Blue 500: Primary buttons, active states */
  --color-accent-hover: #60a5fa;     /* Blue 400: Button hover */
  --color-accent-subtle: #172554;    /* Blue 950: Active tab/selection highlight */
  --color-accent-text: #ffffff;

  /* Semantic Feedback */
  --color-danger-default: #ef4444;   /* Red 500: Delete action, errors */
  --color-danger-hover: #f87171;     /* Red 400: Danger hover */
  --color-danger-subtle: #450a0a;    /* Red 950: Error container background */
  --color-danger-text: #fca5a5;      /* Red 300: Error text */

  --color-warning-default: #f59e0b;  /* Amber 500: Quota warning */
  --color-warning-subtle: #451a03;   /* Amber 950: Warning background */

  /* Focus & Selection */
  --color-focus-ring: #60a5fa;       /* Blue 400: 2px accessible focus ring */
  --color-selection-bg: #1e3a8a;     /* Blue 900: Text selection highlight */

  /* Code */
  --color-code-bg: #0e141d;
  --color-code-text: #f1f5f9;
}
```

---

## 5. Spacing, radius, and elevation

### Spacing scale:
- `--space-1`: `4px` (Tight padding, badge gaps)
- `--space-2`: `8px` (Icon/label spacing, inline form gaps)
- `--space-3`: `12px` (Control padding, dropdown item padding)
- `--space-4`: `16px` (Card padding, form field gaps)
- `--space-6`: `24px` (Major section gaps)
- `--space-8`: `32px` (Page header margin, container padding)
- `--space-12`: `48px` (Page top/bottom padding)

### Border radius scale:
- `--radius-none`: `0px` (Code blocks if full-bleed)
- `--radius-sm`: `4px` (Badges, small inline buttons, tags)
- `--radius-md`: `6px` (Standard buttons, form inputs, dropdowns)
- `--radius-lg`: `8px` (Containers, editor border, dialog panels)
- `--radius-xl`: `12px` (Max outer modal dialog radius)
*Note*: Radiuses larger than 12px are strictly forbidden.

### Shadows:
- Shadows are minimal and strictly functional (used only to elevate dialogs from backdrops):
  - Card / Input: `none` (use 1px solid border instead).
  - Dialog / Modal: `0 4px 12px rgba(0, 0, 0, 0.15)`.

---

## 6. Component styling rules

### 6.1 Buttons
Buttons have three visual roles plus a quiet text action:
1. **Primary**: Solid accent background (`--color-accent-default`), white text (`--color-accent-text`), subtle hover (`--color-accent-hover`). Used once per immediate task (e.g. `Create share`, `Save changes`).
2. **Secondary**: Neutral surface (`--color-surface`), 1px border (`--color-border-default`), primary text (`--color-text-primary`). Used for non-primary actions (e.g. `Copy`, `Raw`, `Cancel`).
3. **Danger**: Solid red (`--color-danger-default`) or neutral with danger text, transitioning to red on hover. Used exclusively for permanent deletion (`Delete share`).
4. **Quiet / Ghost**: Transparent background, text only, subtle hover background. Used for secondary navigation or dismiss actions.

### 6.2 Form inputs and editor
- Inputs, selects, and textareas: 1px solid border (`--color-border-default`), background (`--color-surface`), radius `6px`, padding `8px 12px`.
- Active focus: 2px outline (`--color-focus-ring`) with 2px offset. Never remove outline without replacing it with an accessible focus indicator.
- Editor textarea: occupies the central viewport, min-height `320px`, expands responsively, with line numbers aligned to the left.

### 6.3 Tabs
- Text / File tabs use a clean bottom-border tab indicator:
  - Active tab: `--color-text-primary` with 2px solid bottom border (`--color-accent-default`).
  - Inactive tab: `--color-text-secondary`, no bottom border.

---

## 7. Responsive viewport contracts and visual QA

Phase 7 must visually validate and pass visual QA across the following 5 mandatory screen widths:

| Viewport | Device category | Target resolution | Layout contract |
|---|---|---|---|
| `360 px` | Small mobile | 360 × 640 | Single column, stacked actions, horizontal scroll only inside code blocks |
| `390 px` | Standard mobile | 390 × 844 | Full-width controls, 16px horizontal page margins, minimum 44px touch targets |
| `768 px` | Tablet portrait | 768 × 1024 | Working width 720px, inline controls for Format/Privacy/Expiration |
| `1024 px` | Tablet landscape / Laptop | 1024 × 768 | Working width 960px, comfortable editing margins |
| `1440 px` | Desktop workstation | 1440 × 900 | Working width 1140px centered, content column does not stretch infinitely |

### Mobile layout rules:
- **No horizontal window scrolling**: `body` and root containers must never overflow horizontally.
- **Code block containment**: Long lines in `SOURCE` view scroll horizontally within their dedicated container (`overflow-x: auto`) rather than breaking the page width.
- **Stacked action buttons**: On viewports ≤ 480px, action buttons (e.g. `Open share`, `Manage share`, `New share`) stack vertically with full width.

---

## 8. Motion and transitions

- **Timing**: Transitions must be subtle and fast: **100ms – 180ms** with `ease-out`.
- **Scope**: Applied only to interactive states: button hover colors, focus rings, and dialog backdrop opacity.
- **Reduced motion**:
  ```css
  @media (prefers-reduced-motion: reduce) {
    *, ::before, ::after {
      animation-duration: 0.01ms !important;
      animation-iteration-count: 1 !important;
      transition-duration: 0.01ms !important;
    }
  }
  ```
