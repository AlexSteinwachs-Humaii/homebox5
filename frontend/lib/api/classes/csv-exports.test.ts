import { afterEach, expect, it, vi } from "vitest";
import { CsvExportsAPI } from "./csv-exports";
import { Requests } from "../../requests";
import { inventoryExport } from "../../exports/inventory";

afterEach(() => vi.unstubAllGlobals());

it("creates collection artifacts through the typed endpoint without pagination", async () => {
  const fetch = vi.fn().mockResolvedValue(
    new Response(JSON.stringify({ id: "artifact", status: "pending" }), {
      status: 202,
      headers: { "Content-Type": "application/json" },
    })
  );
  vi.stubGlobal("fetch", fetch);
  const api = new CsvExportsAPI(new Requests("", "", { "X-Tenant": "collection" }));
  const request = inventoryExport({ q: "lamp", page: 5 });
  expect((await api.create(request)).data.id).toBe("artifact");
  expect(fetch.mock.calls[0]![0]).toBe("/api/v1/group/csv-exports");
  expect(JSON.parse(fetch.mock.calls[0]![1].body)).toEqual(request);
});

it("downloads the retained artifact without parsing CSV into JSON or regenerating", async () => {
  const fetch = vi.fn().mockResolvedValue(new Response('Name\n"灯"\n', { headers: { "Content-Type": "text/csv" } }));
  vi.stubGlobal("fetch", fetch);
  const api = new CsvExportsAPI(new Requests(""));
  const result = await api.download("collection", "artifact");
  expect(await result.response.text()).toBe('Name\n"灯"\n');
  expect(fetch).toHaveBeenCalledOnce();
  expect(fetch.mock.calls[0]![0]).toBe("/api/v1/group/csv-exports/artifact/download");
});

it("routes account metadata creation, history and download only through private endpoints", async () => {
  const fetch = vi.fn().mockImplementation(() =>
    Promise.resolve(
      new Response(JSON.stringify({ items: [], id: "private-artifact" }), {
        headers: { "Content-Type": "application/json" },
      })
    )
  );
  vi.stubGlobal("fetch", fetch);
  const api = new CsvExportsAPI(new Requests("", "", { "X-Tenant": "unrelated-collection" }));
  const { administrationExport } = await import("../../exports/administration");
  await api.create(administrationExport("api-keys", id => id));
  expect((await api.list("private")).data.items).toEqual([]);
  await api.download("private", "private-artifact");
  expect(fetch.mock.calls.map(call => call[0])).toEqual([
    "/api/v1/user/csv-exports",
    "/api/v1/user/csv-exports",
    "/api/v1/user/csv-exports/private-artifact/download",
  ]);
});
