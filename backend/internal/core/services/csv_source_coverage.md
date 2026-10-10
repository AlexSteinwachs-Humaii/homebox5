# Server CSV source coverage / capture contract (story 1)

`CSVSourceCatalogue()` in `csv_sources.go` is the executable matrix: it supplies
**every permitted column and order column**, scope, authorization class, adapter
ID, supported filter families and required context/selector/recent boundary. The
following table maps that catalogue to verified repository/list consumers. Adapter
IDs are typed dispatch contracts, **not yet executable row implementations**;
stories 3–4 provide those. No CSV endpoints, persistence or worker are introduced
by this story. Generated public API contracts belong to story 8.

## Coverage

All rows use all-results semantics (no page, pageSize, limit, offset or virtual
window). Collection sources require an explicit collection ID; there is no default
collection fallback. Private and cross-collection sources forbid that field and
use requesting-user-only history. A cross-collection source means the user's
currently authorized collections, not a caller-supplied arbitrary tenant list.

| Source IDs | Typed adapter / existing row basis | Query semantics / required context | Authorization scope |
| --- | --- | --- | --- |
| `items`, `locations` | `entities` / `repo.EntitySummary` | Entity predicates; fixed item/location kind | Collection member |
| `dashboard-recent-items` | `entities` / `EntitySummary`; `frontend/pages/home/table.ts` | Captured recent membership, separate from entity predicates; dashboard currently displays newest 5 | Collection member; all captured IDs checked |
| `entity-inventory`, `entity-hierarchy` | `entities` / `EntitySummary`, `TreeItem`, `EntityPath` | Required entity root and descendants flag; entity predicates; preserve parent context | Collection member; root and descendants in collection |
| `tag-inventory` | `entities` / `EntitySummary` | Required tag root; descendants flag and entity predicates | Collection member; tag/entity references in collection |
| `tags`, `tag-hierarchy` | `tags` / `TagSummary` | Name/description text predicate; hierarchy requires tag root/descendants | Collection member; validated tag hierarchy |
| `entity-types` | `entity-types` / `EntityTypeSummary` | Name/description text predicate | Collection member |
| `templates` | `templates` / `EntityTemplateSummary` | Name/description text predicate | Collection member |
| `maintenance`, `entity-maintenance` | `maintenance` / `MaintenanceEntryWithDetails`, `MaintenanceEntry` | Scheduled/completed/both status; embedded list requires entity root | Collection member with entity access |
| `collection-members` | `members` / `UserSummary` from group members | Entire member list; project only ID/name/email, never password/token fields | Collection member (existing GET `/groups/members`) |
| `collection-invitations` | `invitations` / `GroupInvitation` | Entire invitation metadata list; **no invitation token/join credential** | Collection member (existing GET `/groups/invitations`); creation/deletion remain owner-only |
| `collection-notifiers` | `notifiers-metadata` / `NotifierOut` projection | Current user's notifiers in selected collection (`GetByUser`) | Collection member **and requesting user**; never group-wide substitution |
| `entity-attachments`, `entity-media` | `attachments-metadata` / `ItemAttachment` projection | Required entity root; metadata only; media adapter restricts to photo/media types | Collection member with entity access |
| `backup-history` | `backup-history` / `ExportOut` projection | Existing collection ZIP export/import jobs; omit `ArtifactPath`, raw errors and reusable download paths | Collection member (existing `/group/exports` GET); not a new ZIP exporter |
| `collection-csv-history` | `csv-history` / future CSV artifact metadata DTO | Collection CSV lifecycle history | Collection member plus captured-source permissions; prerequisite: story 2/6 DTO/repo |
| `private-csv-history` | `csv-history` / future CSV artifact metadata DTO | User-private CSV lifecycle history; **no selected collection** | Account-private / requesting user; prerequisite: story 2/6 DTO/repo |
| `account-api-keys` | `api-key-metadata` / `APIKeyOut` only | Current user's metadata; ID/name/createdAt/expiresAt/lastUsedAt only | Account-private / requesting user; no collection |
| `account-collections`, `collection-selector` | `user-collections` / `Group` from `GetAllGroups(userID)` | Name text filter; authorized collection records; selector also requires eligibility/exclusions | Cross-collection / requesting user; private history; no collection |
| `item-selector`, `location-selector`, `entity-selector` | `entities` / `EntitySummary` | Entity predicates; explicit server-verified eligible membership and exclusions; fixed kind for item/location | Collection member; originating operation/context must be validated |
| `tag-selector` | `tags` / `TagSummary` | Text predicate; explicit eligibility/exclusions | Collection member |
| `entity-type-selector` | `entity-types` / `EntityTypeSummary` | Text predicate; explicit eligibility/exclusions | Collection member |
| `template-selector` | `templates` / `EntityTemplateSummary` | Text predicate; explicit eligibility/exclusions | Collection member |

