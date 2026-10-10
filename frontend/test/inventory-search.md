# Inventory search — story 1 verification

## Scope

Restyles `pages/items.vue` and `components/Search/Filter.vue` using Phase 1's `hb-panel`, `hb-section-title` and the existing shadcn Input/Button/Select/Switch/Checkbox/Popover components and active-theme tokens. No new theme palette, backend work, table/card-control changes or approved inventory artboard is implied. The referenced dashboard is visual guidance only; the existing inventory composition remains.

Search and filter labels remain the same. Added explicit label associations, a named delete-field button, selector-search accessible names and a translated selector-empty message (English, with normal locale fallback). Loading and existing failed-search feedback now have inline status/alert presentation; the existing toast remains. Empty inventory output is hidden while an empty request is loading or has failed, without unmounting the selectable view or changing its state.

`useOptionalRouteQuery`, mount hydration/locks, field-value cache, parsing, watchers/debouncing, request parameters, router.push/default omission, pagination and the existing Reset Search implementation are unchanged. The only new script state is the failed-request presentation flag. This deliberately does not redesign Reset Search or expand route-history hydration beyond the existing implementation.

## Checks run

- `pnpm run build`: passed (Nuxt static generate). Existing duplicate-component and chunk-size warnings remain.
- `pnpm exec eslint pages/items.vue components/Search/Filter.vue test/e2e/inventory-search.browser.spec.ts`: passed.
- `pnpm exec vitest --run --config ./test/vitest.config.ts composables/use-preferences.test.ts test/detail-surfaces.test.ts test/entity-forms.test.ts`: **3 files, 29 tests passed**.
- `pnpm exec playwright test -c test/playwright.config.ts test/e2e/inventory-search.browser.spec.ts --project=chromium --workers=2 --reporter=line`: **8 passed** against the generated static site with mocked API responses and fixture session cookies. This is a frontend regression walk, not live backend/auth integration verification.

Browser checks cover cinematic/light/homebox at 390px and 1440px: combined initial URL hydration and reloading, location/tag counts, field/value labels and API request fields, archived enabled/checked state, mutually exclusive photo switches by keyboard, tag clearing, selector-empty feedback, field-selector removal, query clearing, query back/forward, invalid asset text, loading/empty/failure distinction, visible search focus and viewport overflow. Additional German narrow-width check covers translated options and long unbroken location names with screenshots. Desktop and mobile cinematic screenshots were inspected. Table/cards, bulk selection, every filter's history navigation, live backend scenarios and exhaustive contrast/cross-browser checks remain outside this focused story's test coverage (later stories own broad verification).

## Repository-wide failures

`pnpm run lint:ci` failed due to two unchanged warnings (the project allows at most one):

```text
frontend/locales/zh-TW.test.ts
  12:17 warning ... prettier/prettier
frontend/pages/location/[id]/index/index.vue
  239:17 warning ... prettier/prettier
ESLint found too many warnings (maximum: 1).
```

`pnpm run typecheck` exited 2 with **195 diagnostics**. Restoring both modified Vue files to HEAD, running typecheck again, and then restoring the work produced the same 195 diagnostic kinds/messages (line numbers normalized), with no added or removed diagnostics. Example existing diagnostics:

```text
pages/items.vue(506,57): error TS2322: Type 'true | string[] | undefined' is not assignable to type 'PropertyKey | undefined'.
pages/items.vue(506,70): error TS2322: Type 'true | string[] | undefined' is not assignable to type 'AcceptableValue'.
pages/templates.vue(20,5): error TS2322: Type 'ComputedRef<string>' is not assignable to type 'ResolvableTitle'.
plugins/i18n.ts(33,14): error TS2345: Argument of type 'I18n<Record<string, any>, {}, {}, string, false>' is not assignable to parameter of type 'Plugin<unknown[], unknown[]>'.
```

The inventory diagnostics refer to the unchanged `allFields` async-data template typing. No unrelated fixes were included.
