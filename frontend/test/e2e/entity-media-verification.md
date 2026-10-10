# Phase 4 story 3 — truthful record media

## Scope and behavior

- Item gallery, image dialog, attachment lists, editor attachment panel, photo picker/previews and drop zones use shared theme surfaces without filters, blending, opacity effects or decorative image transformations. Images keep their aspect ratio. Explicit user-requested preview rotation remains available.
- The viewer has a translated title and named delete/download/close controls, a link to the authenticated original for native browser zoom, an unavailable-image message, and focus return to the initiating control. Original URLs, attachment tokens, classification types and API payloads are unchanged.
- Missing photos show a neutral `No photos added` state. Non-photo records keep readable filenames, neutral icons and existing download/external-link behavior. Long titles wrap; media controls remain available at 390px.
- Photo pickers have unique label associations and visible keyboard focus. Preview tooltips no longer nest buttons; primary selection exposes its pressed state. Attachment classification labels target the actual select trigger; editor cancellation/update returns focus.
- Fixed failed attachment updates: reset the loading state and show the existing update-error translation rather than the deletion error. Draft title/type/primary remain intact for retry.
- Fixed partial create/upload failure: retain the saved entity ID, input values and only failed photo previews. Explain that the record already exists, freeze saved metadata, and retry uploads against that record (never POST a duplicate entity). Removing failed photos lets the member continue. Successful create/add/navigation behavior and request mapping remain unchanged. This recovery state is in-memory; refreshing the application does not persist local upload files.
- No schema/storage/API or permission changes. Phase 3 is not merged in this queue, so existing child inventories were preserved. The referenced design is dashboard-only: its decorative camera treatment was deliberately not applied to record images.

## Automated verification

From `frontend/`, against generated static output served on localhost:3000:

- `pnpm exec vitest --run --config test/vitest.config.ts test/record-media.test.ts test/entity-forms.test.ts test/detail-surfaces.test.ts lib/data/themes.test.ts lib/datelib/dateOnly.test.ts lib/datelib/datelib.test.ts`: **45 passed**. Includes real template compilation, actual extracted create/update handlers, partial upload retry without duplicate records or duplicate successful uploads, input retention, payloads/validation, relationships and photo metadata/deletion.
- `pnpm exec playwright test -c test/playwright.config.ts test/e2e/ui-revamp-entities.browser.spec.ts test/e2e/entity-forms.browser.spec.ts test/e2e/detail-surfaces.browser.spec.ts --project=chromium --reporter=line --retries=0 --workers=2`: **18 passed**. Cinematic/HomeBox/Light × 390/1440px. Covers detail/editor relationships, create/edit/cancel, save and upload failures/retries, attachment classification, delete confirmation cancellation and failure/retry, keyboard focus, original-image links/download attributes, preview rotation/deletion and partial create recovery. Server-saved theme GET fixtures survive reload. Tests use auth cookies and mocked API boundaries, not a live seeded backend; no shared collection is mutated.
- Original deterministic 96×64 PNG is served through the authenticated attachment URL; the decoded sample is `[220,30,40,255]`, dimensions remain unchanged, and computed image filter/opacity/blending are `none`/`1`/`normal`. Inspected the narrow viewer and editor screenshots: controls fit, filenames wrap, no image effects. Screenshots are emitted to ignored Playwright test-results output.
- `pnpm run build`: **passed**.
- `pnpm run lint:ci`: **passed** with existing warning `locales/zh-TW.test.ts:12` (prettier).
- `pnpm run typecheck`: **fails with 195 pre-existing diagnostics**. Stashed the entire story and ran unchanged HEAD with the same dependencies; restored it and compared diagnostic messages after excluding line/column offsets: no added or removed diagnostics. Examples: `components/Entity/CreateModal.vue: error TS18048: 'form.location' is possibly 'undefined'.` and `components/Item/View/ItemChangeDetails.vue: error TS2339: Property 'location' does not exist on type 'EntitySummary'.`

## Measured contrast

Browser-resolved theme tokens, same results at both widths. Tests attach complete ratio JSON and assert 4.5:1 text / 3:1 focus and input boundaries.

| Theme | Main text/card | Muted text/card | Primary text/background | Primary button text | Destructive button text | Focus ring/card | Input/background |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Cinematic | 14.29 | 8.78 | 13.87 | 11.14 | 7.26 | 11.85 | 4.73 |
| HomeBox | 12.63 | 12.63 | 6.71 | 5.99 | 6.03 | 6.71 | 4.42 |
| Light | 14.68 | 14.68 | 7.45 | 7.45 | 6.03 | 7.45 | 4.42 |

Text/popover and muted/popover also meet 4.5:1. These are representative media/form tokens, not an audit of every legacy theme. Firefox/WebKit, live backend attachment storage/download byte transfer, actual account theme persistence, and arbitrary real-world media formats were not exercised in this story's browser run.
