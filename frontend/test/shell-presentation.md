# Application shell presentation (Phase 1, story 3)

## Implementation boundaries

- Reuses the existing sidebar state/provider, navigation arrays, collection API/reload path, permissions and dialog dispatch. Existing icons and HomeBox artwork are retained rather than copying the artboard. No dashboard hero or domain-form redesign is included.
- Uses theme-aware sidebar, background, popover, border and focus tokens, with smaller navigation typography, a framed brand mark, collection selector and primary Create control.
- Keeps `--header-height: 4rem` and `--header-height-mobile: 7rem` unchanged, including the legacy-header breakpoint; Phase 4 sticky edit controls need no offset change.
- Labels can wrap in sidebar/collection/command controls. Dialogs use viewport gutters, scrolling and wrapping titles/footers. Reka/Vaul still own focus containment, Escape and focus return.
- The inline/portal DialogContent wrapper now explicitly forwards native listeners. Quick-menu Escape is captured before the combobox can consume it. Quick-menu click and number-key creation share parameterized dispatch so Location remains Location rather than opening an unparameterized entity dialog.
- New sidebar-toggle label is in English localization; existing i18n fallback supplies it for languages without that key. Close/search/scanner reuse existing keys.

## Verification

- `pnpm run build`: passed (static Nuxt generation).
- `pnpm run lint:ci`: passed with the existing single Prettier warning in `locales/zh-TW.test.ts:12`.
- Explicit ESLint over changed UI primitives: zero errors; two existing optional-prop default warnings for SidebarMenuButton tooltip/hotkey.
- `pnpm exec vitest --run --config test/vitest.config.ts composables/use-preferences.test.ts lib/data/themes.test.ts`: 2 files / 13 tests passed.
- `E2E_BASE_URL=http://localhost:3001 pnpm exec playwright test -c test/playwright.config.ts test/e2e/shell-presentation.browser.spec.ts --project=chromium --workers=1 --retries=0 --reporter=line`: 3 tests passed. Ran against generated frontend and an isolated demo Go API with a temporary same-origin preview proxy. Tests fixture GET settings and intercept PUT settings to avoid changing shared-account preferences.
- Browser coverage: existing destinations and active state, expanded/collapsed sidebar, search submission, keyboard quick menu, Tab containment/Escape/focus return, Location action, 390px collection creation drawer with long title, saved light theme/legacy header, scanner dialog, Ctrl+B and sign-out.
- Desktop cinematic screenshot inspected against approved revision 3: shared shell hierarchy is reproduced; existing dashboard data/composition remains untouched.
- `pnpm run typecheck`: fails with exit 2. Compared against HEAD before this story: both have the same 100 distinct file/error-message pairs (ignoring shifted line numbers), with no added errors. Examples: `layouts/default.vue: Property 'build' does not exist on type 'Ref<APISummary | undefined, APISummary | undefined>'`; `pages/templates.vue: Type 'ComputedRef<string>' is not assignable to type 'ResolvableTitle'`; `plugins/i18n.ts: I18n ... is not assignable to Plugin ...`.

## Remaining phase-wide verification (story 4)

Owner/member visibility and real multi-collection switching, exhaustive destinations/Create actions, confirmation dialogs and translated control labels, tablet/other browser engines, measured contrast and real camera permission/hardware behavior remain for the dedicated regression story. No schema, route, permission or scanner media-access changes were made here.
