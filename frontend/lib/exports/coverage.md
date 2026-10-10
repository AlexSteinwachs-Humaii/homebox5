# CSV record-list coverage contract

Audited against `frontend/pages` and `frontend/components` (including every `v-for`, selector, tree and shared Item/View consumer). This is the **foundation inventory**, not a claim that buttons/adapters already exist. “UI integration” identifies the insertion point and the implementation story (S4–S8); all are pending. S2 implements source adapters, S3 authorization/lifecycle. No reporting dashboard is introduced.

`backend/internal/core/services/csvexports/registry.json` is the authoritative source/column/filter/access allowlist. `source-registry.generated.ts` and `../api/types/csv-contracts.generated.ts` are generated with `cd backend && go run ./app/tools/csvcontractgen` (also run by `task typescript-types`). `source-registry.ts` narrows UI descriptors to the source's scope, columns and boundary kinds. Changes require updating this matrix, regeneration and contract tests.

## Rules common to every row

- Full queries are not loaded-page exports. No page/pageSize/offset exists in the contract. Recent lists have an intentional `recent.limit`; explicit sets contain the **complete** matched ID set in display order, not the virtualization slice. Empty explicit sets are empty, never “all”. History captures IDs before adding its own CSV job.
- Columns are ordered `{id,label}` data columns from the allowlist. Tables use actual visible order; cards/trees/auxiliary records use the displayed field set. No checkbox/action/image bytes/blob path/authenticated URL/hidden credential column is eligible; the explicit private invitation-visible source is limited to the already-displayed one-time token. Full values, not visual truncation. `parentId/path` provide tree context independent of expansion.
- Collection source IDs come from trusted auth/tenant context, not request-supplied identity. Entity/template IDs are source constraints only and must be authorized. Private sources never acquire a group scope. Client-provided IDs/snapshots never confer permission.
- `member` means current tenant member. `requester-member` additionally means the requesting user's source rows only. `source-access` means each historical artifact's original source policy must still hold. `requester` is authenticated user only, independent of collections. `requester-membership` revalidates every included collection. `referenced-entity-access` revalidates every referenced record before copying displayed values into a privately retained CSV. `collection-owner` revalidates ownership of the visible invitation's collection.
- `snapshot` is bounded, schema-constrained displayed draft/external data. It is not a general object map or a database read instruction. Resolvers must verify visible invitations and referenced entities; draft photos only contain name/MIME/size, never file content.
- Retention policies default to 30 positive whole days. Collection policy updates require owner authorization; private updates use the authenticated user and work without a tenant. Policies are captured per request; expiry is fixed at completion + captured days. The arithmetic cap of 106751 days is a Go duration limit, not a business retention limit.

## Column sets

These are **eligible** columns, not instructions to export every field. Exact ordered displayed subsets are captured by each integration:

| Set | IDs |
| --- | --- |
| E (entity table) | assetId, name, quantity, insured, purchasePrice, location, archived, createdAt, updatedAt; cards use name, description, quantity, location, tags; trees add parentId/path |
| L (location) | name, itemCount; trees add parentId/path, description when displayed |
| T (tag) | name; hierarchy adds parentId/path and displayed counts |
| F (custom/template field) | name, type, value (id only if displayed) |
| A (attachment/photo) | title, type, mimeType, sizeBytes, createdAt (displayed subset; no payload/URL) |
| M (maintenance) | name, description, entityName, scheduledDate, completedDate, cost, status |
| H (CSV history) | source, status, createdAt, completedAt, expiresAt, sizeBytes |
| Z (ZIP/import history) | kind, status, createdAt, updatedAt, sizeBytes, error |

## Surface matrix

Each path is relative to `frontend/`. Shared component rows cover **every** caller, not just the example pages.

