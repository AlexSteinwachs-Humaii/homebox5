import { exportColumns } from "./inventory";
import { csvSources, type CsvSourceRequest } from "./source-registry";
import type { CsvArtifact } from "../api/types/csv-contracts.generated";

const fields = {
  templates: ["name", "description"],
  members: ["name", "email"],
  notifiers: ["name", "isActive", "createdAt"],
  "entity-types": ["name", "isLocation", "defaultTemplate"],
  "zip-history": ["createdAt", "status", "sizeBytes"],
  "api-keys": ["name", "createdAt", "lastUsedAt", "expiresAt"],
} as const;

// These views have no pagination, search or local sorting. Empty sort preserves
// their source repository order. The registry supplies scope, never the active
// collection preference (in particular API keys always belong to the account).
export function administrationExport<S extends keyof typeof fields>(
  source: S,
  label: (id: string) => string
): CsvSourceRequest<S> {
  return {
    source,
    scope: csvSources[source].scope,
    filters: {},
    boundary: { kind: "all" },
    columns: exportColumns(fields[source], label),
    sort: [],
  } as unknown as CsvSourceRequest<S>;
}

export function maintenanceExport(
  status: string,
  entityId: string | undefined,
  label: (id: string) => string
): CsvSourceRequest<"maintenance"> {
  return {
    source: "maintenance",
    scope: "collection",
    filters: { status, ...(entityId ? { entityId } : {}) },
    boundary: { kind: entityId ? "children" : "all" },
    columns: exportColumns(
      entityId
        ? ["name", "scheduledDate", "completedDate", "cost", "description"]
        : ["entityName", "name", "scheduledDate", "completedDate", "cost", "description"],
      label
    ),
    sort: [],
  };
}

export function templateFieldsExport(
  templateId: string,
  label: (id: string) => string
): CsvSourceRequest<"template-fields"> {
  return {
    source: "template-fields",
    scope: "collection",
    filters: { templateId },
    boundary: { kind: "children" },
    columns: exportColumns(["name", "value"], label),
    sort: [],
  };
}

// The view merges newly-created invites with loaded invites and hides expired
// rows. Capture the COMPLETE matching ID set, deduplicated in display order;
// never include the separately visible one-time token or reconstruct its hash.
export function invitationsExport(
  rows: readonly { id: string }[],
  label: (id: string) => string
): CsvSourceRequest<"invitations"> {
  return {
    source: "invitations",
    scope: "collection",
    filters: {},
    boundary: {
      kind: "explicit",
      recordIds: [...new Set(rows.map(row => row.id))],
    },
    columns: exportColumns(["expiresAt", "remainingUses"], label),
    sort: [],
  };
}

// S8's history presentation can reuse this descriptor. Fixed membership avoids
// including the newly-created history export itself. Server access is rechecked.
export function csvHistoryExport<S extends "collection-csv-history" | "private-csv-history">(
  source: S,
  rows: readonly Pick<CsvArtifact, "id">[],
  label: (id: string) => string
): CsvSourceRequest<S> {
  return {
    source,
    scope: csvSources[source].scope,
    filters: {},
    boundary: { kind: "explicit", recordIds: rows.map(row => row.id) },
    columns: exportColumns(["source", "status", "createdAt", "completedAt", "expiresAt", "sizeBytes"], label),
    sort: [],
  } as CsvSourceRequest<S>;
}

export const administrationLabelKeys: Record<string, string> = {
  name: "global.name",
  description: "components.template.form.template_description",
  email: "collection.members.email",
  isActive: "profile.enabled",
  createdAt: "global.created",
  lastUsedAt: "profile.api_key_last_used",
  expiresAt: "collection.expires_at",
  isLocation: "components.entityTypes.create_dialog.is_container_location_type_label",
  defaultTemplate: "components.template.form.template_name",
  status: "tools.backups_set.table.status",
  sizeBytes: "tools.backups_set.table.size",
  remainingUses: "collection.remaining_uses",
  entityName: "components.template.form.item_name",
  scheduledDate: "maintenance.modal.scheduled_date",
  completedDate: "maintenance.modal.completed_date",
  cost: "maintenance.modal.cost",
  value: "components.template.form.default_value",
  source: "csv_export.source",
  completedAt: "csv_export.completed_at",
};
