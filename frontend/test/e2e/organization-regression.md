# Phase 5 organization regression

Run from `frontend/` after `pnpm run build`, against a server serving `.output/public` with SPA fallback (or navigate to the built `/maintenance` route before reloading when using Python's static server):

```sh
E2E_BASE_URL=http://localhost:3001 pnpm exec playwright test -c test/playwright.config.ts test/e2e/organization-hierarchy.browser.spec.ts test/e2e/template-management.browser.spec.ts test/e2e/ui-revamp-organization.browser.spec.ts --project=chromium --workers=2 --reporter=line
pnpm exec vitest --run --config test/vitest.config.ts lib/data/themes.test.ts lib/datelib/dateOnly.test.ts lib/utils.test.ts
```

These browser specs intercept the normal API endpoints with deterministic records; they verify frontend routing, request scopes/payloads, refreshes and failure handling, not database persistence or server permissions. No backend changes were made. Firefox/WebKit and a real authenticated backend pass are still useful release checks.

## Coverage

- Hierarchy suite: nested location/tag links, item visibility, persisted/query tree state, deep/long names, empty/failed lists; custom tag colors/icons, parent/cycle filtering, clear selection, tag create/update/delete failures and confirmations.
- Template suite: create/duplicate/edit/delete, associations, warranty/custom-field defaults, NIL_UUID payloads, template application in the existing item selector, loading, validation/error retention and confirmations. Create-submit regression uses the keyboard to avoid a toast overlapping the mobile pointer target.
- Organization suite: global and item-scoped maintenance GETs, status filters and pressed state, entry counts/cost/date display, complete status/date payload, create/edit failure retention and retry, disabled saving with aria-busy, duplicate defaults/Escape, canceled/failed/successful deletion, restoration to the originating control or the toolbar after deletion.
- Maintenance matrix: cinematic, saved light and saved HomeBox themes at 390/834/1440px; long unbroken names, translated-label stress, dialog and document overflow checks, saved theme after reload. Screenshots include long-name and normal lists.
- Contrast tests resolve CSS tokens in the browser and attach measured ratios: surface/body/muted/primary/destructive-button/popover text >= 4.5:1; focus/input boundaries >= 3:1. Red text on saved light/HomeBox popovers measured only 2.748:1, so maintenance inline error copy uses popover foreground and a destructive border instead. These checks are not a claim about every arbitrary user-selected tag color or every legacy theme.

## Verification in this story

- `pnpm run build`: passed.
- Combined Chromium suites: 41 passed (2.1m). After adding explicit count/date assertions, maintenance/contrast suite: 12 passed (48.0s).
- Vitest suites above: 27 passed.
- Targeted ESLint on changed Vue files/specs: passed with `--max-warnings 0`.
- `pnpm run lint:ci`: failed on two existing Prettier warnings (`locales/zh-TW.test.ts:12`, `pages/location/[id]/index/index.vue:239`), zero errors; maximum permitted is one warning.
- `pnpm run typecheck`: failed before reporting TS diagnostics with `[Vue] Load plugin failed: vue-router/volar/sfc-route-blocks`. The installed vue-router package has no `volar/` directory. Prior story's 195-error baseline was not re-established in this environment.

## Maintenance implementation notes

List failures return an empty renderable array instead of passing the HTTP error body to statistics. Refresh preserves populated cards so save-close focus can return to an existing trigger. Empty/deleted lists restore focus to the status toolbar. EditModal captures the opening element and restores it through Reka's close-auto-focus event. Failed creates keep the form open; partially successful multi-item creates retain only failed target IDs for retry, avoiding duplicate successful entries. Global duplicate now uses the entry's itemID instead of an undefined per-item prop. Existing cost coercion, date-only strings, formatting and completion rules remain unchanged.
