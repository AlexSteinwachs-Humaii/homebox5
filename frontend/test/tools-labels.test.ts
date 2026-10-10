import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { compileTemplate, parse } from "vue/compiler-sfc";

const read = (path: string) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

describe("tools and label presentation", () => {
  it.each([
    "pages/collection/index/tools.vue",
    "pages/reports/label-generator.vue",
    "components/DetailAction.vue",
    "components/global/LabelMaker.vue",
  ])("compiles %s", path => {
    const { descriptor, errors } = parse(read(path));
    expect(errors).toEqual([]);
    expect(compileTemplate({ source: descriptor.template!.content, filename: path, id: path }).errors).toEqual([]);
  });

  it("provides a separate restore control instead of nesting buttons", () => {
    expect(read("components/DetailAction.vue")).toContain('<slot v-if="$slots.control" name="control" />');
    const tools = read("pages/collection/index/tools.vue");
    expect(tools).toContain("<template #control>");
    expect(tools).toContain('@change="onRestoreFile"');
    expect(tools).toContain('accept=".zip"');
    expect(tools).toContain("status === 409");
  });

  it("keeps sanitized instructions and physical label dimensions separate from screen settings", () => {
    const source = read("pages/reports/label-generator.vue");
    expect(source).toContain("hb-label-settings bg-background text-foreground print:hidden");
    expect(source).toContain("DOMPurify.sanitize($t('reports.label_generator.instruction_3'))");
    expect(source).toContain("height: `${out.card.height}${out.measure}`");
    expect(source).toContain("width: `${out.card.width}${out.measure}`");
    expect(source).toContain("background: `white`");
    expect(source).toContain("color: `black`");
    expect(source).toContain("itemsCopy.unshift(...Array.from({ length: skipLabels }, () => null))");
  });

  it("preserves legacy and asset shortcut redirects", () => {
    expect(read("pages/label/[id].vue")).toContain("navigateTo(`/tag/${tagId}`, { replace: true, redirectCode: 308 })");
    expect(read("pages/a/[id].vue")).toContain(
      'navigateTo("/assets/" + assetId.value, { replace: true, redirectCode: 301 })'
    );
    const asset = read("pages/assets/[id].vue");
    expect(asset).toContain("case 0:");
    expect(asset).toContain("case 1:");
    expect(asset).toContain("navigateTo(`/item/${data.items[0]!.id}`, { replace: true, redirectCode: 302 })");
  });
});
