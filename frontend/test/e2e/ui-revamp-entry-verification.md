# Phase 7 — story 1 verification

Scope: entry, registration/OIDC availability, forgot/reset password, no-collection presentation and the empty layout. Scanner, labels and tools are not changed. The referenced dashboard is visual-language guidance, not an entry artboard; existing page composition and routes remain in place.

## Automated checks

- `pnpm run build`: passed (static generation).
- Targeted ESLint on the five changed Vue files and the new browser spec: passed.
- `pnpm exec playwright test -c test/playwright.config.ts test/e2e/ui-revamp-entry.browser.spec.ts --project=chromium --reporter=line`: 4 passed. Uses isolated API fixtures, not live accounts. Covers login error, password toggle and keyboard ring, registration form availability, long OIDC button copy, German narrow layout, light/cinematic preference retention, recovery failure/success and token payload, missing-token disabled submit, OIDC-only/registration-disabled entry, no-collection create/join dialog access.
- `pnpm exec vitest --run --config test/vitest.config.ts composables/use-preferences.test.ts lib/data/apply-theme.test.ts`: 9 preference tests passed; no apply-theme test matched that filename.
- `pnpm run lint:ci`: fails its one-warning limit with two existing formatting warnings: `locales/zh-TW.test.ts:12` and `pages/location/[id]/index/index.vue:239`. Zero errors.
- `pnpm run typecheck`: fails with 198 diagnostics. Temporarily restoring the five changed files to HEAD and repeating produces the same 198 diagnostics, with no new diagnostic messages after ignoring shifted line numbers. Example: `pages/index.vue:70 TS2345` (Ref/WatchSource incompatibility), `pages/template/[id].vue:305 TS2339` (defaultDescription on Ref), `plugins/i18n.ts:33 TS2345` (I18n/Plugin incompatibility). Broader dependency/type repairs are outside this story.

## Visual checks

Inspected a Chromium full-page screenshot at 390px with saved cinematic theme, German translation and long OIDC text. Content wraps and password/submit controls remain visible. At 390, 834 and 1440px the login form has no horizontal overflow under light and cinematic themes.

Measured computed foreground/background contrast on login labels, recovery link and submit button (sRGB WCAG luminance): light labels 14.68:1, link/button 7.45:1; cinematic labels 14.29:1, link 11.85:1, button 11.14:1. These are sampled controls, not an audit of every theme.

## Remaining human checks

Real OIDC provider/auto-redirect, mail delivery and actual account registration/reset were not integration-tested; handlers and availability conditions are unchanged. Verify with configured accounts/mail service. Firefox/WebKit, all legacy themes and real mobile keyboard behavior remain for broader story 4 verification. No-collection retains its existing authenticated layout, whereas entry/recovery retain the sidebar-free empty layout. The singleton toaster and theme initialization remain in app.vue; removing the duplicate empty-layout toaster avoids double feedback without adding another theme watcher. The existing logged-out collectionId clearing behavior is preserved; no new preference reset or migration is introduced.
