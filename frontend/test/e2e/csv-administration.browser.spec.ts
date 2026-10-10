import { expect, test, type Page } from "@playwright/test";
import type { CsvRequest } from "../../lib/api/types/csv-contracts.generated";

test.use({ serviceWorkers: "block" });
const id = "11111111-1111-4111-8111-111111111111";
const oldHistoryId = "22222222-2222-4222-8222-222222222222";

async function fixture(page: Page, baseURL: string, noCollection = false) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  const requests: { path: string; body: CsvRequest }[] = [];
  const template = {
    id,
    name: "Template 雪",
    description: "Long description",
    defaultQuantity: 1,
    fields: [{ id, name: "Color", textValue: "Blue" }],
    defaultTags: [],
  };
  await page.route("**/api/v1/**", route => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    if (path.endsWith("/csv-exports") && route.request().method() === "POST") {
      requests.push({ path, body: route.request().postDataJSON() });
      return route.fulfill({ status: 202, json: { id: "artifact", status: "pending" } });
    }
    if (path.endsWith("/csv-exports/artifact"))
      return route.fulfill({ json: { id: "artifact", source: requests.at(-1)?.body.source, status: "completed" } });
    if (path.endsWith("/csv-exports/artifact/download"))
      return route.fulfill({ status: 403, json: { message: "CSV access denied" } });
    if (path.endsWith("/csv-exports"))
      return route.fulfill({ json: { items: [{ id: oldHistoryId, source: "api-keys", status: "completed" }] } });
    if (path === "/status") return route.fulfill({ json: { build: { version: "v1.0.0" } } });
    if (path === "/users/self")
      return route.fulfill({
        json: {
          item: {
            id: "user",
            name: "Member",
            email: "me@example.com",
            defaultGroupId: noCollection ? null : "group",
            group: noCollection ? null : { id: "group", name: "Collection", currency: "USD" },
          },
        },
      });
    if (path === "/users/self/settings")
      return route.fulfill({ json: { item: { theme: "cinematic", language: "en" } } });
    if (path === "/groups/all")
      return route.fulfill({ json: noCollection ? [] : [{ id: "group", name: "Collection" }] });
    if (path === "/groups") return route.fulfill({ json: { id: "group", name: "Collection", currency: "USD" } });
    if (path === "/templates") return route.fulfill({ json: [template] });
    if (path === `/templates/${id}`) return route.fulfill({ json: template });
    if (path === `/entities/${id}`)
      return route.fulfill({ json: { id, name: "Item", fields: [], attachments: [], tags: [], quantity: 1 } });
    if (path === "/groups/invitations")
      return route.fulfill({
        json: [
          { id, expiresAt: "2099-01-01", uses: 3, token: "never-export-token" },
          { id: oldHistoryId, expiresAt: "2020-01-01", uses: 2 },
        ],
      });
    if (path === "/groups/members")
      return route.fulfill({ json: [{ id: "user", name: "Member", email: "me@example.com" }] });
    if (path === "/notifiers")
      return route.fulfill({
        json: [{ id, name: "Notifier", url: "secret://credential", isActive: true, createdAt: "2026-01-01" }],
      });
    if (path === "/entity-types") return route.fulfill({ json: [{ id, name: "Item", isLocation: false }] });
    if (path === "/users/self/api-keys")
      return route.fulfill({ json: [{ id, name: "Key", createdAt: "2026-01-01", lastUsedAt: null, expiresAt: null }] });
    if (path === "/entities") return route.fulfill({ json: { items: [], total: 0 } });
    return route.fulfill({ json: [] });
  });
  return requests;
}

for (const [path, source] of [
  ["/templates", "templates"],
  [`/template/${id}`, "template-fields"],
  ["/collection/members", "members"],
  ["/collection/invites", "invitations"],
  ["/collection/notifiers", "notifiers"],
  ["/collection/entity-types", "entity-types"],
  ["/collection/tools", "zip-history"],
  ["/profile", "api-keys"],
  ["/maintenance", "maintenance"],
  [`/item/${id}/maintenance`, "maintenance"],
] as const) {
  test(`${source} at ${path} captures its source without disturbing navigation`, async ({ page, baseURL }) => {
    const requests = await fixture(page, baseURL!, source === "api-keys");
    await page.goto(path);
    const url = page.url();
    const action = page.locator("[data-csv-export]").first();
    await action.getByRole("button", { name: "Export CSV", exact: true }).click();
    await action.getByRole("button", { name: "Generate CSV", exact: true }).click();
    await expect.poll(() => requests.length).toBe(1);
    const request = requests[0]!;
    expect(request.body.source).toBe(source);
    expect(request.path).toBe(source === "api-keys" ? "/user/csv-exports" : "/group/csv-exports");
    expect(JSON.stringify(request.body)).not.toMatch(/never-export-token|secret:\/\/|pageSize/);
    expect(page.url()).toBe(url);
    if (source === "invitations") expect(request.body.boundary).toEqual({ kind: "explicit", recordIds: [id] });
    if (source === "maintenance") {
      expect(request.body.filters.status).toBe("scheduled");
      if (path.includes("/item/")) expect(request.body.filters.entityId).toBe(id);
    }
    if (source === "template-fields") expect(request.body.filters.templateId).toBe(id);
  });
}

for (const scope of ["private", "collection"] as const) {
  test(`${scope} history captures fixed membership before export`, async ({ page, baseURL }) => {
    const requests = await fixture(page, baseURL!, scope === "private");
    await page.setViewportSize({ width: 390, height: 1000 });
    await page.goto(scope === "private" ? "/profile" : "/collection/tools");
    await page
      .getByRole("button", {
        name: scope === "private" ? "Export private CSV history metadata" : "Export CSV history metadata",
        exact: true,
      })
      .click();
    const action = page.locator("[data-csv-history-export] [data-csv-export]");
    await action.getByRole("button", { name: "Export CSV", exact: true }).press("Enter");
    await action.getByRole("button", { name: "Generate CSV", exact: true }).click();
    await expect.poll(() => requests.length).toBe(1);
    expect(requests[0]!.body.source).toBe(`${scope}-csv-history`);
    expect(requests[0]!.body.scope).toBe(scope);
    expect(requests[0]!.body.boundary).toEqual({ kind: "explicit", recordIds: [oldHistoryId] });
    await action.getByRole("button", { name: "Download CSV", exact: true }).click();
    await expect(action.getByRole("status")).toContainText("The server rejected the download");
    expect(requests.length).toBe(1); // permission failure must not regenerate
  });
}

for (const [status, button] of [
  ["completed", "Completed"],
  ["both", "Both"],
] as const) {
  test(`maintenance captures ${status} at initiation without rewriting the snapshot`, async ({ page, baseURL }) => {
    const requests = await fixture(page, baseURL!);
    await page.goto("/maintenance");
    await page.getByRole("button", { name: button, exact: true }).click();
    const action = page.locator("[data-csv-export]").first();
    await action.getByRole("button", { name: "Export CSV", exact: true }).click();
    // A later source-filter edit must not silently change the initiated export.
    await page.getByRole("button", { name: "Scheduled", exact: true }).click();
    await action.getByRole("button", { name: "Generate CSV", exact: true }).click();
    await expect.poll(() => requests.length).toBe(1);
    expect(requests[0]!.body.filters).toEqual({ status });
    expect(requests[0]!.body.sort).toEqual([]);
  });
}
