import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { compileTemplate, parse } from "vue/compiler-sfc";
import { createSSRApp, defineComponent } from "vue";
import { renderToString } from "vue/server-renderer";
import { createI18n } from "vue-i18n";
import en from "../locales/en.json";

const read = (path: string) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");
const item = read("pages/item/[id]/index.vue");
const location = read("pages/location/[id]/index/index.vue");
const details = read("components/global/DetailsSection/DetailsSection.vue");

// Use the real templates, not a second implementation of the detail layout.
describe("record detail surfaces", () => {
  it.each([item, location, details])("compiles the detail template", source => {
    const { descriptor, errors } = parse(source);
    expect(errors).toEqual([]);
    expect(
      compileTemplate({ source: descriptor.template!.content, filename: "Detail.vue", id: "detail" }).errors
    ).toEqual([]);
  });

  it("renders zero/false, long field names and values, links, rich text, dates and money", async () => {
    const passthrough = defineComponent({ template: "<slot />" });
    const app = createSSRApp({
      template: parse(details).descriptor.template!.content,
      setup: () => ({
        details: [
          { name: "items.quantity", text: 0 },
          { name: "items.insured", text: false },
          { name: "LongCustomField".repeat(12), text: "UnbrokenValue".repeat(30), copyable: true },
          { name: "items.purchase_date", type: "date", text: "2026-01-01", date: true },
          { name: "items.purchase_price", type: "currency", text: "42.00" },
          { name: "items.notes", type: "markdown", text: "Recorded notes" },
          { name: "items.manual", type: "link", text: "LongLink".repeat(30), href: "https://example.com/manual" },
        ],
        maybeUrl: () => ({ isUrl: false }),
        badgeVariants: () => "badge",
      }),
    });
    app.use(createI18n({ legacy: false, locale: "en", messages: { en }, missingWarn: false, fallbackWarn: false }));
    for (const component of ["ClientOnly", "TooltipProvider", "Tooltip", "TooltipTrigger", "TooltipContent"]) {
      app.component(component, passthrough);
    }
    app.component("DateTime", defineComponent({ props: ["date"], template: "<time>{{ date }}</time>" }));
    app.component("Currency", defineComponent({ props: ["amount"], template: "<span>{{ amount }}</span>" }));
    app.component("Markdown", defineComponent({ props: ["source"], template: "<p>{{ source }}</p>" }));
    app.component("CopyText", defineComponent({ props: ["text"], template: "<button>{{ text }}</button>" }));
    app.component("MdiOpenInNew", defineComponent({ template: "<svg />" }));
    const html = await renderToString(app);
    for (const value of [
      "Quantity",
      ">0<",
      ">false<",
      "UnbrokenValue".repeat(30),
      "2026-01-01",
      "42.00",
      "Recorded notes",
    ]) {
      expect(html).toContain(value);
    }
    expect(html).toContain('href="https://example.com/manual"');
    expect(html).toContain('rel="noopener noreferrer"');
    expect(html).toContain("whitespace-normal");
    expect(html).toContain("min-w-0");
    expect(html).not.toContain("opacity-0");
  });

  it("retains detail groups, relationships and existing inventory refresh controls", () => {
    for (const binding of ["itemDetails", "attachmentDetails", "purchaseDetails", "warrantyDetails", "soldDetails"]) {
      expect(item).toContain(`:details="${binding}"`);
    }
    for (const source of [item, location]) {
      expect(source).toContain(':items="items" @refresh="refreshItemList"');
      expect(source).toContain("hb-panel");
      expect(source).toContain("font-editorial");
      expect(source).toContain("text-muted-foreground");
    }
    expect(item).toContain('v-for="tag in itemTags"');
    expect(item).toContain('v-for="(part, idx) in fullpath"');
    expect(location).toContain(':details="locationDetails"');
    expect(location).toContain('v-for="tag in location.tags"');
    expect(location).toContain(':to="`/location/${location.parent.id}`"');
    expect(location).toContain('v-for="child in location.children"');
  });

  it("retains action bindings and supplies visible keyboard/touch quantity controls", () => {
    for (const handler of [
      "createSubitem",
      "handleDuplicateClick",
      "saveAsTemplate",
      "deleteItem",
      "adjustQuantity(-1)",
      "adjustQuantity(1)",
    ]) {
      expect(item).toContain(`@click="${handler}"`);
    }
    for (const handler of ["openCreateItem", "goToEdit", "confirmDelete()"]) {
      expect(location).toContain(`@click="${handler}"`);
    }
    expect(item).toContain("items.decrease_quantity");
    expect(item).toContain("items.increase_quantity");
    expect(item).not.toContain("opacity-10");
    expect(location).toContain(":aria-label=\"$t('global.delete')\"");
    expect(location).toContain(":aria-label=\"$t('global.edit')\"");
    expect(item).toContain('v-if="!hasNested"');
    expect(item).toContain('v-model="preferences.showEmpty"');
  });
});
