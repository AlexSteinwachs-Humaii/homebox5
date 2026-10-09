# Phase 1 foundation regression verification

## Automated coverage

`e2e/ui-revamp-foundation.browser.spec.ts` adds 27 tests, alongside the three existing shell presentation tests:

- Clean browser contexts with existing-account settings snapshots: absent theme means HomeBox; explicit HomeBox, Light and Cinematic survive reload. GET/PUT fixtures do not mutate the shared demo user's settings.
- Real, uniquely named local registration, Cinematic default, all 30 theme options/names retained, keyboard selection and pressed state, visible theme-picker focus, real server PUT, removal of device-local storage and reload.
- Real creation of a second collection, switch back to the original collection and reload. Uses the disposable registered account, not the shared demo collection.
- Every existing sidebar destination navigates and receives `aria-current`. Tests start from Home separately to avoid overlapping asynchronous page navigation.
- Each Create action opens the correct entity/tag dialog, Tab remains inside, Escape closes and focus returns to Create. Expanded nested selectors consume Escape before the parent closes.
- Scanner permission denial feedback and granted-media dialog access/focus return. The media request is stubbed, not a claim of physical camera verification.
- Cinematic and saved HomeBox light theme at 1440px, 834px and 390px: long German-label stress, sidebar/page overflow checks, settled mobile drawer position, bounded quick-menu geometry, Tab containment, accessible title, Escape and focus return. Labels are substituted in the rendered DOM after initial settings/collection loading; this is a layout stress test, not a translation-catalog audit. Shell screenshots are saved to each test's output directory.
- Existing shell suite additionally covers collapse/expand, Ctrl+B, search routing, quick-menu Location dispatch, 390px collection-creation drawer, legacy light header, scanner and sign-out.
- Existing Go repository/service tests verify invited registration, new OIDC account defaults and preservation on existing-account OIDC login/linking. OIDC is not mocked as a browser provider in this suite.

Run against a running API and frontend with registration/demo login enabled:

```sh
cd frontend
E2E_BASE_URL=http://localhost:3001 pnpm exec playwright test -c test/playwright.config.ts \
  test/e2e/ui-revamp-foundation.browser.spec.ts test/e2e/shell-presentation.browser.spec.ts \
  --project=chromium --project=firefox --workers=1 --retries=0 --reporter=line
```

Verification used generated Nuxt output and the previous story's isolated demo Go API (temporary SQLite), with a same-origin static preview proxy forwarding Host/cookies. No production settings were changed. The registration test intentionally leaves uniquely named test accounts/collections in the test database; use a disposable database.

## Measured contrast and corrections

Browser checks resolve the actual HSL custom properties through computed CSS into sRGB, linearize channels per WCAG relative luminance, and calculate `(Llighter + .05) / (Ldarker + .05)`. Each contrast test attaches a JSON measurement table to the Playwright report. Use `--reporter=html` to retain the attachments, or `--reporter=json` for machine-readable results.

| Shared roles tested | Cinematic minimum | HomeBox light minimum | Threshold |
| --- | ---: | ---: | ---: |
| Normal text across background/card/popover/muted/background-accent; button/selected/sidebar text; primary links; primary hover | 7.26:1 | 4.69:1 | 4.5:1 |
| Focus ring, sidebar ring and input boundaries against shared surfaces | 3.69:1 | 3.54:1 | 3:1 |
| Primary button 90% hover fill over page background | 9.08:1 | 4.69:1 | 4.5:1 |

Initial HomeBox failures: primary foreground on primary **3.99:1**, input on white **1.57:1**, sidebar ring on sidebar **1.25:1**. The HomeBox green primary/ring lightness is now 33% (was 43%); input gray lightness 47% (was 81%). Decorative border colors are unchanged. Cinematic and the remaining legacy palettes are retained unchanged. This verifies these shared roles in Cinematic/HomeBox, not every legacy theme or every page's content. Disabled controls are exempt from contrast requirements; decorative separators are not used as the only control boundary.

Visible focus is checked on the theme picker and search input. Theme names/pressed state, navigation labels/current state, collection-search label, scanner, quick-menu title and close controls are verified or use existing translated labels.

## Accessibility fixes from the browser walk

- Collection search now has an explicit translated accessible name.
- Create selection is dispatched after the dropdown closes, with focus first restored to the durable Create trigger. This avoids returning to a removed menu item (observed in Firefox).
- The shared scroll-dialog wrapper forwards native listeners explicitly. Shared Create dialogs handle Escape in capture when no nested selector is expanded, preventing an unmounted menu/selector layer from swallowing Escape (observed in Chromium). Expanded selectors keep their own first-Escape behavior; Reka still owns trapping and focus restoration.
- Login tests wait for demo credential initialization before filling fields, and collection-dependent checks wait for the collection to load rather than snapshotting the placeholder.

## Verification results and limitations

- Targeted Chromium + Firefox: **60 tests passed**, no retries. Responsive/contrast evidence rerun in Chromium: **8 passed**.
- `pnpm exec vitest --run --config test/vitest.config.ts composables/use-preferences.test.ts lib/data/themes.test.ts`: **13 passed**.
- `cd backend && go test ./internal/data/repo ./internal/core/services`: **passed**.
- `pnpm run build`: **passed**.
- `pnpm run lint:ci`: **passed**, existing `locales/zh-TW.test.ts:12` Prettier warning only.
- `pnpm run typecheck`: **exit 2**, 100 distinct existing file/error-message pairs, identical to the previous story's diagnostics after ignoring shifted line numbers. Examples: `layouts/default.vue: Property 'build' does not exist on type 'Ref<APISummary | undefined, APISummary | undefined>'`; `pages/templates.vue: Type 'ComputedRef<string>' is not assignable to type 'ResolvableTitle'`; `plugins/i18n.ts: I18n ... is not assignable to Plugin ...`. No added diagnostic pairs. Nuxt also logs `[Vue] Load plugin failed: vue-router/volar/sfc-route-blocks`.
- WebKit was attempted after installing its system dependencies. It returns to Login after briefly reaching Home in this local preview; saved Light/Cinematic assertions consequently receive `homebox`. Recheck: **2 passed / 3 failed** (existing-account defaults + 390px Cinematic). The unchanged `login.browser.spec.ts` reproduces `Expected /home; Received /` in WebKit as well. This is unresolved, not a WebKit accessibility sign-off; investigate authentication/preview behavior on the normal E2E server.
- Initial preview run failed with `net::ERR_EMPTY_RESPONSE`: rebuilding removed the Python server's working directory. Restarted the helper with an explicit stable document-root path; subsequent Chromium/Firefox runs pass.
- Initial registration fixture used a password containing the deliberately prohibited word `password`; replaced the fixture, without relaxing validation.

Reference revision 3 informs shared hierarchy, not dashboard artwork. Responsive screenshots were inspected for wrapping and mobile drawer bounds. Existing dashboard content remains unchanged; Phase 2 owns the hero. Manual checks still needed: real camera/device permissions and decoding, assistive-technology announcements, and WebKit on the normal deployed/test server. Do not infer whole-application WCAG conformance from these shared-foundation measurements.