Entities unify items and locations; no separate location persistence is invented.
Asset lookup `frontend/pages/assets/[id].vue` resolves a record, not an additional
list. Label-generator's entity **record selection** is an item selector; its print
layout controls are excluded. Static navigation, themes, language options,
currency choice lists, arbitrary strings in generic select widgets, buttons,
checkboxes and controls are not sources/columns. A collection record's `currency`
is data, unlike a currency choice list.

## Query/projection contracts for subsequent adapters

- `CSVEntityFilters` mirrors the predicates in `repo.EntityQuery` without paging.
  Parent IDs and parent-item IDs filter parent relationships; tag IDs include
  descendants, negateTags changes inclusion to exclusion; field name/text-value
  pairs are ORed, other predicate groups are ANDed. Search uses the repository's
  case-insensitive name/description/serial/model/manufacturer/notes search.
  Asset IDs must parse to a positive repository asset ID; contradictory photo
  flags and empty negated-tag predicates are rejected. Every referenced UUID
  must resolve in the source scope before any query is accepted.
- `repo.QueryByGroup` defaults nil `IsLocation` to **items**, despite its DTO
  comment. Source adapters must pin item/location surfaces explicitly. Combined
  entity hierarchy/selector traversal requires a separate all-entity path, not
  accidental reuse of that default. `FilterChildren` means roots only. No
  predicate may be ignored for a surface; incompatible context/filter combinations
  must fail in authority/adapter validation, not silently produce a broader query.
- Non-entity text search is case-folded substring matching on name/description
  (name only for collections). Selectors currently use client fuzzy ranking;
  their explicit eligible IDs preserve the captured eligible universe. With no
  requested ordering, selectors preserve eligible-ID order after exclusions.
  Authority must verify provenance/operation constraints, not trust those IDs.
- Ordering is the ordered `CSVOrder` sequence, with only matrix order columns and
  `asc`/`desc`. Workers add ID as a deterministic final tie-breaker if absent.
  With empty ordering, entity defaults match existing list semantics: recent
  createdAt descending, otherwise name ascending; maintenance scheduledDate
  ascending; other sources name ascending where available, otherwise ID ascending.
  Requested order always wins. Existing entity `SortBy` is unused and `OrderBy`
  accepts only name/createdAt/updatedAt/assetId. The typed adapter must also implement the safe client table sorts explicitly enumerated by the matrix (quantity, insured, purchasePrice, location display name, archived); forwarding these to the existing repository would silently use name and is forbidden. No arbitrary SQL ordering is allowed.
- Hierarchy context (`kind`, root UUID, descendants boolean) is separate from
  filters. False means direct children/attached records; true means transitive
  descendants, with required parent IDs/context preserved. Maintenance and
  attachment roots identify the owning entity; structural validation rejects descendants
  for attachments/media and entity maintenance, which only support direct records. Tag/entity graph cycles or stale
  references must fail safely, never fall back to an unrestricted list.
- Columns are an explicit allowlist projection from the row DTO basis in the table,
  ordered exactly as captured. Nested relationships flatten through named columns
  (`parentId`, `location`, `entityType`, `tags`), never arbitrary object paths. Safe metadata
  adapters must not reflect the entire DTO: `APIKeyCreatedOut.Token`, Ent API-key
  hash, password/token data in user rows, notifier URL credentials, attachment
  paths/thumbnail objects and backup artifact paths/errors are **not** exportable.
  Matrix columns represent supported data projections; adding more requires a
  deliberate contract/allowlist change and security tests.

