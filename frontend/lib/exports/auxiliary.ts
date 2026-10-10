import type { CsvSnapshotRow } from "../api/types/csv-contracts.generated";
import { inventoryExport, exportColumns } from "./inventory";
import type { CsvSourceRequest } from "./source-registry";
import { csvSources } from "./source-registry";
import type { ItemsQuery } from "../api/classes/items";

type SelectorSource = "collections" | "locations" | "tags" | "templates" | "entity-types";
const selectorColumns = {
  collections: ["name"],
  locations: ["name", "path"],
  tags: ["name", "path"],
  templates: ["name", "description"],
  "entity-types": ["name", "isLocation"],
} as const;
// IDs come from the COMPLETE eligible, fuzzy-matched list, before virtualization.
// Do not send fuzzy text as a server substring predicate: that changes membership.
export function selectorExport<S extends SelectorSource>(
  source: S,
  ids: string[],
  label: (id: string) => string
): { [K in S]: CsvSourceRequest<K> }[S] {
  return {
    source,
    scope: csvSources[source].scope,
    filters: {},
    boundary: { kind: "explicit", recordIds: [...new Set(ids)] },
    sort: [],
    columns: exportColumns(selectorColumns[source], label),
  } as unknown as { [K in S]: CsvSourceRequest<K> }[S];
}
export function itemSelectorExport(
  query: ItemsQuery,
  excludeIds: string[],
  label: (id: string) => string
): CsvSourceRequest<"entities"> {
  return {
    ...inventoryExport(query),
    boundary: { kind: "eligible", excludeIds: [...new Set(excludeIds)] },
    columns: exportColumns(["name"], label),
  };
}
export function attachmentsExport(
  entityId: string,
  ids: string[],
  label: (id: string) => string
): CsvSourceRequest<"attachments"> {
  return {
    source: "attachments",
    scope: "collection",
    filters: { entityId },
    boundary: { kind: "explicit", recordIds: [...ids] },
    sort: [],
    columns: exportColumns(["title", "type", "mimeType"], label),
  };
}
export function referencedExport<S extends "scanner-results" | "label-records">(
  source: S,
  ids: string[],
  label: (id: string) => string
): CsvSourceRequest<S> {
  return {
    source,
    scope: "private",
    filters: {},
    boundary: { kind: "explicit", recordIds: [...new Set(ids)] },
    sort: [],
    columns: exportColumns(
      source === "label-records"
        ? ["assetId", "name", "location"]
        : [
            "name",
            "assetId",
            "location",
            "quantity",
            "purchasePrice",
            "childItemCount",
            "locationChildCount",
            "totalPrice",
            "path",
          ],
      label
    ),
  } as unknown as CsvSourceRequest<S>;
}
export function snapshotExport<S extends "draft-fields" | "draft-photos" | "product-results">(
  source: S,
  rows: CsvSnapshotRow[],
  label: (id: string) => string
): CsvSourceRequest<S> {
  if (rows.length > 1000) throw new Error("CSV snapshots support at most 1000 records");
  // Project declared metadata only; never accept file data, URLs or object spreads.
  const snapshot = rows.map(row =>
    Object.fromEntries(
      csvSources[source].columns.filter(key => key in row).map(key => [key, row[key as keyof CsvSnapshotRow]])
    )
  );
  return {
    source,
    scope: "private",
    filters: {},
    boundary: { kind: "snapshot" },
    sort: [],
    columns: exportColumns(csvSources[source].columns, label),
    snapshot,
  } as unknown as CsvSourceRequest<S>;
}
export function draftFieldsExport(
  fields: {
    name: string;
    type?: string;
    textValue?: string;
    numberValue?: number;
    booleanValue?: boolean;
    timeValue?: string;
  }[],
  label: (id: string) => string
) {
  return snapshotExport(
    "draft-fields",
    fields.map(f => ({
      name: f.name,
      type: f.type ?? "text",
      value: String(
        f.type === "number"
          ? (f.numberValue ?? "")
          : f.type === "boolean"
            ? (f.booleanValue ?? false)
            : f.type === "time"
              ? (f.timeValue ?? "")
              : (f.textValue ?? "")
      ),
    })),
    label
  );
}

export function entityRecordsExport<S extends "entity-fields" | "entity-tags" | "entity-path">(
  source: S,
  entityId: string,
  label: (id: string) => string
): CsvSourceRequest<S> {
  return {
    source,
    scope: "collection",
    filters: { entityId },
    boundary: { kind: "children" },
    sort: [],
    columns: exportColumns(
      source === "entity-fields" ? ["name", "type", "value"] : ["name", "parentId", "path"],
      label
    ),
  } as unknown as CsvSourceRequest<S>;
}
export function fieldOptionsExport(
  fieldName: string,
  label: (id: string) => string
): CsvSourceRequest<"entity-fields"> {
  return {
    source: "entity-fields",
    scope: "collection",
    filters: { fieldName },
    boundary: { kind: "eligible" },
    sort: [],
    columns: exportColumns(fieldName ? ["name", "value"] : ["name"], label),
  };
}
