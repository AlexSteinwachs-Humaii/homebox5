import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { compileTemplate, parse } from "vue/compiler-sfc";
import { createSSRApp, defineComponent } from "vue";
import { renderToString } from "vue/server-renderer";
import { createI18n } from "vue-i18n";
import en from "../locales/en.json";

const read = (path: string) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");
const paths = [
  "components/App/ScannerModal.vue",
  "components/Scanner/ARControls.vue",
  "components/Scanner/ARView.vue",
  "components/Scanner/AROverlayCard.vue",
  "components/Item/BarcodeModal.vue",
  "components/App/ImportDialog.vue",
];

async function render(path: string, state: Record<string, unknown>) {
  const i18n = createI18n({ legacy: false, locale: "en", messages: { en } });
  const app = createSSRApp({
    template: parse(read(path)).descriptor.template!.content,
    setup: () => ({
      t: i18n.global.t,
      openArMode: () => {},
      handleButtonClick: () => {},
      handleClick: () => {},
      ...state,
    }),
  });
  app.use(i18n);
  const slot = defineComponent({ template: "<div><slot /></div>" });
  for (const name of [
    "Dialog",
    "DialogScrollContent",
    "DialogHeader",
    "DialogTitle",
    "ButtonGroup",
    "Select",
    "SelectTrigger",
    "SelectValue",
    "SelectContent",
    "SelectItem",
    "Badge",
    "Skeleton",
    "ScannerARControls",
    "ScannerAROverlayCard",
  ])
    app.component(name, slot);
  app.component("Button", defineComponent({ template: "<button><slot /></button>" }));
  for (const name of ["MdiBarcode", "MdiAlertCircleOutline", "MdiArrowLeft", "MdiCameraFlip", "MdiCameraOutline"])
    app.component(name, defineComponent({ template: "<svg />" }));
  return renderToString(app);
}

describe("scanner presentation", () => {
  it.each(paths)("compiles %s", path => {
    const { descriptor, errors } = parse(read(path));
    expect(errors).toEqual([]);
    expect(compileTemplate({ source: descriptor.template!.content, filename: path, id: path }).errors).toEqual([]);
  });

  it.each([en.scanner.permission_denied, en.scanner.unsupported, en.scanner.error, en.scanner.no_sources])(
    "renders readable modal feedback: %s",
    async errorMessage => {
      const html = await render(paths[0]!, {
        DialogID: { Scanner: "scanner" },
        errorMessage,
        detectedBarcode: "",
        sources: [],
        selectedSource: null,
      });
      expect(html).toContain(errorMessage);
      expect(html).toContain('role="alert"');
      expect(html).toContain("hb-scanner-feedback");
      expect(html).toContain('for="scanner-source"');
      expect(html).toContain("AR Mode");
    }
  );

  it("renders barcode detection and its existing fetch action", async () => {
    const html = await render(paths[0]!, {
      DialogID: { Scanner: "scanner" },
      errorMessage: null,
      detectedBarcode: "1234567890123",
      detectedBarcodeType: "EAN-13",
      loading: false,
      sources: [],
      selectedSource: null,
    });
    expect(html).toContain("1234567890123");
    expect(html).toContain("Fetch product data");
    expect(html).toContain('role="status"');
  });

  it("renders scanning status and labeled camera controls without translucent surfaces", async () => {
    const html = await render(paths[1]!, { isScanning: true });
    expect(html).toContain("Scanning");
    expect(html).toContain('aria-label="Back"');
    expect(html).toContain("Switch Camera");
    expect(html).toContain("bg-card");
    expect(html).not.toContain("bg-background/60");
  });

  it("renders failed and named result overlays on opaque shared surfaces", async () => {
    const failed = await render(paths[3]!, {
      canNavigate: false,
      loading: false,
      error: true,
      entity: null,
      cardStyle: {},
    });
    expect(failed).toContain("Item not found");
    expect(failed).toContain('role="status"');
    const result = await render(paths[3]!, {
      canNavigate: true,
      loading: false,
      error: false,
      cardStyle: {},
      resultName: "Inventory camera",
      entity: { item: { id: "item", name: "Inventory camera", quantity: 1 } },
    });
    expect(result).toContain('aria-label="Open result: Inventory camera"');
    expect(result).toContain('tabindex="0"');
    expect(result).toContain("hb-scanner-surface");
  });

  it("keeps overlay and product selection keyboard accessible and isolates selection styling", () => {
    const overlay = read(paths[3]!);
    expect(overlay).toContain('@keydown.enter.prevent="handleClick"');
    expect(overlay).toContain('@keydown.space.prevent="handleClick"');
    expect(overlay).toContain(':tabindex="canNavigate ? 0 : undefined"');
    const barcode = read(paths[4]!);
    expect(barcode).toContain('@click.stop="selectProduct(index)"');
    expect(barcode).toContain(':aria-pressed="selectedRow === index"');
    expect(barcode).toContain('v-model="barcode"');
    expect(barcode).toContain("sm:flex-row");
    expect(barcode).not.toContain("tr.selected");
  });
});
