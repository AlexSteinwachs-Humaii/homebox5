# CSV lifecycle APIs

This capability is independent of list UI and existing ZIP exports. All routes
require authenticated user-role access. Collection routes additionally validate
the selected tenant using the existing tenant middleware. Private routes do not
use tenant middleware, require no collection/default membership, and ignore a
supplied tenant header rather than interpreting it as authority.

| Operation | Collection | Private / cross-collection |
| --- | --- | --- |
| Create | POST `/api/v1/group/csv-exports` | POST `/api/v1/users/self/csv-exports` |
| History | GET same base | GET same base |
| Status | GET base + `/{id}` | GET base + `/{id}` |
| Download | GET base + `/{id}/download` | GET base + `/{id}/download` |
| Explicit retry | POST base + `/{id}/retry` | POST base + `/{id}/retry` |
| Delete | DELETE base + `/{id}` | DELETE base + `/{id}` |

Create accepts only the typed `CSVCaptureRequest`, decoded by the strict bounded
capture decoder. Collection capture IDs must equal the selected tenant. Private
and cross-collection captures must not contain a collection ID. Roles, blob keys,
unknown properties, unknown source IDs and unauthorized columns are not accepted.
Selector/recent exports still require the existing trusted boundary authority;
these routes do not create trust from caller-supplied eligible/recent IDs.

Create returns 202 with `CSVArtifact`. Status returns that same safe projection;
history returns the existing `Results[CSVArtifact]` wrapper. Fields are `id`,
`source`, `scope`, `status`, `createdAt` (generation request time), `updatedAt`
(last persisted lifecycle transition), `completedAt`, `expiresAt`, `sizeBytes`,
`failureCode`, `retryMode` and optional `retryAt`. Captures, user credentials,
internal paths, blobs, diagnostic details and cleanup errors are not returned.

Generation failures use safe worker codes (`source`, `encoding`) and
`requester-explicit` retry. Storage failures (`spool`, `upload`, `metadata`) use
`automatic-or-requester-explicit` retry, with the worker's persisted `retryAt`.
Only the original requesting user can explicitly retry, and only a failed job
can be retried. Retry returns 202; source permissions are checked again both at
retry and generation. Completed artifacts cannot be marked failed or regenerated
by a local-save outcome API: no such API exists.

Every operation checks current membership and captured source permissions in the
service, not just middleware. User-specific collection sources additionally
require the original user. Private/cross-collection history and artifacts are
always user-only. Shared history omits rows whose current source access cannot be
established. Existing member-readable list sources remain member-readable; role
checks follow the source catalogue rather than inventing owner-only restrictions.

## Historical data safety

Before any retained bytes are served, the service loads the captured source using
current permissions, context, filters and trusted eligibility/recent boundaries.
It encodes current canonical CSV and compares it with the retained blob. Different
bytes deny the artifact, including when a record was removed from the current
user's collection set or made ineligible. This deliberately conservative rule
also denies benign edits/additions and changing history datasets; request a fresh
export. No partial bytes or unrestricted/signed blob URL are returned on failure.
The service buffers the blob and current CSV, in addition to the source adapter's
existing full snapshot: very large downloads have memory cost proportional to
artifact size. Streaming/chunked authorization proofs are not implemented here.

Expiry is checked before storage access and again immediately before returning
bytes, at or after persisted `expiresAt`, irrespective of cleanup progress. The
service rechecks lifecycle/source access after storage reads to catch deletion or
permission changes during those reads. An authorized download returns the original
canonical blob, never a newly regenerated replacement. Responses are `no-store`
and `nosniff`, with an attachment disposition and no redirect.

Delete returns 202 after a durable `cleanup-pending` transition, immediately
revoking downloads and fencing worker publication. Repetition is safe, including
when scoped metadata has already been removed; missing/foreign IDs receive the
same delete response without revealing another scope. The deletion intent retains
the controlled blob reference until blob-first cleanup confirms removal. Recurring
cleanup and retention-policy routes belong to the next story, not these APIs.

Errors are safe JSON `{code: ...}`: invalid captures/IDs 400, unavailable or denied
artifacts 404, not-ready/non-retryable states 409, expired downloads 410, storage
read failures 503, unexpected server errors 500. No raw SQL/blob/source diagnostic
is returned. Client cancellation, disconnection and local file-save failures do
not mutate an otherwise completed server artifact.

Swagger/TypeScript contract generation and retention policy endpoints remain the
separate contract/retention stories. ZIP routes and their cleanup are unchanged.