## Acceptance boundary and asynchronous semantics

`DecodeCSVCapture` bounds capture JSON to 1 MiB and rejects unknown/case-aliased or
duplicate keys, trailing JSON, unsupported sources/scopes, columns/sorts/filter
families, missing contexts and malformed IDs. It does **structural validation**,
not authentication. Call `ValidateCSVCapture` with authenticated user ID and a
mandatory `CSVCaptureAuthority` before enqueueing. No authority means failure.
Authority must verify current membership, per-user/owner/data restrictions and
**all** source/context/filter/selector/recent IDs; it must not trust client claims
of eligibility or role. Validate and serialize an immutable capture before job
submission; do not retain references to a mutable request. Generation/download
recheck current authorization; successful syntax validation is never permission.

Recent sets explicitly contain a capture time and record IDs. These are distinct
from pagination: dashboard pageSize=5 is only the UI's current means of choosing
its recent set. A future capture endpoint must freeze/verify those five IDs at
capture time, not rerun top-5 at generation time. Empty array means empty set;
missing/null membership is rejected. The worker may read current values within
that frozen membership only; new records do not enter it. Deleted/inaccessible
references fail safely. Selector eligible sets are likewise explicit (including
valid empty sets) and exclusions must be a subset; the supported eligibility
policy is `explicit-set`, not an unchecked caller expression. Server-verified
capture provenance for recent/selector sets is an explicit upstream prerequisite
for the API/adapter stories (e.g. server resolution or signed capture handles).

## Dependency evidence / prerequisites (not Feature containment)

The queue has only this enhancement and no earlier merged items. A repository
search found no named Phase 1–7 contract documents; no canonical Feature link
exists. The following are **control dependencies**, not invented Feature
attachments or assertions that entire phases have landed:

| Dependency | Evidence currently present | Explicit prerequisite / integration responsibility |
| --- | --- | --- |
| Phase 1 foundations | Unified Entity/EntityType/Tag/Template repos and Ent models; API DTOs; event bus/background services | Stable server list predicates, tenant context and typed DTOs; generated CSV API DTOs deferred to story 8 |
| Applicable Phase 2–7 tenant/role controls | `repo_authz.go` in-group helpers; collection membership/ownership routes and `service_group.go` | Capture authority implementation and current membership/data rechecks in stories 3/4/6; do not replace owner/user restrictions with generic tenant middleware |
| Applicable Phase 2–7 hierarchy/template controls | `TreeQuery`, `PathForEntity`, tag descendants; entity type default templates | Validated traversal/context and cycle/stale-reference behavior in story 3; no unscoped descendant fallback |
| Applicable Phase 2–7 private/account controls | `APIKeyOut` omits token; user-scoped notifiers; `GetAllGroups(userID)` | Source-specific private/current-user adapters in story 4; never use a default collection for private history |
| Applicable Phase 2–7 maintenance/media controls | `MaintenanceFilters`; entity maintenance and attachment repos | Safe source metadata projection and reference authorization in stories 3/4; URL/blob/credential exclusion |
| Applicable Phase 2–7 export lifecycle controls | Existing ZIP `ExportOut`, service/worker and collection-scoped repo | Independent CSV DTO/persistence/history, retry/retention policy and authorized byte serving in stories 2/5/6/7; existing ZIP seven-day cleanup unchanged |
| Recent/selector capture provenance | Dashboard pageSize=5; Item/Location/Tag/Template selectors use client filtering/eligibility | Server-verifiable boundary and selector-operation contracts absent today; authority must not accept raw client assertions. Resolve before independent create API is enabled |

No exact phase-to-Feature mapping is available. This matrix records applicable
controls and missing upstream contracts without guessing phase ownership. Adding
new sources must extend both the executable catalogue and the coverage test;
unsupported datasets must fail, never return empty success or silently widen.
