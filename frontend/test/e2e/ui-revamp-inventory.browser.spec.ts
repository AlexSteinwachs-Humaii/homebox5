import { expect, test, type Page } from "@playwright/test";

const longName = "Inventory " + "LongUnbrokenLabel".repeat(8);
const records = Array.from({ length: 30 }, (_, i) => ({
  id: `record-${i}`,
  name: i === 0 ? longName : `Record ${String(i).padStart(2, "0")}`,
  quantity: i + 1,
  insured: i % 2 === 0,
  purchasePrice: i,
  assetId: String(i + 1).padStart(6, "0"),
  tags: [],
  parent: null,
  imageId: null,
  thumbnailId: null,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
}));

// Deterministic, isolated API responses: mutations never touch the demo collection.
async function fixture(page: Page, baseURL: string, theme: string) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  let settings: Record<string, unknown> = { theme, displayLegacyHeader: false };
  let items = [...records];
  const requests: URLSearchParams[] = [];
  const mutations: string[] = [];
  await page.route("**/api/v1/**", route => {
    const req = route.request();
    const url = new URL(req.url());
    const path = url.pathname.replace("/api/v1", "");
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
    if (path === "/users/self/settings") {
      if (req.method() === "PUT") settings = req.postDataJSON();
      return route.fulfill({ json: { item: settings } });
    }
    if (path === "/groups/all") return route.fulfill({ json: [{ id: "group", name: "Collection" }] });
    if (path === "/groups") return route.fulfill({ json: { id: "group", name: "Collection", currency: "USD" } });
    if (path === "/entities") {
      if (url.searchParams.get("isLocation") === "true") return route.fulfill({ json: { items: [], total: 0 } });
      requests.push(url.searchParams);
      const embedded = url.searchParams.has("tags") || url.searchParams.has("parentIds");
      const pageNumber = Number(url.searchParams.get("page") || 1);
      const size = Number(url.searchParams.get("pageSize") || items.length);
      return route.fulfill({
        json: {
          items: embedded ? items : items.slice((pageNumber - 1) * size, pageNumber * size),
          total: items.length,
          page: pageNumber,
          pageSize: size,
        },
      });
    }
    if (req.method() === "DELETE" && path.startsWith("/entities/")) {
      const id = path.split("/").at(-1)!;
      mutations.push(id);
      items = items.filter(item => item.id !== id);
      return route.fulfill({ json: {} });
    }
    if (path === "/entities/parent")
      return route.fulfill({
        json: {
          ...records[0],
          id: "parent",
          name: "Parent record",
          fields: [],
          attachments: [],
          children: [],
          description: "",
          notes: "",
          purchaseTime: "0001-01-01",
          soldTime: "0001-01-01",
          warrantyExpires: "0001-01-01",
        },
      });
    if (path === "/tags/tag")
      return route.fulfill({
        json: { id: "tag", name: "Test tag", description: "", color: "", icon: "", parentId: null },
      });
    return route.fulfill({ json: [] });
  });
  return { requests, mutations, settings: () => settings };
}

const settingsButton = (page: Page) => page.getByRole("button", { name: "Table Settings", exact: true }).first();
const tableRows = (page: Page) => page.locator("tbody > tr");

