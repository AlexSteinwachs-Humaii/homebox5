import { expect, test, type Page } from "@playwright/test";

const rootId = "11111111-1111-1111-1111-111111111111";
const childId = "22222222-2222-2222-2222-222222222222";
const longName = "Deep " + "UnbrokenOrganizationName".repeat(10);

async function fixture(page: Page, baseURL: string, theme: string, failed = false, empty = false) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  const tags = empty
    ? []
    : [
        { id: rootId, name: "Root tag", color: "#ffffff", icon: "palette-outline", parentId: null },
        { id: childId, name: longName, color: "#000000", icon: "palette-outline", parentId: rootId },
      ];
  await page.route("**/api/v1/**", route => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    if (path === "/status") return route.fulfill({ json: { build: { version: "v1.0.0" } } });
    if (path === "/users/self")
      return route.fulfill({
        json: {
          item: {
            id: "user",
            name: "Member",
            defaultGroupId: "group",
            group: { id: "group", name: "Collection", currency: "USD" },
          },
        },
      });
    if (path === "/users/self/settings")
      return route.fulfill({ json: { item: { theme, displayLegacyHeader: false } } });
    if (path === "/groups/all") return route.fulfill({ json: [{ id: "group", name: "Collection" }] });
    if (path === "/groups") return route.fulfill({ json: { id: "group", name: "Collection", currency: "USD" } });
    if (path === "/tags" || path === "/entities/tree") {
      if (failed) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
      return route.fulfill({
        json:
          path === "/tags"
            ? tags
            : empty
              ? []
              : [
                  {
                    id: rootId,
                    name: "Root location",
                    type: "location",
                    children: [
                      {
                        id: childId,
                        name: longName,
                        type: "location",
                        children: [
                          {
                            id: "33333333-3333-3333-3333-333333333333",
                            name: "Stored item",
                            type: "item",
                            children: [],
                          },
                        ],
                      },
                    ],
                  },
                ],
      });
    }
    return route.fulfill({ json: [] });
  });
}

for (const theme of ["cinematic", "light", "homebox"]) {
  for (const width of [390, 1440]) {
    for (const kind of ["tags", "locations"]) {
      test(`${kind} ${theme} ${width}: keyboard disclosure, links and query restore`, async ({ page, baseURL }) => {
        await page.setViewportSize({ width, height: 900 });
        await fixture(page, baseURL!, theme);
        await page.goto(`/${kind}`);
        const root = page.getByRole("button", {
          name: new RegExp(`Tree: Root ${kind === "tags" ? "tag" : "location"}$`),
          exact: true,
        });
        await expect(root).toHaveAttribute("aria-expanded", "false");
        await root.focus();
        await page.keyboard.press("Enter");
        await expect(root).toHaveAttribute("aria-expanded", "true");
        await expect(page.getByRole("link", { name: longName, exact: true })).toBeVisible();
        await expect(page.getByRole("link", { name: longName, exact: true })).toHaveAttribute(
          "href",
          `/${kind === "tags" ? "tag" : "location"}/${childId}`
        );
        await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        await expect(page).toHaveURL(new RegExp(`${kind === "tags" ? "tagTree" : "locationTree"}=`));
        await page.reload();
        await expect(page.getByRole("link", { name: longName, exact: true })).toBeVisible();
        await page.getByRole("button", { name: "Expand Tree", exact: true }).click();
        if (kind === "locations") {
          await expect(page.getByRole("link", { name: "Stored item", exact: true })).toBeVisible();
          await page.getByRole("button", { name: "Hide Items", exact: true }).click();
          await expect(page.getByRole("link", { name: "Stored item", exact: true })).toHaveCount(0);
          await expect(page).toHaveURL(/showItems=false/);
          await page.reload();
          await expect(page.getByRole("button", { name: "Show Items", exact: true })).toHaveAttribute(
            "aria-pressed",
            "false"
          );
        }
        await page.getByRole("button", { name: "Collapse Tree", exact: true }).click();
        await expect(page.getByRole("link", { name: longName, exact: true })).toHaveCount(0);
      });
    }
  }
}

for (const kind of ["tags", "locations"]) {
  for (const failed of [false, true]) {
    test(`${kind}: ${failed ? "failed request is not empty" : "empty hierarchy has create action"}`, async ({
      page,
      baseURL,
    }) => {
      await page.setViewportSize({ width: 390, height: 844 });
      await fixture(page, baseURL!, "cinematic", failed, true);
      await page.goto(`/${kind}`);
      if (failed) await expect(page.getByRole("alert")).toContainText("Unable to load this hierarchy");
      else await expect(page.getByRole("status").getByRole("button")).toBeVisible();
    });
  }
}

