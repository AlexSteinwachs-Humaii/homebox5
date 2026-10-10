import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { csvSources } from "./source-registry";
import type { CsvSourceRequest } from "./source-registry";
import surfaces from "./coverage-surfaces.json";

const backendRegistry = JSON.parse(
  readFileSync(
    fileURLToPath(new URL("../../../backend/internal/core/services/csvexports/registry.json", import.meta.url)),
    "utf8"
  )
);

// Type-level contract checks are also compiled by the focused tsc check.
const privateRequest: CsvSourceRequest<"api-keys"> = {
  source: "api-keys",
  scope: "private",
  filters: {},
  boundary: { kind: "all" },
  columns: [{ id: "name", label: "Name" }],
  sort: [],
};
const unsafeRequest: CsvSourceRequest<"api-keys"> = {
  ...privateRequest,
  // @ts-expect-error Secrets are not data columns.
  columns: [{ id: "token", label: "Token" }],
};
const wrongScope: CsvSourceRequest<"api-keys"> = {
  ...privateRequest,
  // @ts-expect-error A private source cannot use collection history.
  scope: "collection",
};

describe("CSV source foundation", () => {
  it("generates the exact backend allowlist", () => {
    expect(csvSources).toEqual(backendRegistry);
  });
  it("maps every registered source to an audited surface", () => {
    const mapped = new Set(surfaces.flatMap(row => row.sources ?? []));
    expect(Object.keys(csvSources).filter(id => !mapped.has(id))).toEqual([]);
    for (const row of surfaces) {
      expect(row.sources?.length || row.excludedReason?.length).toBeTruthy();
      for (const id of row.sources ?? []) expect(csvSources).toHaveProperty(id);
    }
  });
  it("keeps secret and control fields out of ordinary sources", () => {
    for (const [id, source] of Object.entries(csvSources)) {
      expect(source.columns).not.toContain("password");
      expect(source.columns).not.toContain("artifact_path");
      expect(source.columns).not.toContain("actions");
      expect(source.columns).not.toContain("select");
      if (id !== "invitation-visible") expect(source.columns).not.toContain("token");
    }
    expect(privateRequest.scope).toBe("private");
    expect(unsafeRequest.columns[0]?.id).toBe("token");
    expect(wrongScope.scope).toBe("collection");
  });
});
