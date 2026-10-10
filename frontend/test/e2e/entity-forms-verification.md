# Entity create/edit form verification (Phase 4 story 2)

## Scope

- Create form composition and item/location editors reuse Phase 1 panels, headings, spacing and theme tokens. Existing fields, constraints, selectors, defaults, template flows, payloads and routes remain in place.
- Custom fields use one column on narrow screens (the previous two-column grid plus three-column span created implicit columns). Sticky editor controls still use the application's mobile/desktop header-height variables.
- Create controls wrap and expose a saving spinner instead of a decorative rotating package. Barcode buttons and advanced/entity-type controls have accessible names. Length feedback remains visible and is associated with text inputs; invalid-state attributes do not change validation.
- Autosizing `ui/textarea/Textarea.vue` now forwards input attributes to the actual textarea, not its wrapper. This is a shared-component accessibility correction and also benefits other callers. Date labels target the picker input; the picker uses foundation tokens without changing date-only conversion.
- Conversion cancellation restores focus to the initiating control after Save is re-enabled. Dialog/drawer cancellation otherwise uses the existing shell behavior.
- No API, schema, permission, attachment-operation or navigation changes. The reference canvas has only a dashboard: form structure is preserved rather than inventing a drawn workflow. Media presentation/operations and broader contrast verification remain story 3's scope.

## Verification

From `frontend/`:

- `pnpm exec vitest --run --config test/vitest.config.ts test/entity-forms.test.ts test/detail-surfaces.test.ts lib/data/themes.test.ts`: **23 passed**. Real SFC templates compile; actual extracted create handler runs against mocked boundaries for item, location, template, failure/validation and create-and-add branches. Existing detail tests still pass.
- `pnpm exec vitest --run --config test/vitest.config.ts lib/datelib/dateOnly.test.ts lib/datelib/datelib.test.ts`: **14 passed**.
- `pnpm exec playwright test -c test/playwright.config.ts test/e2e/entity-forms.browser.spec.ts test/e2e/detail-surfaces.browser.spec.ts --project=chromium --reporter=line --retries=0`: **12 passed**. Cinematic/HomeBox/Light × 390/1440px. Includes save failure/value retention, original date/parent/type/tag payloads, save without redirect, save with redirect, advanced visibility, conversion cancel/focus return, location save, create failure/retry/create-and-add, textarea/date accessible names, narrow input bounds, drawer scrolling and Escape/focus return. Fixtures mock API boundaries; no live collection data is mutated. Screenshots are written to Playwright's ignored test-results directory.
- `pnpm run build`: **passed**.
- `pnpm run lint:ci`: **passed**, with the existing one prettier warning at `locales/zh-TW.test.ts:12`.
- `pnpm run typecheck`: **fails with 195 baseline diagnostics**. Stashed this entire story, checked unchanged HEAD with the same dependencies, then restored and checked again: sorted diagnostic messages are identical after excluding source line/column offsets. Examples:
  - `components/Entity/CreateModal.vue(643,19): error TS18048: 'form.location' is possibly 'undefined'.`
  - `components/Item/View/ItemChangeDetails.vue(64,28): error TS2339: Property 'location' does not exist on type 'EntitySummary'.`

Browser tests run against generated static output served by Vite on port 3000. Firefox/WebKit, live backend integration, and actual photo operations were not part of this story's verification.
