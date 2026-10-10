# CSV inventory integration — story 4

## Scope and checks

Implemented items, home recent/location/tag sections, location/tag trees, item child inventories, location item/child-location lists, tag inventories and asset-result cards. Descriptors capture source predicates, effective sort and ordered visible table columns; compact/full card fields and hierarchy path/context are explicit. No export request contains list pagination or selected/rendered row IDs. Other record surfaces, native save-picker workflow, settings/history and whole-enhancement verification remain S5–S9.

The shared action uses existing buttons/panels and the current theme. The inspected revision-3 dashboard design has no CSV flow; it was not edited. A 390px cinematic dashboard export screenshot was inspected: section order preserved, copy/buttons fit, no horizontal overflow. Native picker/device and other-browser checks remain later stories; this is not their sign-off.

## Final verification

- `cd backend && go test -count=1 ./internal/core/services/csvexports ./internal/data/repo ./internal/core/services ./app/api` — all four packages passed. `repo_csv_inventory_views_test.go` decodes an embedded-list request, resolves 55 seeded matching children, checks CSV byte/header ordering (all children, not a page), root/archive semantics, show-items boundaries and nested direct-parent labels/context.
- `cd frontend && pnpm exec vitest --run --config ./test/vitest.config.ts lib/exports/inventory.test.ts lib/exports/source-registry.test.ts lib/api/classes/csv-exports.test.ts` — 3 files, 13 tests passed.
- `cd frontend && pnpm run build` — passed (`nuxt generate`). `cd backend && go build -o /tmp/csv-inventory-api ./app/api` — passed.
- `cd frontend && pnpm run lint:ci` — passed, 0 errors and one existing warning in `locales/zh-TW.test.ts:12`.
- `cd frontend && E2E_BASE_URL=http://localhost:3000 pnpm exec playwright test -c ./test/playwright.config.ts test/e2e/csv-inventory.browser.spec.ts --project=chromium --workers=2 --reporter=line` — 11 passed. The frontend generated build was served with SPA fallback; authenticated API route fixtures test descriptors/UI/canonical download bytes, not a live backend. Server seeded-record checks are separately covered above.
- `cd frontend && pnpm run typecheck` — **fails with existing diagnostics**. Unchanged HEAD was checked using a temporary stash: baseline 198 diagnostics, current 195; comparison normalizing shifted line numbers found **no new diagnostics**. Representative output:

```text
pages/templates.vue(73,10): error TS2322: Type 'true | EntityTemplateSummary[] | undefined' is not assignable to type 'EntityTemplateSummary'.
plugins/i18n.ts(33,14): error TS2345: Argument of type 'I18n<Record<string, any>, {}, {}, string, false>' is not assignable to parameter of type 'Plugin<unknown[], unknown[]>'.
ELIFECYCLE Command failed with exit code 2.
```

## Corrected failures and integration findings

- Initial browser run: `expect(locator).toBeVisible() failed` for Download CSV. Workbox NetworkOnly requests bypassed Playwright page routing and reached the static SPA fallback. Fixtures now explicitly block service workers; production keeps CSV NetworkOnly behavior.
- Fixture corrections: exact selection label is `Select Row`, all visibility preferences must be specified (unspecified TanStack columns remain visible), and the existing header first click is ascending. Final browser run passes without retries.
- Rebuilding removes `.output/public`; a static server whose current working directory is inside it must restart. An intermediate run returned `page.goto: net::ERR_SOCKET_NOT_CONNECTED`; preview logs showed `FileNotFoundError: [Errno 2] No such file or directory` in `os.getcwd()`. Restarting the preview resolved this test-environment failure.
- Initial standalone Prettier used defaults rather than the repository's ESLint formatting settings. Touched files were corrected with targeted `eslint --fix`; final lint passes.
- Integration exposed two source mismatches: home roots must exclude archived locations, and inventory-summary location labels use the direct parent rather than detail-view nearest-location resolution. Both are fixed narrowly in `repo_csv_sources.go` with regression tests; scanner/detail projections, authorization, retention and ZIP lifecycle are untouched.
