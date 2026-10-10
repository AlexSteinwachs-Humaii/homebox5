import { describe, expect, it } from "vitest";
import { captureInventoryView, compactCardFields, inventoryExport } from "./inventory";

const label = (id: string) => `Label ${id}`;

describe("inventory export descriptors", () => {
  it("captures list-equivalent filters without pagination or selection", () => {
    const request = inventoryExport({
      q: "unicode 灯",
      parentIds: ["room"],
      tags: ["tag"],
      fields: ["Color=Blue=dark"],
      negateTags: true,
      includeArchived: true,
      onlyWithoutPhoto: true,
      page: 4,
      pageSize: 12,
      orderBy: "updatedAt",
    });
    expect(request.filters).toMatchObject({
      search: "unicode 灯",
      parentIds: ["room"],
      tagIds: ["tag"],
      fields: [{ name: "Color", value: "Blue=dark" }],
      negateTags: true,
      includeArchived: true,
      onlyWithoutPhoto: true,
    });
    expect(request.boundary).toEqual({ kind: "all" });
    expect(request.sort).toEqual([{ column: "updatedAt", direction: "desc" }]);
    expect(JSON.stringify(request)).not.toMatch(/page|selected|pageSize/);
  });

  it("matches asset-ID search normalization and leaves invalid IDs as search", () => {
    expect(inventoryExport({ q: '#"123-456"' }).filters).toMatchObject({
      assetId: 123456,
      search: "",
    });
    expect(inventoryExport({ q: "#invalid" }).filters).toMatchObject({
      search: "#invalid",
    });
    expect(inventoryExport({ q: "#123junk" }).filters).toMatchObject({
      search: "#123junk",
    });
  });

  it("keeps ordered visible data columns, excluding controls and hidden fields", () => {
    const source = inventoryExport({ orderBy: "createdAt" });
    const result = captureInventoryView(
      source,
      ["select", "purchasePrice", "name", "actions", "secret"],
      [],
      "table",
      label
    );
    expect(result.columns).toEqual([
      { id: "purchasePrice", label: "Label purchasePrice" },
      { id: "name", label: "Label name" },
    ]);
    expect(result.sort).toEqual(source.sort);
  });

  it("captures table multi-sort even when its column is hidden", () => {
    const result = captureInventoryView(
      inventoryExport(),
      ["name"],
      [
        { id: "quantity", desc: true },
        { id: "name", desc: false },
      ],
      "table",
      label
    );
    expect(result.sort).toEqual([
      { column: "quantity", direction: "desc" },
      { column: "name", direction: "asc" },
    ]);
  });

  it("uses displayed compact card fields rather than hidden table preferences", () => {
    const result = captureInventoryView(inventoryExport(), ["createdAt"], [], "card", label);
    expect(result.columns.map(c => c.id)).toEqual(compactCardFields);
  });

  it("keeps the intentional recent and child boundaries", () => {
    const recent = inventoryExport({ orderBy: "createdAt" });
    recent.boundary = { kind: "recent", limit: 5 };
    expect(captureInventoryView(recent, ["name"], [{ id: "name", desc: false }], "table", label).boundary).toEqual({
      kind: "recent",
      limit: 5,
    });
    const child = inventoryExport();
    child.boundary = { kind: "children" };
    child.filters.entityId = "parent";
    expect(captureInventoryView(child, [], [], "card", label).filters.entityId).toBe("parent");
  });

  it("captures a detached snapshot without changing the source or preferences", () => {
    const source = inventoryExport({ parentIds: ["parent"] });
    const before = JSON.stringify(source);
    const ids = ["name"];
    const sorting = [{ id: "quantity", desc: true }];
    const snapshot = captureInventoryView(source, ids, sorting, "table", label);
    expect(JSON.stringify(source)).toBe(before);
    source.filters.parentIds?.push("another");
    ids.push("insured");
    sorting[0]!.desc = false;
    expect(snapshot.filters.parentIds).toEqual(["parent"]);
    expect(snapshot.columns).toHaveLength(1);
    expect(snapshot.sort[0]!.direction).toBe("desc");
  });

  it("does not invent columns for a table with no visible data", () => {
    expect(captureInventoryView(inventoryExport(), ["select", "actions"], [], "table", label).columns).toEqual([]);
  });
});
