import type { CsvRequest } from "../api/types/csv-contracts.generated";
import { csvSources, type CsvSourceId } from "./source-registry.generated";

export { csvSources, type CsvSourceId };
// UI descriptors are narrowed per source; server decoding/authorization remains
// authoritative. No SQL, storage keys, user IDs or permission flags in requests.
type FilterKeys<S extends CsvSourceId> = Extract<
  (typeof csvSources)[S]["filters"][number],
  keyof CsvRequest["filters"]
>;
type SourceFilters<S extends CsvSourceId> = [FilterKeys<S>] extends [never]
  ? Record<string, never>
  : Pick<CsvRequest["filters"], FilterKeys<S>>;

export type CsvSourceRequest<S extends CsvSourceId> = Omit<
  CsvRequest,
  "source" | "scope" | "columns" | "boundary" | "sort" | "filters"
> & {
  filters: SourceFilters<S>;
  source: S;
  scope: (typeof csvSources)[S]["scope"];
  columns: { id: (typeof csvSources)[S]["columns"][number]; label: string }[];
  sort: {
    column: (typeof csvSources)[S]["columns"][number];
    direction: "asc" | "desc";
  }[];
  boundary: Omit<CsvRequest["boundary"], "kind"> & {
    kind: (typeof csvSources)[S]["boundaries"][number];
  };
};

// Discriminated union for the shared action/client: a caller cannot pair a
// private source with collection retention or another source's fields.
export type CsvExportRequest = { [S in CsvSourceId]: CsvSourceRequest<S> }[CsvSourceId];
