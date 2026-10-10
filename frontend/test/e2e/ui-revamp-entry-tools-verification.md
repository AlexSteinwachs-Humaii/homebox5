# Phase 7 — remaining-surface verification

## Scope and implementation

Story 4 only. Referenced dashboard revision 3 is visual-language guidance, not a new layout: no design canvas edits or invented routes. No API/schema/CSV changes. Earlier entry/scanner/tools implementations are retained.

Expanded `ui-revamp-entry.browser.spec.ts`, `scanner-surfaces.browser.spec.ts` and `tools-labels.browser.spec.ts` to 390, 834 and 1440px widths with saved light and explicitly chosen cinematic preferences. Added `ui-revamp-entry-tools.browser.spec.ts`, reusable API fixture and computed-contrast helper.

The keyboard check found a controlled singleton confirmation defect: Escape did not update its open state, and no trigger existed for focus restoration. `components/ModalConfirm.vue` now cancels on Escape and restores the captured, still-connected opener on close. Cancel and confirm/error both return focus; cancel sends no mutation.

## Coverage and evidence boundaries

- Login errors, password visibility toggle/ring, registration availability, OIDC-only availability, long German entry copy, missing reset token, recovery failure/success and token/password payloads. Saved theme, language and unrelated pagination preferences survive navigation.
- No-collection create/join access, initial input focus, Escape, create focus return and dialog fit.
- Scanner denied, unsupported and generic camera failure alerts, Retry keyboard activation/ring, camera selection and AR entry. Existing manual barcode invalid input makes no search request; valid fixture results can be selected with Enter and expose selected state/import action. This is not live detection.
- Existing Ensure Asset IDs confirmation: initial Cancel focus, Escape cancellation/no mutation, confirmed API failure feedback and focus return. Restore remains a separate labeled file control, not nested in another button.
- Empty/error asset responses return home with appropriate feedback; `/a/:id` forwards through the asset route and `/label/:id` reaches `/tag/:id`. Real attachments and multi-match asset records are not integrated by these fixtures.
- Computed WCAG sRGB contrast assertions require >=4.5:1 for sampled login/recovery labels, login submit, tool button, confirmation description and scanner error text. The helper rejects non-opaque/gradient surfaces instead of pretending to composite them. Tool ratios/colors are attached to the Playwright report. This is sampled text contrast, not an exhaustive audit of all states/themes or non-text boundaries.
- Print-media hides settings; sheets retain 8.5in width, 2.63×1in cards and 0.9in QR image width, skip/blank behavior, content, and explicit white background/black text under both themes. Metric equivalents also pass. Subpixel QR rounding uses 0.01px tolerance between media modes. QR endpoint in the browser fixture returns a placeholder SVG, **not a decodable QR code**. No claim of real barcode fidelity or physical print success is made. Existing unit checks protect generated print markup/styles/sanitization.

Inspected the cinematic 390px full-page tools screenshot: descriptions and buttons wrap within cards without horizontal clipping. Inspected the cinematic print-media screenshot: white sheet/black IDs and placeholder symbols, with screen controls absent. The placeholder symbols are visibly not valid QR codes; physical scan sign-off remains mandatory.

Browser artifacts are under ignored `frontend/test-results/`; tool screenshots use per-test output paths, contrast JSON is attached to reports. Scanner/print screenshots are representative artifacts, not golden snapshots. Fixtures block service workers so production PWA caching cannot bypass routed API responses; the camera fixture replaces MediaDevices (WebKit may return new wrappers on each access). These choices affect tests only.

## Commands and actual results

From `frontend/`, with a generated frontend served with SPA fallback at `http://localhost:3001` (dynamic `/a`, `/assets` and `/label` routes need fallback):

```sh
pnpm run build
pnpm exec vitest --run --config test/vitest.config.ts test/scanner-surfaces.test.ts test/tools-labels.test.ts composables/use-preferences.test.ts
E2E_BASE_URL=http://localhost:3001 pnpm exec playwright test -c test/playwright.config.ts test/e2e/ui-revamp-entry.browser.spec.ts test/e2e/ui-revamp-entry-tools.browser.spec.ts test/e2e/scanner-surfaces.browser.spec.ts test/e2e/tools-labels.browser.spec.ts --reporter=line --workers=3 --retries=0
```

