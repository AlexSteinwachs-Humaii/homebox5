import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import {
  administrationExport,
  administrationLabelKeys,
  csvHistoryExport,
  invitationsExport,
  maintenanceExport,
  templateFieldsExport,
} from "./administration";
import { csvSources } from "./source-registry";

const label = (id: string) => `Label ${id}`;

describe("administration export descriptors", () => {
  it("covers ordinary lists with only displayed allowlisted metadata and authoritative scope", () => {
    for (const source of ["templates", "members", "notifiers", "entity-types", "zip-history", "api-keys"] as const) {
      const request = administrationExport(source, label);
      expect(request.scope).toBe(csvSources[source].scope);
      expect(request.boundary).toEqual({ kind: "all" });
      expect(request.sort).toEqual([]);
      expect(request.filters).toEqual({});
      for (const column of request.columns) {
        expect(csvSources[source].columns).toContain(column.id);
        expect(column.label).toBe(label(column.id));
      }
      expect(JSON.stringify(request)).not.toMatch(/token|url|secret|hash|pageSize|selected/);
    }
    expect(administrationExport("api-keys", label).scope).toBe("private");
    expect(administrationExport("notifiers", label).columns.map(c => c.id)).toEqual(["name", "isActive", "createdAt"]);
  });

  it("preserves every maintenance status and the per-item constraint, not a loaded slice", () => {
    for (const status of ["scheduled", "completed", "both"]) {
      const global = maintenanceExport(status, undefined, label);
      expect(global.filters).toEqual({ status });
      expect(global.boundary).toEqual({ kind: "all" });
      expect(global.columns[0]?.id).toBe("entityName");
      const embedded = maintenanceExport(status, "item", label);
      expect(embedded.filters).toEqual({ status, entityId: "item" });
      expect(embedded.boundary).toEqual({ kind: "children" });
      expect(embedded.columns.map(c => c.id)).not.toContain("entityName");
      // Preserve the source repository's status-dependent ordering.
      expect(embedded.sort).toEqual([]);
    }
  });

  it("constrains saved template fields to their template", () => {
    expect(templateFieldsExport("template", label)).toMatchObject({
      source: "template-fields",
      scope: "collection",
      filters: { templateId: "template" },
      boundary: { kind: "children" },
      columns: [{ id: "name" }, { id: "value" }],
    });
  });

  it("captures the entire active invitation set in view order, without duplicate IDs or credentials", () => {
    const visible = [{ id: "local", token: "one-time-secret" }, { id: "loaded" }, { id: "local" }];
    const request = invitationsExport(visible, label);
    expect(request.scope).toBe("collection"); // member-readable metadata must not become owner-only
    expect(request.boundary).toEqual({ kind: "explicit", recordIds: ["local", "loaded"] });
    expect(JSON.stringify(request)).not.toMatch(/token|secret/);
    visible.push({ id: "later" });
    expect(request.boundary.recordIds).toEqual(["local", "loaded"]);
    expect(invitationsExport([], label).boundary.recordIds).toEqual([]);
  });

  it("captures history membership before generating its export and keeps private history private", () => {
    const history = [{ id: "old" }];
    const request = csvHistoryExport("private-csv-history", history, label);
    history.push({ id: "new-history-export" });
    expect(request.scope).toBe("private");
    expect(request.boundary.recordIds).toEqual(["old"]);
    expect(csvHistoryExport("collection-csv-history", [], label).scope).toBe("collection");
    expect(csvHistoryExport("collection-csv-history", [], label).boundary.recordIds).toEqual([]);
  });

  it("has real translation keys for each displayed field", () => {
    const en = JSON.parse(readFileSync(new URL("../../locales/en.json", import.meta.url), "utf8"));
    for (const key of Object.values(administrationLabelKeys)) {
      expect(
        key.split(".").reduce((obj, part) => obj?.[part], en),
        key
      ).toBeTypeOf("string");
    }
  });
});
