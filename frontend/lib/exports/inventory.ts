import type { ItemsQuery } from "../api/classes/items";
import type { CsvColumn, CsvSort } from "../api/types/csv-contracts.generated";
import type { CsvSourceRequest } from "./source-registry";
import { csvSources } from "./source-registry";

export type InventoryExport = CsvSourceRequest<"entities"> | CsvSourceRequest<"asset-results">;
export const compactCardFields = [
  "name",
  "location",
  "insured",
  "archived",
  "quantity",
  "purchasePrice",
  "tags",
  "path",
] as const;
export const fullCardFields = [
  "name",
  "location",
  "insured",
  "archived",
  "quantity",
  "description",
  "tags",
  "path",
] as const;
export const treeFields = ["name", "parentId", "path"] as const;
export const locationCardFields = ["name", "itemCount"] as const;

export function exportColumns<T extends string>(
  ids: readonly T[],
  label: (id: T) => string
): (CsvColumn & { id: T })[] {
  return ids.map(id => ({ id, label: label(id) }));
}

// Match the entity list handler, not the presentation's pagination or row selection.
export function inventoryExport(query: ItemsQuery = {}): CsvSourceRequest<"entities"> {
  let search = query.q ?? "";
  let assetId: number | undefined;
  if (search.startsWith("#")) {
    const raw = search.slice(1).replace(/["-]/g, "");
    if (/^\+?\d+$/.test(raw) && Number.isSafeInteger(Number(raw))) {
      assetId = Number(raw);
      search = "";
    }
  }
  const column = ["createdAt", "updatedAt", "assetId"].includes(query.orderBy ?? "") ? query.orderBy! : "name";
  return {
    source: "entities",
    scope: "collection",
    filters: {
      search,
      ...(assetId !== undefined && assetId > 0 ? { assetId } : {}),
      parentIds: [...(query.parentIds ?? [])],
      tagIds: [...(query.tags ?? [])],
      negateTags: query.negateTags ?? false,
      onlyWithoutPhoto: query.onlyWithoutPhoto ?? false,
      onlyWithPhoto: query.onlyWithPhoto ?? false,
      includeArchived: query.includeArchived ?? false,
      fields: (query.fields ?? []).flatMap(field => {
        const split = field.indexOf("=");
        return split > 0 ? [{ name: field.slice(0, split), value: field.slice(split + 1) }] : [];
      }),
    },
    boundary: { kind: "all" },
    sort: [
      {
        column: column as "name" | "createdAt" | "updatedAt" | "assetId",
        direction: column.endsWith("At") ? "desc" : "asc",
      },
    ],
    columns: [{ id: "name", label: "Name" }],
  };
}

// Capture ordered visible leaf data columns and the effective sort without touching
// table state. No selected IDs, page size or rendered row IDs enter this request.
export function captureInventoryView<T extends InventoryExport>(
  source: T,
  visibleIds: string[],
  sorting: { id: string; desc: boolean }[],
  view: "table" | "card",
  label: (id: string) => string
): T {
  const allowed: readonly string[] = csvSources[source.source].columns;
  const ids = view === "card" ? compactCardFields : visibleIds.filter(id => allowed.includes(id));
  const sort: CsvSort[] = sorting.length
    ? sorting.filter(s => allowed.includes(s.id)).map(s => ({ column: s.id, direction: s.desc ? "desc" : "asc" }))
    : source.sort;
  return JSON.parse(JSON.stringify({ ...source, columns: exportColumns(ids, label), sort })) as T;
}
