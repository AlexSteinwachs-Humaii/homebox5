# CSV server verification — story 9

Verified on 2026-10-10 with Go 1.27.1, SQLite and PostgreSQL **15.19**.
No frontend UI or Feature association was added. These are server tests, not a
browser/UI acceptance walk.

## Added regression checks

- `TestCSVSurfaceContractRegression`: independent 29-surface → typed-adapter
  manifest (not derived from the implementation). Every surface executes with
  reordered columns and all advertised order columns/directions. Applicable
  entity/text/maintenance filters, stale hierarchy contexts, denied boundary
  authority, empty selector/recent sets, duplicate/secret/control columns and
  unsupported ordering are checked. Populated ordering, hierarchy traversal,
  selector exclusions and filter semantics are also exercised by the existing
  inventory/admin fixture tests. Calling an order option on an empty source alone
  does not prove populated sort semantics.
- `TestCanonicalCSVGoldenBytes`: fixed expected UTF-8 bytes for comma/quote/LF
  escaping, Unicode, empty values, fractional quantities, currency numbers,
  date-only values, UTC timestamps, formula-neutralized strings and repeatable
  encoding. Existing writer fixtures cover control/Unicode whitespace, typed
  negative values, invalid UTF-8/nonfinite numbers and writer failures.
- `TestCSVRealStorageLifecycleBothDialects`: fresh independent SQL databases,
  real typed adapters and filesystem blobs. Tests metadata failure after upload,
  a fresh service's SQL reconciliation, duplicate delivery, canonical download
  equality, private ownership isolation, policy edits, before/at/after expiry,
  delayed cleanup, blob deletion failure, metadata deletion failure after blob
  removal, restart recovery, missing-object/repeated cleanup and deletion.
  Collection membership revocation after enqueue prevents generation; revocation
  after completion denies status/history/download/deletion/policies.
  Foreign-user deletion intentionally returns a non-revealing scoped-missing
  success; the test verifies that the victim's artifact is not mutated.

## Passing commands

From `backend/` (URL shown is this sandbox's local disposable test setup):

```sh
export CSV_TEST_POSTGRES_URL='postgres://root@/csv_test?host=/var/run/postgresql&sslmode=disable'
go test -race ./internal/core/services ./internal/data/repo ./app/api/handlers/v1 ./app/api ./app/tools/... -count=1
go test -race ./internal/core/services ./internal/data/repo ./app/api/handlers/v1 ./app/api ./app/tools/... -run 'CSV|CanonicalCSV|Export|Import|SevenDay' -count=1 -v
go vet ./internal/core/services ./internal/data/repo ./app/api/... ./app/tools/...
```

The full-package race run passed before the final golden-byte/cleanup-fault test
additions; the filtered race run passed with **all final test additions**.
Both dialect subtests passed for schema persistence, historical Goose upgrades
and the real-storage lifecycle. PostgreSQL tests create/drop temporary databases;
`CSV_TEST_POSTGRES_URL` requires CREATEDB. Without it PostgreSQL is explicitly
**skipped**, not passed. PostgreSQL 16/17 were not tested here.

Existing passing regressions include `TestExportRoundTrip`,
`TestCSVExportImportPreservesItemParent`, import rejection/readiness checks,
`TestCSVMetadataExcludedFromInventoryArchives` and
`TestSevenDayZIPCleanupDoesNotTouchCSV`. Existing service fault fixtures cover
source/generation failure, interrupted upload, lease expiry/stale deliveries,
SQL publication failure, and concurrent cleanup/deletion/worker delivery.
They simulate failure boundaries using hooks; this is not a literal OS kill or
cloud-provider outage test.

From the repository root:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
task swag typescript-types
```

Passed; regeneration produced **no tracked artifact changes**. Published contract
checks and both API-only workflow and OpenAPI/typegen tool tests passed. From
`frontend/`, the isolated generated contracts compile successfully:

```sh
pnpm exec tsc --noEmit --skipLibCheck --target ES2022 --moduleResolution bundler --module ESNext lib/api/types/data-contracts.ts
```

## Failures and environmental limitations (not passing checks)

The uncached full backend command, with PostgreSQL configured:

```sh
cd backend && go test ./... -count=1
```

failed only in the unrelated notifier validation package:

```text
--- FAIL: TestValidateNotifierURL
    --- FAIL: TestValidateNotifierURL/generic_notifier_with_public_IP_passes
        notifier_url_test.go:428: expected no error but got: bogon/reserved network addresses are blocked
    --- FAIL: TestValidateNotifierURL/generic_notifier_shorthand_host/path_passes
        notifier_url_test.go:428: expected no error but got: bogon/reserved network addresses are blocked
```

Those tests use `example.com`; `getent ahostsv4 example.com` returned
`198.19.0.2` (reserved benchmarking network) in this sandbox. The security guard
was not weakened to make those environmental DNS-dependent checks green.
All CSV-related packages passed in that full run.

`task go:lint` could not execute the configured linter:

```text
"golangci-lint": executable file not found in $PATH
task: Failed to run task "go:lint": exit status 127
```

`cd frontend && pnpm run typecheck` exited 2 with errors outside the changed
server test files, including:

```text
pages/location/[id]/index/index.vue(339,32): error TS2339: Property 'id' does not exist on type 'Ref<EntityOut | undefined, EntityOut | undefined>'.
pages/template/[id].vue(125,74): error TS2345: ... is not assignable to parameter of type 'EntityTemplateUpdate'.
plugins/i18n.ts(33,14): error TS2345: ... is not assignable to parameter of type 'Plugin<unknown[], unknown[]>'.
```

No frontend source or generated contract changes were made. Isolated contract
compilation passed, but **the whole frontend typecheck is not green**. Go vet is
not a substitute for the missing golangci-lint run. Cloud storage, PostgreSQL
16/17 and full browser E2E remain unverified by this story.
