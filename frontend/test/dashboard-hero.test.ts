import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { parse, compileTemplate } from "vue/compiler-sfc";
import { createI18n } from "vue-i18n";
import { createSSRApp } from "vue";
import { renderToString } from "vue/server-renderer";
import en from "../locales/en.json";

const hero = readFileSync(new URL("../components/App/DashboardHero.vue", import.meta.url), "utf8");
const page = readFileSync(new URL("../pages/home/index.vue", import.meta.url), "utf8");

describe("dashboard hero integration", () => {
  it("compiles the original decorative artwork and localized heading", () => {
    const { descriptor, errors } = parse(hero);
    expect(errors).toEqual([]);
    const template = compileTemplate({
      source: descriptor.template!.content,
      filename: "DashboardHero.vue",
      id: "hero",
    });
    expect(template.errors).toEqual([]);
    expect(hero).toContain('aria-hidden="true"');
    expect(hero).toContain('focusable="false"');
    expect(hero).not.toMatch(/<image|<a\b|https?:\/\//);
    expect(hero).toContain('keypath="home.hero.title"');
    expect(hero).toContain("font-editorial");
    expect(hero).toContain("text-primary");
  });

  it("renders the translated title slot and artwork without an inventory dependency", async () => {
    const i18n = createI18n({ legacy: false, locale: "en", messages: { en } });
    const app = createSSRApp({
      template: parse(hero).descriptor.template!.content,
      setup: () => ({ t: i18n.global.t, id: "test-camera" }),
    });
    app.use(i18n);
    const html = await renderToString(app);
    expect(html).toContain("A place for");
    expect(html).toContain('<em class="font-editorial font-normal">everything</em>');
    expect(html).toContain(en.home.hero.description);
    expect(html).toContain('aria-hidden="true"');
    expect(html).toContain("url(#test-camera-glass)");
    expect(html).not.toContain("home.hero.");
  });

  it("falls back to English for new hero copy in other locales", () => {
    const i18n = createI18n({ legacy: false, locale: "de", fallbackLocale: "en", messages: { en, de: {} } });
    expect(i18n.global.t("home.hero.eyebrow")).toBe(en.home.hero.eyebrow);
    expect(i18n.global.t("home.hero.title", { everything: en.home.hero.everything })).toBe("A place for everything.");
    expect(i18n.global.t("home.hero.description")).toBe(en.home.hero.description);
  });

  it("keeps live sections, empty states and desktop/mobile record representations in order after the hero", () => {
    const sections = [
      "<DashboardHero />",
      "home.quick_statistics",
      "home.recently_added",
      "home.storage_locations",
      "home.tags",
    ];
    const positions = sections.map(section => page.indexOf(section));
    expect(positions.every(position => position >= 0)).toBe(true);
    expect(positions).toEqual([...positions].sort((a, b) => a - b));
    for (const binding of ["statCardData(api)", "itemsTable(api)", "locationStore.parentLocations", "tagsStore.tags"]) {
      expect(page).toContain(binding);
    }
    for (const empty of ["items.no_results", "locations.no_results", "tags.no_results"]) {
      expect(page).toContain(empty);
    }
    expect(page).toContain('<Table :items="itemTable.items" />');
    expect(page).toContain(':item="item"');
    expect(page).toContain(':location="location"');
    expect(page).toContain(':tag="tag"');
    expect(page).toContain(':type="stat.type"');
  });
});
