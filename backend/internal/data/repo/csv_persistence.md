# CSV persistence (story 2)

`CSVExportRepository` is wired as `AllRepos.CSVExports`. It does not use
`ExportRepository`, inventory attachments, or the ZIP worker/topic.

## Storage contract

- `csv_exports`: immutable requester `user_id`, source, JSON capture, explicit
  `scope`, optional `group_id`, and server-allocated unique `blob_key`.
- Collection scope requires a collection. Account-private and cross-collection
  scopes prohibit a collection. Private history explicitly checks both scope
  and owning user; collection history explicitly checks scope and collection.
- Statuses: `queued`, `running`, `completed`, `generation-failed`,
  `storage-failed`, `cleanup-pending`. Completion requires completion/expiry
  timestamps and expiry strictly later than completion. Size and attempts are
  nonnegative. Failure code/detail, cleanup error and retry time survive restarts.
- Keys are allocated before uploading: `csv/{user UUID}/{artifact UUID}.csv`.
  Database checks bind the key to those IDs. Do not reuse the same key for
  concurrent generation attempts without an idempotent publication protocol.
  Blob keys and internal failure/cleanup details are sensitive Ent fields and
  omitted from JSON. Public response DTOs must still be mapped deliberately.
- Composite indexes cover collection/private history, lifecycle/expiry, and
  lifecycle/retry scans. `CleanupCandidates` is an unprivileged worker primitive,
  not a public API. Metadata must survive until blob deletion succeeds or absence
  is confirmed. No cleanup executor or lifecycle transition protocol is added here.

## Retention policies

Policies live on `users.csv_retention_days` and `groups.csv_retention_days`, not
on ZIP/import rows or inventory attachments. Both default to 30, including
pre-existing users/groups upgraded with Goose. The supported persistence APIs
accept an integer only, with range 1..106751 (the largest whole-day
`time.Duration`). Ent validators and SQL checks enforce the range; SQLite SQL
checks also reject fractional values despite dynamic typing. HTTP consumers
must decode into the integer type rather than truncate a floating-point number.

Policy updates do not modify existing artifacts. Later worker/retention stories
must choose policy at completion and atomically persist completion plus expiry;
this story supplies storage, not those transitions or authorization endpoints.
Policy repository methods require service-level current-owner/requester checks.
Artifact creation requires the service's already validated typed capture and
current authorization; repositories cannot import the source catalogue because
services depend on repositories. Store the full capture as a JSON object.

## Deletion and ZIP separation

Parent foreign keys use NO ACTION, deliberately not CASCADE/SET NULL. Deleting a
user/collection with CSV rows requires the deletion workflow to remove their
blobs and metadata first. This avoids losing the only cleanup pointer and avoids
silently converting shared artifacts into private rows. Integrate this requirement
when lifecycle/deletion services are implemented.

CSV rows and policies are excluded from the inventory ZIP table allowlist (also
used by restore validation and inventory wiping), so restore cannot resurrect
live CSV blob references. Existing `exports` rows, ZIP/import services and the
seven-day `purgeStaleExports` task are unchanged.

## Verification

From `backend/`:

```sh
go generate ./internal/data/ent
# Optional PostgreSQL tests require a URL-form DSN with CREATEDB permission.
# Each test creates/drops its own database; it never resets the supplied one.
CSV_TEST_POSTGRES_URL='postgres://root@/csv_test?host=/var/run/postgresql&sslmode=disable' \
  go test -race ./internal/data/repo ./internal/core/services ./app/api
```

Tests exercise fresh Ent schemas and full historical Goose upgrades in SQLite
and PostgreSQL, seeding existing users/groups and ZIP/import artifacts before the
CSV migration. They verify policy defaults/invalid values, scope constraints,
isolated histories, controlled keys, completion checks, failure/retry persistence,
query indexes and archive exclusion. API-package regression tests exercise actual
seven-day ZIP/import blob cleanup while retaining CSV metadata and bytes.

Observed: targeted race tests pass with PostgreSQL 15.19 and SQLite. `go test ./...`
fails in two existing `TestValidateNotifierURL` cases with
`expected no error but got: bogon/reserved network addresses are blocked`.
Both failures were reproduced on unchanged parent commit `783f5c0`.
