import { describe, expect, it } from "vitest";
import {
  attachmentsExport,
  draftFieldsExport,
  itemSelectorExport,
  referencedExport,
  selectorExport,
  snapshotExport,
  fieldOptionsExport,
} from "./auxiliary";
const label = (id: string) => id;
describe("auxiliary record descriptors", () => {
  it("preserves the complete fuzzy eligible set and order, not a virtual window", () => {
    const ids = Array.from({ length: 257 }, (_, i) => `id-${i}`);
    const req = selectorExport("locations", [...ids, ids[0]!], label);
    expect(req.boundary.recordIds).toEqual(ids);
    expect(req.filters).toEqual({});
    expect(req.columns.map(c => c.id)).toEqual(["name", "path"]);
    expect(selectorExport("collections", [], label).scope).toBe("private");
  });
  it("exports full server query and exclusions without pagination or row selection", () => {
    const req = itemSelectorExport({ q: "snow", page: 2, pageSize: 10, includeArchived: false }, ["current"], label);
    expect(req.filters.search).toBe("snow");
    expect(req.boundary).toEqual({ kind: "eligible", excludeIds: ["current"] });
    expect(JSON.stringify(req)).not.toContain("page");
  });
  it("projects only bounded private draft metadata and captures detached values", () => {
    const rows = [{ name: "Snow 雪", type: "text", value: "a\nb", fileBase64: "secret", token: "secret" }];
    const req = snapshotExport("draft-fields", rows, label);
    rows[0]!.value = "changed";
    expect(req.scope).toBe("private");
    expect(req.snapshot).toEqual([{ name: "Snow 雪", type: "text", value: "a\nb" }]);
    expect(() => snapshotExport("draft-fields", Array(1001).fill({ name: "x" }), label)).toThrow("1000");
  });
  it("serializes typed draft field values without losing false or zero", () => {
    expect(
      draftFieldsExport(
        [
          { name: "Zero", type: "number", numberValue: 0 },
          { name: "False", type: "boolean", booleanValue: false },
        ],
        label
      ).snapshot?.map(r => r.value)
    ).toEqual(["0", "false"]);
  });
  it("keeps photo subsets bound to their entity and omits credential URLs", () => {
    const req = attachmentsExport("entity", ["photo"], label);
    expect(req.filters.entityId).toBe("entity");
    expect(req.boundary.recordIds).toEqual(["photo"]);
    expect(req.columns.map(c => c.id)).toEqual(["title", "type", "mimeType"]);
  });
  it("scanner and label datasets remain private, ordered references with no print settings", () => {
    expect(referencedExport("label-records", ["one", "two", "one"], label).boundary.recordIds).toEqual(["one", "two"]);
    expect(referencedExport("scanner-results", [], label).scope).toBe("private");
    expect(fieldOptionsExport("Color", label).filters.fieldName).toBe("Color");
  });
});