for (const theme of ["cinematic", "light", "homebox"]) {
  test(`tag details ${theme}: parent filtering, clear, failed save and canceled delete`, async ({
    page,
    baseURL,
  }, testInfo) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await fixture(page, baseURL!, theme);
    const record = {
      id: rootId,
      name: longName,
      color: "#ffffff",
      icon: "palette-outline",
      parentId: null,
      description: "Stored description",
      createdAt: "2026-01-01T00:00:00Z",
    };
    const other = {
      id: "44444444-4444-4444-4444-444444444444",
      name: "Available parent",
      color: "#000000",
      parentId: null,
    };
    let failSave = true;
    let deletions = 0;
    let failDelete = true;
    let saved: Record<string, unknown> = {};
    await page.route("**/api/v1/tags**", route => {
      const req = route.request();
      const path = new URL(req.url()).pathname;
      if (path.endsWith(rootId)) {
        if (req.method() === "PUT") {
          saved = req.postDataJSON();
          if (failSave) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
          Object.assign(record, saved);
        }
        if (req.method() === "DELETE") {
          deletions++;
          if (failDelete) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
        }
        return route.fulfill({ json: record });
      }
      return route.fulfill({ json: [record, other, { id: childId, name: "Descendant", parentId: rootId }] });
    });
    await page.route("**/api/v1/entities?**", route =>
      route.fulfill({ json: { items: [], total: 0, totalPrice: 12 } })
    );
    await page.goto("/tags");
    await page.getByRole("link", { name: longName, exact: true }).click();
    await expect(page.getByRole("heading", { name: longName })).toBeVisible();
    await expect(page.locator("header .rounded-full").first()).toHaveCSS("background-color", "rgb(255, 255, 255)");
    await expect(page.locator("header .rounded-full").first()).toHaveCSS("color", "rgb(0, 0, 0)");
    await expect(page.getByRole("heading", { name: longName })).toContainText("$12.00");
    expect((await page.getByRole("heading", { name: longName }).boundingBox())!.width).toBeGreaterThan(200);
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath("tag-detail-mobile.png"), fullPage: true });
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "Update Tag" });
    await dialog.getByRole("combobox", { name: "Parent Tag" }).click();
    await expect(page.getByRole("option", { name: "Descendant", exact: true })).toHaveCount(0);
    await expect(page.getByRole("option", { name: longName, exact: true })).toHaveCount(0);
    await page.getByRole("option", { name: "Available parent", exact: true }).click();
    await dialog.getByRole("button", { name: "Clear", exact: true }).click();
    await expect(dialog.getByRole("combobox", { name: "Parent Tag" })).toContainText("Select Tags");
    await dialog.getByRole("combobox", { name: "Parent Tag" }).click();
    await page.getByRole("option", { name: "Available parent", exact: true }).click();
    await dialog.getByRole("textbox", { name: "Tag Name", exact: true }).fill("Updated tag");
    await dialog.getByRole("button", { name: "Update", exact: true }).click();
    await expect(page.getByText("Failed to update tag", { exact: true })).toBeVisible();
    await expect(dialog).toBeVisible();
    expect(saved.parentId).toBe(other.id);
    expect(saved.color).toBe("#ffffff");
    expect(saved.icon).toBe("palette-outline");
    failSave = false;
    await dialog.getByRole("button", { name: "Update", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "Updated tag" })).toBeVisible();
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Cancel", exact: true }).click();
    expect(deletions).toBe(0);
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(page.getByText("Failed to delete tag", { exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Updated tag" })).toBeVisible();
    failDelete = false;
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(page).toHaveURL(/\/home$/);
    expect(deletions).toBe(2);
  });
}

test("tag creation retains form after failure and retries unchanged payload", async ({ page, baseURL }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await fixture(page, baseURL!, "cinematic", false, true);
  let failCreate = true;
  let payload: Record<string, unknown> = {};
  await page.route("**/api/v1/tags", route => {
    if (route.request().method() !== "POST") return route.fallback();
    payload = route.request().postDataJSON();
    return failCreate
      ? route.fulfill({ status: 500, json: { message: "Unavailable" } })
      : route.fulfill({ json: { ...payload, id: childId } });
  });
  await page.route(`**/api/v1/tags/${childId}`, route => route.fulfill({ json: { ...payload, id: childId } }));
  await page.goto("/tags");
  await page.getByRole("status").getByRole("button", { name: "Create Tag" }).click();
  const dialog = page.getByRole("dialog", { name: "Create Tag" });
  await dialog.getByRole("textbox", { name: "Tag Name", exact: true }).fill("Created tag");
  await dialog.getByRole("textbox", { name: "Tag Description", exact: true }).fill("Retained description");
  await dialog.getByRole("button", { name: "Create", exact: true }).click();
  await expect(page.getByText("Couldn't create tag", { exact: true })).toBeVisible();
  await expect(dialog.getByRole("textbox", { name: "Tag Name", exact: true })).toHaveValue("Created tag");
  failCreate = false;
  await dialog.getByRole("button", { name: "Create", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/tag/${childId}$`));
  expect(payload.name).toBe("Created tag");
  expect(payload.description).toBe("Retained description");
});

test("deep location hierarchy remains nested and readable without cumulative indentation", async ({
  page,
  baseURL,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await fixture(page, baseURL!, "cinematic");
  type Node = { id: string; name: string; type: string; children: Node[] };
  let node: Node = { id: "deepest", name: longName, type: "location", children: [] };
  for (let depth = 12; depth >= 0; depth--) {
    node = { id: `level${String(depth).padStart(3, "0")}`, name: `Level ${depth}`, type: "location", children: [node] };
  }
  await page.route("**/api/v1/entities/tree**", route => route.fulfill({ json: [node] }));
  await page.goto("/locations");
  await page.getByRole("button", { name: "Expand Tree", exact: true }).click();
  const deepest = page.getByRole("link", { name: longName, exact: true });
  await expect(deepest).toBeVisible();
  expect((await deepest.boundingBox())!.width).toBeGreaterThan(180);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await expect(page.locator("li").filter({ has: deepest })).toHaveCount(14);
});
