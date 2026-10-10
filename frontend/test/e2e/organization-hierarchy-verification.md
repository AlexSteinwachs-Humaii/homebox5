# Location and tag hierarchy refresh — story 1

## Scope and design interpretation

Uses the referenced New UI Revamp dashboard revision 3 as surface/typography guidance only. There are no approved organization artboards. Reuses existing `hb-panel`, `hb-focus`, semantic theme tokens, form/dialog primitives and Phase 3 selectable inventory view. No imagery, motion, backend changes, new counts, templates or maintenance work.

- Shared tree row spacing, native disclosure buttons with expanded state, focus styles and wrapping links.
- Native nested lists replace incomplete ARIA tree roles (the existing view does not implement ARIA tree arrow-key navigation). Enter/Space operate the disclosure buttons; links remain independent navigation targets. Row clicks still toggle branches.
- Visual indentation stops increasing after four levels so deep names retain width. The full nested DOM and every relationship remain intact; no nodes or ancestor links are removed.
- Tree state stores, UUID hashes, sorting, query serialization/restoration and show-items filtering are unchanged.
- Page controls have explicit translated names and use tooltip `as-child` to avoid nested buttons.
- Failed hierarchy requests show an alert instead of presenting a misleading empty list. Empty lists retain existing create actions.
- Tag detail keeps original counts/value, parent links, descriptions, custom color/icon, edit fields and delete confirmation. Mobile actions wrap onto their own row, leaving enough width for long headings.
- Parent-selector clearing is a separate focusable button rather than a button nested inside the combobox trigger. Search, selection and cycle filtering are unchanged.

## Verification

Run from `frontend/` after `pnpm install --frozen-lockfile`:

- `pnpm run build`: passed (Nuxt static generation).
- Changed-file ESLint with `--max-warnings 0`: passed.
- `pnpm exec vitest --run --config test/vitest.config.ts lib/utils.test.ts lib/data/themes.test.ts test/entity-forms.test.ts test/detail-surfaces.test.ts test/inventory-views.test.ts`: **46 passed** in five files.
- `E2E_BASE_URL=http://localhost:3001 pnpm exec playwright test -c test/playwright.config.ts test/e2e/organization-hierarchy.browser.spec.ts --project=chromium --workers=2 --reporter=line`: **21 passed**, no retries in final run. Served the generated `.output/public` using Python's HTTP server; deterministic API routes isolate tests from collection data. No live backend CRUD integration is claimed.

Browser coverage: cinematic/light/homebox saved themes, 390px and 1440px lists, Enter disclosure, expand/collapse-all, query restoration after reload, show-items restoration, long unbroken names, links, empty lists, failed loads, 14-level nested hierarchy, tag detail value, parent/self/descendant filtering, separate clear control, create retry, edit retry with retained color/icon/parent payload, canceled deletion, failed deletion and successful confirmed deletion. White custom icon background is verified to retain black foreground. Tag-detail mobile screenshots are written into Playwright's per-test output directory and were visually reviewed; that review caught the compressed-heading defect before the final build.

## Existing verification failures

`pnpm run typecheck` exits 2 with **195 existing diagnostics**. Re-ran on the unchanged baseline by temporarily stashing this story; normalizing only line/column coordinates yielded identical diagnostic multisets, with no added/removed diagnostics on this branch. Examples:

```text
components/Entity/CreateModal.vue(661,19): error TS18048: 'form.location' is possibly 'undefined'.
components/Item/View/ItemChangeDetails.vue(64,28): error TS2339: Property 'location' does not exist on type 'EntitySummary'.
components/Item/View/table/data-table-dropdown.vue(64,35): error TS2304: Cannot find name 'ItemSummary'.
plugins/i18n.ts(33,14): error TS2345: Argument of type 'I18n<...>' is not assignable to parameter of type 'Plugin<...>'.
ELIFECYCLE Command failed with exit code 2.
```

`pnpm run lint:ci` exits 1 because two unchanged files exceed the allowed one warning:

```text
locales/zh-TW.test.ts:12:17 warning prettier/prettier
pages/location/[id]/index/index.vue:239:17 warning prettier/prettier
2 problems (0 errors, 2 warnings)
ESLint found too many warnings (maximum: 1).
ELIFECYCLE Command failed with exit code 1.
```

No unrelated source files were changed to suppress these failures. Firefox/WebKit, live API integration and exhaustive translation/contrast audits are not part of this story's completed verification; the broader organization audit belongs to story 3.
