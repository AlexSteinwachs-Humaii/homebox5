-- +goose Up
alter table groups add column csv_export_retention_days integer not null default 30
    check (typeof(csv_export_retention_days) = 'integer' and csv_export_retention_days between 1 and 106751);
alter table users add column csv_export_retention_days integer not null default 30
    check (typeof(csv_export_retention_days) = 'integer' and csv_export_retention_days between 1 and 106751);

-- Not part of ZIP backup/import history. No cascading deletion: retain blob
-- pointers until the dedicated CSV cleanup has removed stored artifacts.
create table csv_exports (
    id uuid primary key not null,
    created_at datetime not null,
    updated_at datetime not null,
    requester_id uuid not null references users(id) on delete restrict,
    group_id uuid references groups(id) on delete restrict,
    scope text not null check (scope in ('collection', 'private')),
    source text not null,
    access_requirement text not null,
    request_snapshot json not null,
    retention_days integer not null default 30 check (typeof(retention_days) = 'integer' and retention_days between 1 and 106751),
    status text not null default 'pending' check (status in ('pending', 'running', 'completed', 'failed', 'deleting')),
    completed_at datetime,
    expires_at datetime,
    artifact_path text,
    size_bytes bigint not null default 0 check (size_bytes >= 0),
    error text check (error is null or length(error) <= 1000),
    check ((scope = 'collection' and group_id is not null) or (scope = 'private' and group_id is null)),
    check (status != 'completed' or (completed_at is not null and expires_at is not null and expires_at > completed_at and artifact_path is not null and artifact_path <> ''))
);
create index csvexport_group_id_status on csv_exports(group_id, status);
create index csvexport_requester_id_scope on csv_exports(requester_id, scope);
create index csvexport_expires_at on csv_exports(expires_at);

-- +goose Down
drop table csv_exports;
alter table groups drop column csv_export_retention_days;
alter table users drop column csv_export_retention_days;
