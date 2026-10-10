# Phase 4 story 1 — record details

## Scope

Item and location detail headers and the shared DetailsSection now consume Phase 1 semantic surfaces, typography, section spacing and focus treatments. DetailsSection is also used by other existing detail consumers; its presentation changes apply there without changing its props or slots. No Feature attachment was supplied for this work.

The referenced dashboard is a visual-language reference, not a detail-page specification. No hero art, dark-only palette, photo filters, new API, model or workflow was added. Existing BaseContainer/BaseCard foundation treatments are reused. The location wrapper already consumes BaseContainer and needed no change. Phase 3 is still queued, so existing ItemViewSelectable controls and refresh events are retained, not reimplemented. Creation/edit forms and attachment/photo controls remain for stories 2 and 3.

Existing detail groups, visibility conditions, tags, breadcrumbs, label operations, tabs, child inventories, quantity patching and destructive confirmation handlers are retained. Copy controls and quantity buttons no longer rely on hover opacity to be discoverable. Icon-only actions use translated accessible labels. Location heading text is explicitly shrinkable inside its flex heading; the browser test checks text bounds because the shell hides horizontal overflow.

## Verification

From `frontend/`:

- `pnpm exec vitest --run --config ./test/vitest.config.ts test/detail-surfaces.test.ts lib/data/themes.test.ts`: 10 tests passed. Real templates compile; SSR coverage checks zero/false, long names/values, date/currency/markdown/link rendering, and unchanged group/action bindings. Existing theme tests check cinematic token contrast and theme choice compatibility.
- `pnpm exec playwright test -c test/playwright.config.ts test/e2e/detail-surfaces.browser.spec.ts --project=chromium --workers=1`: 6 tests passed, 390/1440px in cinematic/homebox/light. Uses mocked API responses and a fixture session against the built app, not real backend persistence. Checks long text, zero quantity, keyboard increment, decrement, negative guard, visible duplicate action, cancellation of item/location deletion without DELETE, parent/child location links, create action visibility, edit navigation and quantity focus ring. Full-page screenshots are written to Playwright test outputs. Cinematic narrow item and desktop location screenshots were inspected; a clipped location heading caught in that review was fixed and rechecked.
- `pnpm run build`: passed (static Nuxt generation).
- `pnpm run lint:ci`: passed with the existing single `locales/zh-TW.test.ts` formatting warning.
- `pnpm run typecheck`: fails with 195 diagnostics. Re-running with the three modified production Vue files restored from HEAD produces the same 195 diagnostics (compared by path/code/message, ignoring line offsets); no added diagnostics. Representative output:

```text
pages/templates.vue(52,39): error TS2339: Property 'length' does not exist on type 'Ref<EntityTemplateSummary[] | undefined, EntityTemplateSummary[] | undefined>'.
plugins/i18n.ts(33,14): error TS2345: Argument of type 'I18n<Record<string, any>, {}, {}, string, false>' is not assignable to parameter of type 'Plugin<unknown[], unknown[]>'.
ELIFECYCLE Command failed with exit code 2.
```

Initial browser fixture runs failed because mocks omitted status metadata, full record fields and `/groups/all`; the fixtures were corrected without modifying production behavior. Broad backend persistence, form validation/failure, attachment operations, full localized flows, Firefox/WebKit and per-page measured contrast are not claimed by this story's checks.
