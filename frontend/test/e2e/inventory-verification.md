# Inventory revamp verification — story 3

Verified 2026-10-10. The dashboard revision 3 is a visual-language reference, not an inventory layout approval. Existing routes, pagination scopes and API contracts remain unchanged; no backend changes or new bulk capabilities.

## Automated browser evidence

`ui-revamp-inventory.browser.spec.ts` plus `inventory-search.browser.spec.ts`: **28 Chromium tests passed, no retries, skips or flaky results** (124 seconds).

- Cinematic, HomeBox and Light at 390, 834 and 1440px: saved table/card preference, column visibility/order and page-size reload, saved-theme reload, keyboard column movement/toggles, mixed/all selection, selection carried between table/card views, selection cleared on reload, long unbroken names, missing-photo cards, bounded horizontal table scrolling and no document overflow.
- External `/items` paging clears selection; internal `/tag/:id` and `/item/:id` lists retain sorting and off-page selection without requesting another API page. Shared lists checked at all three widths.
- Keyboard Space operates row/card/select-all checkboxes; Enter operates view/settings/sort controls. Settings close/Escape restores the exact opener. Bulk-delete cancellation restores its menu trigger, keeps selection and performs no mutation; confirmation clears selection and refreshes results. Every existing bulk menu entry remains enabled with selection.
- A real disposable account is registered against the SQLite backend and seeded with 14 records. Real browser login, photo filtering, API order-by changes/reload, bulk duplication and confirmed deletion pass; inventory is cleaned up in `finally`. The remaining disposable account exists only in the isolated test database.
- Existing search suite covers combined URL filters, reload, query back/forward/clearing, custom fields, keyboard photo switches, long location/tag labels, German controls at 390px, invalid asset ID, held loading, empty results and HTTP 500 feedback.

Most layout/state cases intercept API responses and cannot prove server persistence. The final seeded-backend test does **not** intercept responses. Column headers and table/card display are device-local preferences; page size and theme use the existing server sync. Column order is therefore asserted in local storage and after reload, not incorrectly asserted in a server settings payload.

## Contrast measurements

Computed browser colors, sRGB relative luminance and alpha-composited ancestor backgrounds; no copied palette constants. JSON evidence is attached to each theme's contrast test in the Playwright report. Text target: 4.5:1; focus and control-indicator target: 3:1.

| Theme | Header text | Selected/hover row text | Status text | Search text | Search-button hover | Header focus ring | Card unchecked border | Card checked indicator |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Cinematic | 11.14 | 13.03 | 8.78 | 16.72 | 11.06 | 10.81 | 13.87 | 11.14 |
| HomeBox | 5.99 | 10.14 | 12.63 | 12.63 | 5.99 | 5.38 | 6.71 | 5.99 |
| Light | 7.45 | 13.11 | 14.68 | 14.68 | 7.32 | 6.66 | 7.45 | 7.45 |

Hover values can vary slightly during the existing color transition; thresholds are asserted on the rendered result. Focus tokens also pass against background/card/muted/popover surfaces. Focus presence is checked on search, card links and column checkboxes.

Screenshot inspection of the 834px Cinematic table and 390px Light cards found and corrected vertical-letter header compression and opaque unchecked card controls. Long names wrap, missing photos remain neutral placeholders, and horizontal table scrolling stays within the list. Generated `table-selected.png`, `cards-missing-photos.png` and search screenshots live in Playwright's test-results directory; use the HTML reporter to retain the contrast attachments alongside them.

## Fixes from verification

- `data-table.vue`: persist ordered leaf columns rather than definition-ordered `getAllColumns`; explicitly restore table-settings focus to the actual opener (top or bottom controls).
- `data-table-controls.vue`: emit the opener event to the owning table.
- `table-view.vue`: use shared muted/foreground tokens for headers. Before correction, header focus contrast was 1.74:1 in HomeBox and 2.29:1 in Light.
- `columns.ts`: preserve the header button's intrinsic width inside the existing horizontally scrolling table instead of squeezing words into vertical letters.
- `Item/Card.vue`: use shared background for unchecked selection and muted hover only when unchecked; checked/mixed states keep the shared checkbox defaults.

## Repeatable checks

Use the repository's E2E setup with registration enabled and an isolated SQLite database; the seeded test must not target production. Build the current frontend and serve it with SPA fallback and the backend at the same origin. During this run the generated static frontend was served at localhost:3003 with HTTP API forwarding to localhost:7745. This forwarding did not support WebSockets; these tests exercise HTTP refreshes, not event-stream delivery.

From `frontend/`:

```sh
pnpm install --frozen-lockfile
pnpm run build
E2E_BASE_URL=http://localhost:3003 pnpm exec playwright test -c test/playwright.config.ts \
  test/e2e/ui-revamp-inventory.browser.spec.ts test/e2e/inventory-search.browser.spec.ts \
  --project=chromium --workers=2 --retries=0 --reporter=html
pnpm exec vitest --run --config test/vitest.config.ts \
  test/inventory-views.test.ts composables/use-preferences.test.ts lib/data/themes.test.ts
```

Build passed (`NODE_OPTIONS=--max-old-space-size=1400` used to stay within sandbox memory). Targeted Vitest: **23 tests passed**. Targeted ESLint on the modified source/test files passed with zero warnings. `git diff --check` passed.

Full `pnpm run lint:ci` remains red on unrelated formatting warnings:

```text
locales/zh-TW.test.ts 12:17 warning prettier/prettier
pages/location/[id]/index/index.vue 239:17 warning prettier/prettier
ESLint found too many warnings (maximum: 1).
```

Full `pnpm run typecheck` remains red with repository-wide type errors, including:

```text
components/Item/View/table/data-table-dropdown.vue(64,35): error TS2304: Cannot find name 'ItemSummary'.
components/Item/View/table/data-table-expanded-row.vue(47,30): error TS2339: Property 'location' does not exist on type 'EntitySummary'.
plugins/i18n.ts(33,14): error TS2345: Argument of type 'I18n<Record<string, any>, {}, {}, string, false>' is not assignable to parameter of type 'Plugin<unknown[], unknown[]>'.
```

These files/type migrations are not changed by this story. Earlier runs also encountered a mismatched installed Playwright browser, a static server without SPA fallback, stale server working directories after rebuild, build exit 137 under concurrent compilation, and a Go link failure (`mapping output file failed: no space left on device`) with tmpfs temporary files. Installing the pinned browser, rebuilding serially with a bounded Node heap, restarting the static server, and setting Go `TMPDIR` to disk resolved those setup failures; final browser/build/targeted-test results above are from the corrected setup.

## Limits / human follow-up

No Firefox/WebKit, physical touch device, screen-reader announcement audit, other legacy themes or WebSocket reconnect validation was performed. Maintenance/location/tag mutation success and multi-tab/export content are not end-to-end asserted here: their selected-menu availability is checked, while real mutation coverage is duplication/deletion. No claim is made that a dashboard illustration approves inventory layouts. Global lint/typecheck require separate cleanup, not an inventory-only expansion of this story.