- Build: passed.
- Vitest: 3 files, **30 tests passed**.
- Chromium matrix: **63 passed** before the cross-engine fixture refinements; the final combined run also passed all 63 Chromium checks.
- Final combined Chromium/Firefox/WebKit run: **187 passed, 2 failed** (no retries, 7.8m). Failures:
  - Firefox light/1440 asset API failure: `Expected pattern: /\/home\/?$/`, received `http://localhost:3001/assets/000-001` within the 5-second assertion timeout.
  - WebKit cinematic/1440 no-collection create dialog: `Test timeout of 30000ms exceeded`, `locator.evaluate ... waiting for getByRole('dialog')` after the initial-focus assertion.
- Focused desktop repeats: `E2E_BASE_URL=http://localhost:3001 pnpm exec playwright test -c test/playwright.config.ts test/e2e/ui-revamp-entry-tools.browser.spec.ts --project=firefox --project=webkit --grep '1440' --reporter=line --workers=1 --retries=0 --repeat-each=2`: **16 passed**. Concurrent desktop timing remains an open flakiness concern; do not describe the combined run as wholly green.
- Firefox initially failed print comparison on `86.39999389648438` versus `86.40000915527344` pixels. Corrected to physical-unit/subpixel-tolerance assertions, not changed product geometry.
- Initial cross-engine launch failed with missing browser executables. `pnpm exec playwright install firefox webkit` and `pnpm exec playwright install-deps firefox webkit` installed browsers/dependencies; all three engines subsequently launched.
- Changed-file ESLint: passed after `eslint ... --fix`.
- `pnpm run lint:ci`: **fails**, unchanged warnings at `locales/zh-TW.test.ts:12` and `pages/location/[id]/index/index.vue:239`: `ESLint found too many warnings (maximum: 1).` Zero errors.
- `pnpm run typecheck`: **fails** with 198 diagnostics, exit 2. Restoring `ModalConfirm.vue` to HEAD temporarily and rerunning yielded the same 198 diagnostic lines (`diff` empty). Examples: `pages/templates.vue(69,44): error TS2339: Property 'length' does not exist on type 'Ref<...>'`; `plugins/i18n.ts(33,14): error TS2345: Argument of type 'I18n<...>' is not assignable to parameter of type 'Plugin<...>'`. Tests and documentation do not repair the pre-existing dependency/type baseline.

## Required human verification (not completed by automation)

1. Real configured local registration/login and recovery mail delivery: request mail, follow its real token, check expired/reused tokens and sign in after reset. Verify configured external OIDC redirect/callback/logout and invitation return path. Fixtures only test handler/UI contracts, not external services.
2. On actual desktop/tablet/mobile devices, permit/deny camera, select front/rear cameras, start/stop/retry and close/reopen; ensure indicators turn off on navigation/unmount. Scan real QR/barcodes and activate AR/detection results; check glare, orientation and focus. Permission fixtures are not hardware evidence.
3. Generate real QR/barcode labels through the configured backend. Print at 100%/actual size, disable fit-to-page, confirm physical sheet/card/gap/padding measurements, quiet zones and readable IDs. Scan printed labels back to intended assets. Compare saved light and cinematic output; browser placeholders cannot establish decoder fidelity, DPI or printer margins.
4. Real mobile soft keyboard, scrolling/zoom, screen reader naming and tab traversal; all legacy themes and high-contrast OS modes are outside the sampled theme matrix.
5. Seeded live-backend tool authorization, ZIP/CSV/BOM downloads, restore empty-collection validation, backup deletion/wipe cancellation, attachment access and one/multiple asset results. Do not run destructive operations on production data. These tests deliberately isolate APIs rather than mutating live inventory.
6. Investigate/repeat the two concurrent desktop failures above on CI and live backend before claiming a stable cross-engine sign-off.