for (const theme of ["cinematic", "light", "homebox"]) {
  for (const width of [390, 834, 1440]) {
    test(`${theme} ${width}px saved views, columns, ordering and keyboard selection`, async ({
      page,
      baseURL,
    }, info) => {
      await page.setViewportSize({ width, height: 1000 });
      const api = await fixture(page, baseURL!, theme);
      await page.goto("/items?q=inventory&archived=true&onlyWithoutPhoto=true");
      await expect(page.getByRole("checkbox", { name: new RegExp(longName) })).toBeVisible();
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      await page.getByRole("button", { name: "Table", exact: true }).press("Enter");
      const settings = settingsButton(page);
      await settings.press("Enter");
      const dialog = page.getByRole("dialog");
      await dialog.getByRole("checkbox", { name: "Asset ID", exact: true }).press("Space");
      await dialog.getByRole("button", { name: "Move Quantity up", exact: true }).press("Enter");
      await dialog.getByRole("combobox", { name: "Rows per page" }).click();
      await page.getByRole("option", { name: "24", exact: true }).click();
      await expect(dialog.getByRole("combobox", { name: "Rows per page" })).toHaveAttribute("aria-expanded", "false");
      await dialog.getByRole("button", { name: "Close", exact: true }).press("Enter");
      await expect(settings).toBeFocused();
      await expect(tableRows(page)).toHaveCount(24);
      await expect.poll(() => api.settings().itemsPerTablePage).toBe(24);
      await expect
        .poll(() =>
          page.evaluate(() =>
            JSON.parse(localStorage.getItem("homebox/preferences/location")!)
              .tableHeaders.slice(0, 3)
              .map((h: { value: string }) => h.value)
          )
        )
        .toEqual(["assetId", "quantity", "name"]);
      await page.reload();
      await expect(page.getByRole("button", { name: "Table", exact: true })).toHaveAttribute("aria-pressed", "true");
      await expect(page.locator("thead th").nth(2)).toContainText("Quantity");
      await expect(page.locator("thead th").nth(1)).toContainText("Asset ID");
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      await expect(page.locator("[data-inventory-search] input[type=search]")).toHaveValue("inventory");
      const selection = page.getByRole("checkbox", { name: `Select Row: ${longName}`, exact: true });
      await selection.press("Space");
      await expect(page.getByRole("checkbox", { name: "Select All", exact: true })).toHaveAttribute(
        "aria-checked",
        "mixed"
      );
      await expect(page.getByRole("status").filter({ hasText: "1 of 24" }).first()).toBeVisible();
      await page.getByRole("checkbox", { name: "Select All", exact: true }).press("Space");
      await expect(page.locator("tbody tr[data-state=selected]")).toHaveCount(24);
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await expect
        .poll(() => page.locator("thead").evaluate(el => el.getBoundingClientRect().height))
        .toBeLessThan(100);
      await page.screenshot({ path: info.outputPath("table-selected.png"), fullPage: true });
      await page.getByRole("button", { name: "Card", exact: true }).press("Enter");
      await expect(page.getByRole("checkbox", { name: "Select All", exact: true })).toBeChecked();
      await page.reload();
      await expect(page.getByRole("button", { name: "Card", exact: true })).toHaveAttribute("aria-pressed", "true");
      const card = page.getByRole("link", { name: longName, exact: true }).first();
      await card.focus();
      await expect.poll(() => card.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
      await expect(page.getByRole("checkbox", { name: `Select Card: ${longName}`, exact: true })).not.toBeChecked();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.screenshot({ path: info.outputPath("cards-missing-photos.png"), fullPage: true });
    });
  }
}

test("external pagination clears selection; bulk delete cancellation returns focus and success refreshes", async ({
  page,
  baseURL,
}) => {
  const api = await fixture(page, baseURL!, "cinematic");
  await page.goto("/items");
  await page.getByRole("button", { name: "Table", exact: true }).click();
  await page.getByRole("checkbox", { name: `Select Row: ${longName}`, exact: true }).press("Space");
  const bulk = page.locator("thead").getByRole("button", { name: "Open menu" });
  await bulk.press("Enter");
  for (const name of [
    "View items",
    "Toggle Expand",
    "Change Location",
    "Change Tags",
    "Create Maintenance Entry for Selected Items",
    "Duplicate Selected Items",
    "Delete Selected Items",
    "Download Table as CSV",
    "Download Table as JSON",
  ]) {
    await expect(page.getByRole("menuitem", { name, exact: true })).toBeEnabled();
  }
  await page.getByRole("menuitem", { name: "Delete Selected Items", exact: true }).click();
  const confirm = page.getByRole("alertdialog");
  await expect(confirm).toBeVisible();
  await confirm.getByRole("button", { name: "Cancel", exact: true }).press("Enter");
  await expect(bulk).toBeFocused();
  expect(api.mutations).toEqual([]);
  await bulk.press("Enter");
  await page.getByRole("menuitem", { name: "Delete Selected Items", exact: true }).click();
  await confirm.getByRole("button", { name: "Confirm", exact: true }).press("Enter");
  await expect.poll(() => api.mutations).toEqual(["record-0"]);
  await expect(page.getByRole("checkbox", { name: `Select Row: ${longName}`, exact: true })).toHaveCount(0);
  await page.getByRole("checkbox", { name: "Select Row: Record 01", exact: true }).press("Space");
  await page.getByRole("button", { name: "Page 2", exact: true }).first().click();
  await expect.poll(() => api.requests.at(-1)?.get("page")).toBe("2");
  await expect(page.locator("tbody tr[data-state=selected]")).toHaveCount(0);
});

for (const width of [390, 834, 1440]) {
  for (const path of ["/tag/tag", "/item/parent"]) {
    test(`${width}px ${path} shared list retains local sorting and internal pagination`, async ({ page, baseURL }) => {
      await page.setViewportSize({ width, height: 1000 });
      const api = await fixture(page, baseURL!, "cinematic");
      await page.goto(path);
      await page.getByRole("button", { name: "Table", exact: true }).click();
      await expect(tableRows(page)).toHaveCount(12);
      await page.getByRole("columnheader", { name: "Quantity", exact: true }).getByRole("button").press("Enter");
      await expect(page.getByRole("columnheader", { name: "Quantity", exact: true })).toHaveAttribute(
        "aria-sort",
        "ascending"
      );
      await page.getByRole("columnheader", { name: "Quantity", exact: true }).getByRole("button").press("Enter");
      await expect(tableRows(page).first()).toContainText("Record 29");
      await page.getByRole("checkbox", { name: "Select Row: Record 29", exact: true }).press("Space");
      const requests = api.requests.length;
      await page.getByRole("button", { name: "Page 2", exact: true }).first().click();
      await expect(tableRows(page).first()).toContainText("Record 17");
      expect(api.requests.length).toBe(requests);
      await expect(page.getByRole("status").filter({ hasText: "1 of 30" }).first()).toBeVisible();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.getByRole("button", { name: "Card", exact: true }).click();
      await expect(page.getByRole("checkbox", { name: "Select All", exact: true })).toHaveAttribute(
        "aria-checked",
        "mixed"
      );
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    });
  }
}

for (const theme of ["cinematic", "homebox", "light"]) {
  test(`${theme} rendered inventory text, selected states, hover and focus contrast`, async ({
    page,
    baseURL,
  }, info) => {
    await fixture(page, baseURL!, theme);
    await page.goto("/items");
    await page.getByRole("button", { name: "Table", exact: true }).click();
    const row = tableRows(page).first();
    await row.getByRole("checkbox").press("Space");
    await row.hover();
    const contrast = async (selector: string, token?: string) =>
      page
        .locator(selector)
        .first()
        .evaluate((el, token) => {
          const rgb = (color: string) => color.match(/[\d.]+/g)!.map(Number);
          const composite = (fg: number[], bg: number[]) =>
            fg.slice(0, 3).map((v, i) => v * (fg[3] ?? 1) + bg[i]! * (1 - (fg[3] ?? 1)));
          const ancestors: Element[] = [];
          for (let node: Element | null = el; node; node = node.parentElement) ancestors.unshift(node);
          let background = [255, 255, 255];
          for (const node of ancestors) background = composite(rgb(getComputedStyle(node).backgroundColor), background);
          let color = getComputedStyle(el).color;
          if (token === "border") color = getComputedStyle(el).borderTopColor;
          else if (token) {
            const probe = document.createElement("span");
            probe.style.color = `hsl(var(--${token}))`;
            el.append(probe);
            color = getComputedStyle(probe).color;
            probe.remove();
          }
          const foreground = composite(rgb(color), background);
          const luminance = (color: number[]) =>
            color
              .map(v => v / 255)
              .map(v => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
              .reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i]!, 0);
          const [lo, hi] = [luminance(foreground), luminance(background)].sort((a, b) => a - b);
          return { foreground, background, ratio: (hi! + 0.05) / (lo! + 0.05) };
        }, token);
    const results = [];
    for (const selector of [
      "thead th button",
      "tbody tr[data-state=selected] td a",
      "[role=status]",
      "[data-inventory-search] input[type=search]",
    ]) {
      const result = await contrast(selector);
      results.push({ selector, ...result });
      expect(result.ratio, selector).toBeGreaterThanOrEqual(4.5);
    }
    const searchButton = page.locator("[data-inventory-search]").getByRole("button", { name: "Search", exact: true });
    await searchButton.hover();
    results.push({ selector: "search button hover", ...(await contrast("[data-inventory-search] button")) });
    expect(results.at(-1)!.ratio).toBeGreaterThanOrEqual(4.5);
    // Resolve focus/checked-control tokens from the loaded theme, never copied RGB constants.
    const headerRing = await contrast("thead th", "ring");
    results.push({ selector: "header focus ring", ...headerRing });
    expect(headerRing.ratio).toBeGreaterThanOrEqual(3);
    const ring = await page.evaluate(() => {
      const probe = document.createElement("span");
      document.body.append(probe);
      const rgb = (token: string) => {
        probe.style.color = `hsl(var(--${token}))`;
        return getComputedStyle(probe)
          .color.match(/[\d.]+/g)!
          .slice(0, 3)
          .map(Number);
      };
      const lum = (color: number[]) =>
        color
          .map(v => v / 255)
          .map(v => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
          .reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i]!, 0);
      const ratios = ["background", "card", "muted", "popover"].map(surface => {
        const [lo, hi] = [lum(rgb("ring")), lum(rgb(surface))].sort((a, b) => a - b);
        return { surface, ratio: (hi! + 0.05) / (lo! + 0.05) };
      });
      probe.remove();
      return ratios;
    });
    for (const result of ring) expect(result.ratio, result.surface).toBeGreaterThanOrEqual(3);
    await page.getByRole("button", { name: "Card", exact: true }).click();
    const cardSelection = page.getByRole("checkbox", { name: `Select Card: ${longName}`, exact: true });
    await cardSelection.press("Space");
    await expect(cardSelection).not.toBeChecked();
    const cardBorder = await contrast("[role=checkbox][aria-label^='Select Card']", "border");
    expect(cardBorder.ratio).toBeGreaterThanOrEqual(3);
    await cardSelection.press("Space");
    await expect(cardSelection).toBeChecked();
    const cardChecked = await contrast("[role=checkbox][aria-label^='Select Card']");
    expect(cardChecked.ratio).toBeGreaterThanOrEqual(3);
    results.push(
      { selector: "card selection border", ...cardBorder },
      { selector: "card checked indicator", ...cardChecked }
    );
    await page.getByRole("button", { name: "Table", exact: true }).click();
    await info.attach("contrast.json", {
      body: JSON.stringify({ results, ring }, null, 2),
      contentType: "application/json",
    });
    await settingsButton(page).press("Enter");
    const column = page.getByRole("dialog").getByRole("checkbox", { name: "Location", exact: true });
    await column.focus();
    await expect.poll(() => column.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
    await page.keyboard.press("Escape");
    await expect(settingsButton(page)).toBeFocused();
  });
}

