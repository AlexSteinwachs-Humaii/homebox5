# Retained CSV lifecycle (story 3)

CSV artifacts live in `csv_exports`, not the portable ZIP table list. The existing ZIP topics, backup/import routes and seven-day purge are unchanged. `CSVExportService` is wired through `AllServices`; `csv-export-worker` sweeps immediately at startup and every five seconds. Database rows are the durable queue (no dependency on in-memory pubsub delivery).

## HTTP contract

Authenticated bases:
- `/api/v1/group/csv-exports`: trusted tenant from existing tenant middleware; current collection membership required.
- `/api/v1/user/csv-exports`: account authentication plus user role only; **no tenant middleware**, ignores X-Tenant/default collection. Private artifacts always belong only to the authenticated requester.

Each base supports:
- `POST /`: strict `csvexports.Request` / generated `CsvRequest`, returns 202 with pending `Artifact` (generated `CsvArtifact`).
- `GET /`: `{ items: Artifact[] }` history, filtered by current source authorization, newest first.
- `GET /{id}`: current authorized metadata.
- `GET /{id}/download`: canonical `text/csv; charset=utf-8` bytes, safe ID-based attachment filename. No raw keys or signed URLs.
- `POST /{id}/retry`: 202; requester-only failed-to-pending transition. Completed exports cannot be regenerated. Local-save retries must reuse download, not this endpoint.
- `DELETE /{id}`: 204, idempotent. Private owner only; collection requester or collection owner with current source access. Running jobs return 409 rather than racing blob cleanup.
- `GET /retention`, `PUT /retention`: `{ days: positive whole integer }`. Collection updates require owner; private settings belong to the current user. Unknown/trailing fields rejected. Default 30, arithmetic-safe maximum inherited from the shared contract.

Artifacts use the standalone generated types in `frontend/lib/api/types/csv-contracts.generated.ts`. The UI client and controls are later stories. All CSV responses use `Cache-Control: no-store`; the PWA has a higher-priority NetworkOnly rule for both bases. Errors are 403 access, 404 unavailable/out-of-scope ID, 409 not ready, 410 expired, 400 malformed request/policy, 500 sanitized generation/storage operation failure.

## Authorization and expiry

Source adapters are re-run against the current actor at initiation, generation, listing and download. This includes references to entities/invitations in private snapshots; invitation origins are derived from stored referenced IDs. Requester-member artifacts (notifiers) cannot expose one requester's source to another member. Private history never enters a collection query.

Completion time is captured only after canonical storage commits and source access is checked again. Expiry is completion plus the captured retention days, not creation or current policy. Downloads reject expiry before opening storage and again before returning its reader; asynchronous cleanup is not an access gate.

## Recovery and deletion

Pending claims atomically change to running and persist the deterministic, separately scoped CSV key **before any storage write**. Duplicate deliveries cannot claim a running/completed job. Each generation has a five-minute context deadline; running claims abandoned for ten minutes are returned to pending. Failed jobs retain request, requester, scope, captured policy and blob pointer, with generic public diagnostics; explicit retry uses the same artifact ID/key. Completed jobs always reuse the stored bytes.

Deletion atomically marks deleting, reloads the latest persisted pointer (the caller may hold a pre-generation snapshot), deletes the object, and only then removes the row. Missing blobs are success; storage/DB failures leave a deleting row for the next sweep. Exact deterministic key equality rejects traversal/foreign keys. Restrictive identity FKs remain unchanged. User/group service deletion purges their CSV objects first; cleanup failure or an in-progress job aborts identity deletion, retaining pointers. Concurrent creates cannot bypass the restrictive FK safeguard.

Generation currently materializes source rows/CSV in memory, as the source resolver from story 2 already does. History revalidation favors correctness over an optimized permission-only query; it may be expensive for large inventories. Real PostgreSQL execution and native browser save-picker tests remain later verification work; these tests exercise SQLite and both memory/failure-injected and file storage.
