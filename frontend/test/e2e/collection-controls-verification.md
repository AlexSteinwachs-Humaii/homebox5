# Collection controls — Phase 6 story 1

## Scope

Applied existing semantic tokens, `hb-panel`, section typography, responsive spacing and shared form/button/dialog primitives to the collection shell, settings and create/join/invite dialogs. The referenced dashboard is visual direction only: no admin artboard, hero imagery, forced theme or new administration controls were introduced. Members, invites, notifiers, entity-types, profile and tools page composition remain outside this story.

Preserved collection API requests, validation predicates, trimming/token parsing, expiry defaults, save feedback, success redirects/reloads, membership-dependent leave/delete choice and confirmation cancellation. Selector remains unchanged. Necessary shell integration corrections use the router's live path for active-tab highlighting and load collection scope on direct mobile visits even when the selector is not mounted.

The modal wrapper has an opt-in `keyboardFocus` prop for these three collection drawers: focus the first input on open, trap focus and use the existing Escape handling without dismissing an expanded nested selector. Non-opted-in drawers keep their prior defaults. Field labels now point to the actual currency trigger and invite-expiry input. Existing toast errors also remain visible inside the dialog/settings panel.

## Automated checks

- `pnpm run build`: passes (Nuxt static generation).
- Changed-file ESLint with `--max-warnings 0`: passes.
- `pnpm exec vitest --run --config test/vitest.config.ts lib/data/themes.test.ts`: 4 tests pass, including cinematic token contrast.
- `E2E_BASE_URL=http://localhost:3001 pnpm exec playwright test -c test/playwright.config.ts test/e2e/collection-controls.browser.spec.ts --project=chromium --workers=2 --retries=0 --reporter=line`: 10 tests pass against the production static build with intercepted API fixtures. A static SPA server must serve unknown paths as `index.html`.
- `pnpm run typecheck`: fails with 198 existing TypeScript diagnostics. Compared to the unchanged HEAD baseline after temporarily restoring the modified source files: zero added/removed diagnostics (normalized for line/column positions). Example: `pages/collection/index/tools.vue(79,53): error TS2345: Argument of type 'string | Date' is not assignable to parameter of type 'string'.`
- `pnpm run lint:ci`: fails on the same two baseline Prettier warnings in `locales/zh-TW.test.ts` and `pages/location/[id]/index/index.vue`: `ESLint found too many warnings (maximum: 1).` No warnings in changed files.

## Browser coverage and limits

Settings matrix: light/cinematic at 390, 834 and 1440px, long collection names, all six labeled navigation links, active-tab semantics, name/currency request payloads, save errors, keyboard-visible focus and leave/delete cancellation (multi-member versus sole-member fixtures). Switching checks verify settings and member data reload with the second collection's tenant header. Desktop dialogs verify create trimming, join-code rejection and URL parsing, invite uses validation, payloads, error retention and Escape. Mobile drawers verify keyboard entry/focus, readable inline errors, horizontal containment, create/join transition, expiry label and Escape dismissal.

These fixtures exercise the real frontend with stubbed responses, not backend authorization or successful create/join/invite persistence. Broader owner/non-owner administration, authenticated persistence, full translated locale testing, Firefox/WebKit and profile regression remain for the subsequent stories. Permissions and business rules were not changed.
