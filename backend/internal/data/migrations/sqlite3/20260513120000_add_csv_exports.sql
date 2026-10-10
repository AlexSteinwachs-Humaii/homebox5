-- +goose Up
-- CSV metadata is intentionally absent from the ZIP export/import table list.
ALTER TABLE users ADD COLUMN csv_retention_days bigint NOT NULL DEFAULT 30
    CONSTRAINT user_csv_retention CHECK (csv_retention_days BETWEEN 1 AND 106751 AND csv_retention_days = CAST(csv_retention_days AS BIGINT));
ALTER TABLE groups ADD COLUMN csv_retention_days bigint NOT NULL DEFAULT 30
    CONSTRAINT group_csv_retention CHECK (csv_retention_days BETWEEN 1 AND 106751 AND csv_retention_days = CAST(csv_retention_days AS BIGINT));
CREATE TABLE csv_exports (
 id uuid PRIMARY KEY NOT NULL,
 created_at datetime NOT NULL,
 updated_at datetime NOT NULL,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE NO ACTION,
 group_id uuid REFERENCES groups(id) ON DELETE NO ACTION,
 source text NOT NULL CHECK (length(source) > 0),
 capture json NOT NULL,
 scope text NOT NULL CHECK (scope IN ('collection', 'account-private', 'cross-collection')),
 status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'completed', 'generation-failed', 'storage-failed', 'cleanup-pending')),
 completed_at datetime,
 expires_at datetime,
 blob_key text NOT NULL UNIQUE CONSTRAINT csv_blob CHECK (blob_key = 'csv/' || CAST(user_id AS TEXT) || '/' || CAST(id AS TEXT) || '.csv'),
 size_bytes bigint NOT NULL DEFAULT 0,
 failure_code text CHECK (length(failure_code) <= 100),
 failure_detail text CHECK (length(failure_detail) <= 1000),
 attempts bigint NOT NULL DEFAULT 0,
 retry_at datetime,
 cleanup_error text CHECK (length(cleanup_error) <= 1000),
 CONSTRAINT csv_scope_group CHECK ((scope = 'collection' AND group_id IS NOT NULL) OR (scope IN ('account-private', 'cross-collection') AND group_id IS NULL)),
 CONSTRAINT csv_completion CHECK (status <> 'completed' OR (completed_at IS NOT NULL AND expires_at IS NOT NULL AND expires_at > completed_at)),
 CONSTRAINT csv_size CHECK (size_bytes >= 0 AND attempts >= 0)
);
CREATE INDEX csvexport_group_id_scope_created_at ON csv_exports(group_id, scope, created_at);
CREATE INDEX csvexport_user_id_scope_created_at ON csv_exports(user_id, scope, created_at);
CREATE INDEX csvexport_status_expires_at ON csv_exports(status, expires_at);
CREATE INDEX csvexport_status_retry_at ON csv_exports(status, retry_at);
