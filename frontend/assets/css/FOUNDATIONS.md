# Shared UI foundations

Story 1 of New UI Revamp Phase 1 adds the selectable `cinematic` theme. It does **not** change the legacy `homebox` preference fallback, initialize account settings, redesign the shell, or add hero art. Those belong to subsequent stories/phases.

## Consumption

Use the existing HSL roles instead of page-local colors:

- `bg-background` / `bg-surface`: canvas.
- `bg-card` / `bg-surface-raised`, `text-card-foreground`: working panels.
- `bg-muted` / `bg-surface-inset`: subordinate surfaces.
- `bg-popover` / `bg-surface-overlay`, `text-popover-foreground`: overlays.
- `text-foreground`, `text-muted-foreground`: normal/secondary text.
- `bg-primary text-primary-foreground`: main actions; `bg-accent text-accent-foreground`: interactive highlights.
- `border-border`: decorative dividers. `border-input`: control boundaries. `ring-ring`: focus.

The `surface` utility aliases reference the existing roles directly so nested theme previews and every saved theme resolve their own palette. Only `.theme-cinematic` contains the new dark colors. `.theme-homebox` explicitly shares the legacy palette so its preview is accurate even within a cinematic page.

Shared geometry uses `px-page-gutter` (responsive 1–2rem) and `my-section-gap` / `gap-section-gap` (1.5rem). Existing theme radii remain intact. `hb-panel`, `hb-section-title`, `hb-display`, `hb-eyebrow`, and `hb-focus` are reusable component treatments. Avoid combining these with conflicting utilities from a primitive; use explicit title utilities when overriding `CardTitle` defaults.

Body and section typography use a local system sans-serif stack; `font-editorial` / `hb-display` offer a local Georgia/Times serif display treatment. No downloaded fonts, third-party branding, decorative art, or new animations are needed. Hero composition is deliberately not applied in this story.

Keep Reka/shadcn focus handling and component contracts. The existing button/input/dialog primitives already consume semantic colors and focus rings. The theme picker uses native buttons, theme names as accessible labels, and `aria-pressed` for selection. Its focus/selection ring belongs to the **current** theme, not the preview theme. Existing preference synchronization and the early theme script remain in place.

## Contrast evidence

`lib/data/themes.test.ts` calculates WCAG sRGB relative luminance directly from the cinematic CSS HSL values. It guards normal text at 4.5:1 and control/focus boundaries at 3:1. Representative measured ratios:

| Pair | Ratio |
| --- | ---: |
| Foreground / canvas | 16.64:1 |
| Muted text / card | 8.81:1 |
| Primary action text / primary | 11.13:1 |
| Accent text / accent | 9.86:1 |
| Focus ring / accent | 9.06:1 |
| Input boundary / accent | 3.09:1 |

Decorative borders intentionally remain quieter than input boundaries. This is token-level evidence, not accessibility sign-off for every page, image, opacity variant, or existing theme.

## Story verification

- Targeted Vitest tests cover choice preservation, all selectable theme styles, root class replacement, cinematic normal text, and control/focus contrast.
- Chromium component smoke check compiled the actual Vue ThemePicker and actual theme composable with an isolated in-memory preference provider. At 1440, 834, and 390px, all 30 choices were present, Enter/Space switched themes, selected semantics updated, focus was visible, and cinematic/light layouts had no horizontal overflow. This does not verify authenticated settings persistence or the full application shell.
- Production Nuxt generation and lint pass. Lint retains the existing single formatting warning in `locales/zh-TW.test.ts`.
- Full typecheck has 196 pre-existing diagnostics, verified by running the same command on the clean queue baseline and comparing diagnostic sets. No new diagnostics remain in this story.

Run token tests with `pnpm exec vitest --run --config ./test/vitest.config.ts lib/data/themes.test.ts` from `frontend/`. Broader authenticated theme persistence, account defaults, shell, dialog, and browser regression coverage remain for subsequent stories.