| Record-list surface | Source / boundary | Filters / eligibility / all-results rule | Columns | Access / retention | UI integration |
| --- | --- | --- | --- | --- | --- |
| `pages/items.vue` | entities / all | EntityQuery search, location/item/tag/parent IDs, field predicates, asset ID, photo/archived/root flags; capture effective multi-sort | E visible table order or card fields | member / collection | Item/View/Selectable section header, S4 |
| `pages/home/index.vue` recent table and mobile cards | entities / recent | Existing recent query is **five** records; capture sort and explicit limit 5 | E visible recent table or compact card fields | member / collection | Recent section header, S4 |
| Home location cards | locations / roots | Root-location set, not recent-item query | L | member / collection | Location section header, S4 |
| Home tag chips | tags / roots | Actual root/tag set; no inherited item pagination | T | member / collection | Tag section header, S4 |
| `pages/locations.vue`, `components/Location/Tree/{Root,Node}.vue` | locations / hierarchy | Search, show-items state; entire hierarchy independent of collapse; parent/path | L and displayed entity fields | member / collection | Root tree header, S4 |
| `pages/tags.vue`, `components/Tag/Tree/{Root,Node}.vue` | tags / hierarchy | Search, show-items state; whole hierarchy independent of collapse | T and displayed entity fields | member / collection | Root tree header, S4 |
| `pages/item/[id]/index.vue` child inventory | entities / children | entityId and parentItemIds; all children in captured order | E | member + entity access / collection | Child inventory header, S4 |
| `pages/location/[id]/index/index.vue` item inventory | entities / children | entityId, parentIds; existing tag/archived/search predicates | E | member + entity access / collection | Item inventory header, S4 |
| Location child-location cards | locations / children | entityId direct children, not all descendant items | L | member + entity access / collection | Child locations header, S4 |
| `pages/tag/[id].vue` item inventory | entities / all | tagIds and current list predicates; every page | E | member / collection | Item inventory header, S4 |
| `pages/assets/[id].vue` result cards | entities / all | assetId route boundary (may identify multiple records) | E card fields | member / collection | Results header, S4 |
| `components/Item/View/{Selectable,Table}.vue`, `table/{data-table,table-view,card-view,data-table-expanded-row}.vue` | parent's descriptor | Preserve page-owned query, column visibility/order and effective sorting; expanded tag chips are entity-tag records below | E | parent's collection source | Shared descriptor plumbing, S4; no per-row export buttons |
| `pages/templates.vue`, `components/Template/Card.vue` | templates / all | Search/matched order if present | name, description | member / collection | List header, S5 |
| `pages/template/[id].vue` saved template fields | template-fields / children | templateId; complete fields | F | member + template access / collection | Fields header, S5 |
| Template detail/create edit default-field drafts, `components/Template/CreateModal.vue` | draft-fields / snapshot | Current unsaved repeated field rows in display order | F | requester / private | Draft fields header, S6 |
| `components/Maintenance/ListView.vue`, `pages/maintenance.vue`, `pages/item/[id]/index/maintenance.vue` | maintenance / all or children | scheduled/completed/status predicate; entityId for per-item; actual sort | M | member + entity access when constrained / collection | Shared ListView header, S5 |
| `pages/collection/index/members.vue` | members / all | Current collection members | name, email | member / collection | Table header, S5 |
| `pages/collection/index/invites.vue` stored metadata | invitations / all | Current collection; no reconstruction of hashed tokens | expiresAt, remainingUses | member / collection | Table header, S5 |
| Invitation creation/visible merged token rows in invites table | invitation-visible / snapshot | Displayed one-time token only, verify originating collection and ownership; no hash-to-token synthesis | token, expiresAt, remainingUses | collection-owner / private | Table descriptor variant, S5/S6 |
| `pages/collection/index/notifiers.vue` | notifiers / all | Current user AND collection, not all members' notifier rows | name, isActive, createdAt | requester-member / collection | List header, S5; credentials/notification URL excluded |
| `pages/collection/index/entity-types.vue` | entity-types / all | Actual current tenant types | name, isLocation, defaultTemplate | member / collection | Cards header, S5 |
| `pages/collection/index/tools.vue` ZIP/import history | zip-history / all or explicit | Existing history, filtered when applicable | Z | member / collection | Separate ZIP history header, S5; never change restore/seven-day cleanup |
| Future collection CSV history in tools | collection-csv-history / explicit | Capture authorized history IDs **before** creating export; revalidate each source | H | source-access / collection | Export/ExportHistory, S8 |
| `pages/profile.vue` API-key list | api-keys / all | Authenticated user's metadata only | name, createdAt, lastUsedAt, expiresAt | requester / private | API keys header, S5; no token/hash/one-time secret; current API has no prefix |
| Future account CSV history in profile/no-collections | private-csv-history / explicit | Complete authenticated user's prior history snapshot; no tenant needed | H | requester / private | Export/ExportHistory, S8 |
| `components/Collection/Selector.vue` (shell/all callers) | collections / eligible or explicit | Fuzzy search over complete memberships, displayed order, no other users' collections | name, role if displayed | requester-membership / private | Stable picker header action, S6 |
| `components/Item/Selector.vue` (create/edit/bulk/labels callers) | entities / eligible | Caller-owned full server query; exclude current item/descendants per caller, no visibleItems virtual slice | name, assetId and displayed context | member / collection | Picker header, S6 |
| `components/Location/Selector.vue` (all callers) | locations / eligible or explicit | Fuzzy search complete eligible hierarchy; current-location/cycle exclusions, displayed order | name, parentId/path | member / collection | Picker header, S6 |
| `components/Tag/{Selector,SingleSelector}.vue` (all callers) | tags / eligible or explicit | Fuzzy matching and parent/current/selected exclusions; complete matched set | name, parentId/path | member / collection | Picker header, S6 |
| Selected tag chips in Tag/Selector | tags / explicit | Current selected ID set, not available-options set | name | member / collection | Selected-values descriptor, S6 |
| `components/Template/Selector.vue` (compact and dialog modes/all callers) | templates / eligible or explicit | Complete fuzzy matches and caller eligibility, visible order | name, description when displayed | member / collection | Stable selector header, S6 |
| `components/Entity/Selector.vue` (all create/edit callers) | entity-types / eligible | isLocation eligibility of caller; full type list | name, isLocation | member / collection | Selector header, S6 |
| Item/location photo galleries; `components/Item/{AttachmentsList,ImageDialog}.vue` | attachments / children or explicit | entityId; type=photo for photos; complete attachment set or explicit gallery subset | A | member + entity/attachment access / collection | Gallery/list header, S6; image dialog is same gallery, no duplicate source |
| Item/location edit attachment rows | attachments / children | entityId and actual displayed type filter | A | member + entity/attachment access / collection | Attachments editor header, S6 |
| `components/Form/{PhotoUploader,PhotoUploaderPreview}.vue` draft previews (all callers) | draft-photos / snapshot | Current selected local photos, no file bytes/base64/blob URLs | name, mimeType, sizeBytes | requester / private | Preview header, S6 |
| Item/location saved custom fields in `components/global/DetailsSection/DetailsSection.vue` consumers | entity-fields / children | entityId; repeated custom field records only | F | member + entity access / collection | Custom field section header, S6 |
| Item/location edit custom field drafts; `components/Entity/CreateModal.vue` template-derived drafts | draft-fields / snapshot | Current unsaved field rows including edits | F | requester / private | Draft fields header, S6 |
| Item/location tag chips, Item/Card and expanded table tags | entity-tags / children | entityId; full associated tags (not only truncated chip display); hierarchy context | name, parentId/path | member + entity access / collection | Owning record-list/section descriptor, S6 |
| Item/location/tag breadcrumb ancestor records | entity-path or tags / explicit | Captured ancestor IDs/order; validate referenced records; do not export navigation controls | name, parentId/path | member / collection | Detail context descriptor, S6 |
| `pages/scanner-ar.vue`, `components/Scanner/{ARView,ARControls,AROverlayCard}.vue` detections | scanner-results / explicit | Complete currently detected resolved entity IDs in display order; no camera frames | name, assetId, quantity, location, purchasePrice, childItemCount, locationChildCount, totalPrice, parentId/path (displayed subset) | referenced-entity-access / private | AR results header, S6 |
| AR overlay child-record previews | scanner-results / explicit | **Explicit three-child preview boundary**, not all child items; referenced entity checks | name, assetId, quantity | referenced-entity-access / private | Overlay descriptor, S6 |
| `components/Item/BarcodeModal.vue` external product-search results | product-results / snapshot | Complete returned product set, actual displayed order; no imageBase64/source credentials | name, manufacturer, modelNumber, searchEngineName | requester / private | Search results header, S6; additional audited surface |
| `pages/reports/label-generator.vue` generated record output | label-records / explicit | Complete referenced record dataset; exclude blank skip positions and blank pre-printable placeholders beyond inventory; preserve the actual generated-record order | name, assetId, location when printed | referenced-entity-access / private | Screen controls, S6; export stays out of print media |
| `components/Search/Filter.vue`, `pages/items.vue` custom-field name/value options | entity-fields / eligible | Distinct authorized field names/values from current collection, search and current field-name predicate; not static choices | name, value | member / collection | Filter record-options header, S6; additional audited surface |