test("seeded backend: authenticated filters, API ordering and bulk duplicate/delete persist", async ({ page }) => {
  test.slow();
  const email = `inventory-${Date.now()}-${test.info().workerIndex}@example.com`;
  const password = "InventoryRegression42!";
  const registered = await page.request.post("/api/v1/users/register", {
    data: { email, password, name: "Inventory regression", token: "" },
  });
  expect(registered.status()).toBe(204);
  const login = await page.request.post("/api/v1/users/login", { data: { username: email, password } });
  expect(login.ok()).toBe(true);
  const { token } = await login.json();
  const headers = { Authorization: token };
  const types = await (await page.request.get("/api/v1/entity-types", { headers })).json();
  const entityTypeId = types.find((type: { isLocation: boolean }) => !type.isLocation).id;
  try {
    for (let i = 0; i < 14; i++) {
      const result = await page.request.post("/api/v1/entities", {
        headers,
        data: {
          name: `Seed inventory ${String(i).padStart(2, "0")}`,
          description: "No photo",
          quantity: 1,
          entityTypeId,
          tagIds: [],
        },
      });
      expect(result.ok(), await result.text()).toBe(true);
    }
    await page.context().clearCookies();
    await page.goto("/");
    await page.locator("input[type=text]").fill(email);
    await page.locator("input[type=password]").fill(password);
    await page.locator("button[type=submit]").click();
    await expect(page).toHaveURL(/\/home\/?$/);
    await page.goto("/items?q=Seed%20inventory&onlyWithoutPhoto=true");
    await page.getByRole("button", { name: "Table", exact: true }).click();
    await expect(tableRows(page)).toHaveCount(12);
    await page.getByRole("button", { name: "Options", exact: true }).click();
    const order = page.getByRole("combobox", { name: "Order By", exact: true });
    await order.click();
    await page.getByRole("option", { name: "Updated At", exact: true }).click();
    await expect.poll(() => new URL(page.url()).searchParams.get("orderBy")).toBe("updatedAt");
    await order.click();
    await page.getByRole("option", { name: "Name", exact: true }).click();
    await page.getByRole("button", { name: "Options", exact: true }).click();
    await page.locator("[data-inventory-search]").getByRole("button", { name: "Search", exact: true }).click();
    await expect(tableRows(page).first()).toContainText("Seed inventory 00");
    await page.reload();
    await expect(tableRows(page).first()).toContainText("Seed inventory 00");
    await page.getByRole("checkbox", { name: "Select Row: Seed inventory 00", exact: true }).press("Space");
    const bulk = page.locator("thead").getByRole("button", { name: "Open menu" });
    await bulk.press("Enter");
    await page.getByRole("menuitem", { name: "Duplicate Selected Items", exact: true }).click();
    const count = async () =>
      (await (await page.request.get("/api/v1/entities?q=Seed%20inventory", { headers })).json()).total;
    await expect.poll(count).toBe(15);
    await page.getByRole("checkbox", { name: "Select Row: Seed inventory 00", exact: true }).press("Space");
    await bulk.press("Enter");
    await page.getByRole("menuitem", { name: "Delete Selected Items", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).press("Enter");
    await expect.poll(count).toBe(14);
  } finally {
    // Account is disposable; remove all seeded/duplicated inventory even after an assertion failure.
    const all = await (await page.request.get("/api/v1/entities?pageSize=100", { headers })).json();
    for (const item of all.items ?? []) await page.request.delete(`/api/v1/entities/${item.id}`, { headers });
  }
});
