# Dashboard accessibility and theme regression audit

Story: Verify dashboard interaction, accessibility and theme compatibility (Phase 2).

## Reproduce

Use a running seeded API with registration enabled and serve the current generated frontend at the same origin (proxy `/api` to the API when serving separately). Tests use the existing demo login for read-only presentation fixtures and create disposable accounts for real preference/collection persistence. Do not run against production. Set `E2E_BASE_URL` to that origin; leave `TEST_SHUTDOWN_API_SERVER` unset.

From `frontend/`:

```sh
pnpm run build
E2E_BASE_URL=http://localhost:3001 pnpm exec playwright test -c test/playwright.config.ts test/e2e/ui-revamp-dashboard.browser.spec.ts test/e2e/dashboard-overview.browser.spec.ts --project=chromium --workers=2 --retries=0 --reporter=line
pnpm exec vitest --run --config test/vitest.config.ts lib/utils.test.ts composables/use-preferences.test.ts lib/data/themes.test.ts
pnpm run lint
pnpm run typecheck
```

## Evidence and limits

- Chromium: 28 dashboard browser cases passed. HomeBox, Light and Cinematic × 390/834/1440px × empty/populated; populated fixtures include unchanged photo URLs/fit, absent photos, colored/uncolored tags, long item/location/tag names and zero location counts.
- Keyboard-only Tab/Enter reaches existing item, location and tag destinations at mobile and desktop widths. Link names are nonempty, link markup is not nested, and focus rings are checked. Photo-link focus uses a neutral backing and an inset overlay so neither card clipping nor photo content obscures it.
- Contrast is calculated from browser-computed styles, with ancestor alpha backgrounds composited, not from the design illustration. Every visible dashboard text node is checked at 4.5:1 (also stricter than AA's large-text minimum). Hover and keyboard-focus states are included. Focus/input tokens are checked at 3:1 against rendered surfaces. The measured matrix minima were 5.18:1 for text and 3.53:1 for control/focus surfaces. Each presentation test writes `computed-contrast.json` and `dashboard.png` to Playwright's output directory; mobile photo focus is captured separately.
- Light/Cinematic preference tests perform a real server PUT, clear only the device preference cache **before** app startup, reload, create/switch between real collections, then reload again. Clearing localStorage while mounted would itself change the reactive preference and save a default; that is not a reload test.
- Full-page screenshots inspected at desktop, tablet and mobile: hierarchy retained, camera hidden below its existing large breakpoint, original photos and neutral placeholders distinguishable, labels wrap without page overflow. The application's `lg` JS breakpoint is 768px, so the existing table remains at 834px; no breakpoint migration was introduced.
- Unit tests: 25 passed, including a 4,096-color sRGB grid verifying black/white tag text AA selection, existing theme handling, and preference sync.
- Build passed. Full lint passed with the existing formatting warning in `locales/zh-TW.test.ts`.
- Typecheck exits 2 with 195 existing diagnostics. A comparison against pre-story HEAD using the same installed dependencies found identical diagnostic messages/counts (ignoring shifted source line numbers). Examples: `components/Entity/CreateModal.vue(638,19): error TS18048: 'form.location' is possibly 'undefined'.` and `components/Item/View/table/columns.ts: error TS2339: Property 'location' does not exist on type 'EntitySummary'.` No new diagnostic was introduced.

This is scoped dashboard verification, not whole-product accessibility certification or approval of every legacy theme. Other browsers, assistive-technology announcement quality and production photo/color variations still merit human review. Decorative camera colors, uploaded imagery and the brand logotype are not text/control contrast evidence. The referenced design's decorative approval is not accessibility sign-off. No backend, API, theme preference migration or design-canvas changes were made.