## Explicit exclusions (not unexplained omissions)

- `App/QuickMenuModal`, page tab loops, sidebar/header/navigation, theme/language/currency/icon/color choices, date/calendar controls, camera-device choices: static configuration or navigation, not product record lists.
- Home/maintenance statistics: aggregate scalar metrics, not rows of records. No new report source.
- Table column settings, pagination items, sort/status/attachment-type options, bulk-edit toggles: view controls, not source records. Print propertyInputs/layout knobs/blank placeholders are excluded.
- `DetailsSection` main/purchase/warranty/sale properties and edit mainFields/purchaseFields/warrantyFields/soldFields: fixed properties of **one** entity, not repeated records. Custom fields are explicitly included above.
- `pages/a/[id].vue` and `pages/label/[id].vue` are compatibility redirects; `components/global/LabelMaker.vue` and QR/barcode renderers show a single label image, not a repeated record list. Report label record output is covered above. ImageDialog has no separate record query, and its gallery descriptor is covered above.
- Display-only Tag/Chip, Item/Card, Location/Card, Template/Card and tree Nodes belong to the containing list. No action inside each selectable row, no duplicate export per child. Associated tag/breadcrumb/AR-child records have explicit rows above.

## Lifecycle handoff

Dedicated `csv_exports` rows record requester, optional collection (NULL for private), source policy, immutable request snapshot/retention, status and server-only artifact path, completion and fixed expiry. No CSV row appears in `exports` or portable ZIP archives. Foreign keys restrict deleting a requester/collection while a CSV blob pointer remains; S3 must remove blobs before rows before user/group deletion. No public CSV route or generation worker is registered in this foundation story; therefore no artifact can be created through the existing API before authorization/lifecycle is implemented. Swagger's existing live-route contracts are intentionally unchanged. The separate generated CSV API contracts are ready for S3's endpoint integration.
