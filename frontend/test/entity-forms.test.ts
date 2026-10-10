import { readFileSync } from "node:fs";
import { describe, expect, it, vi } from "vitest";
import { compileTemplate, parse } from "vue/compiler-sfc";
import ts from "typescript";
import type { PhotoPreview } from "../components/Form/photo-uploader";

const read = (path: string) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");
const create = read("components/Entity/CreateModal.vue");
const item = read("pages/item/[id]/index/edit.vue");
const location = read("pages/location/[id]/index/edit.vue");

// Execute the actual handler from the SFC, with API/composable boundaries stubbed.
// This checks payloads without maintaining a second implementation of the forms.
function handler(source: string, name: string, context: Record<string, unknown>) {
  const script = parse(source).descriptor.scriptSetup!.content;
  const ast = ts.createSourceFile("Form.ts", script, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  const declaration = ast.statements.find(n => ts.isFunctionDeclaration(n) && n.name?.text === name)!;
  const js = ts.transpileModule(declaration.getText(ast), {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText;
  return new Function(...Object.keys(context), `${js}; return ${name};`)(...Object.values(context));
}

function createContext() {
  return {
    form: {
      name: "Draft",
      quantity: 3,
      description: "Description",
      location: { id: "room" },
      parentId: "parent",
      tags: ["tag"],
      manufacturer: "Maker",
      modelNumber: "Model",
      photos: [],
      color: "",
    },
    pendingCreatedEntity: { value: null },
    selectedEntityType: { value: { id: "item-type", isLocation: false } },
    templateData: { value: null as null | { id: string } },
    selectedTemplate: { value: null },
    templateUserSelected: { value: false },
    showTemplateDetails: { value: false },
    focused: { value: false },
    loading: { value: false },
    shift: { value: false },
    api: {
      items: {
        create: vi.fn().mockResolvedValue({ data: { id: "created" } }),
        createLocation: vi.fn().mockResolvedValue({ data: { id: "created" } }),
      },
      templates: { createItem: vi.fn().mockResolvedValue({ data: { id: "created" } }) },
    },
    toast: { error: vi.fn(), success: vi.fn() },
    t: (s: string) => s,
    navigateTo: vi.fn(),
    closeDialog: vi.fn(),
    DialogID: { CreateEntity: "create" },
    restoreLastTemplate: vi.fn(),
  };
}

describe("entity form presentation and unchanged behavior", () => {
  it("retains only failed photos and retries the saved record without creating it again", async () => {
    const original = createContext();
    const first = { photoName: "one.png", fileBase64: "original-one", primary: true } as PhotoPreview;
    const second = { photoName: "two.png", fileBase64: "original-two", primary: false } as PhotoPreview;
    const upload = vi.fn().mockResolvedValueOnce({}).mockResolvedValueOnce({ error: "offline" }).mockResolvedValue({});
    const c = {
      ...original,
      pendingCreatedEntity: { value: null as { id: string } | null },
      form: { ...original.form, photos: [first, second] },
      api: { ...original.api, items: { ...original.api.items, attachments: { add: upload } } },
      toast: { ...original.toast, info: vi.fn(), warning: vi.fn() },
      AttachmentTypes: { Photo: "photo" },
    };
    const log = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      const createRecord = handler(create, "create", c);
      await createRecord(false);
      expect(c.api.items.create).toHaveBeenCalledTimes(1);
      expect(c.pendingCreatedEntity.value).toEqual({ id: "created" });
      expect(c.form.photos).toEqual([second]);
      expect(c.form.name).toBe("Draft");
      expect(c.loading.value).toBe(false);
      expect(c.closeDialog).not.toHaveBeenCalled();
      await createRecord(false);
      expect(c.api.items.create).toHaveBeenCalledTimes(1);
      expect(upload).toHaveBeenCalledTimes(3);
      expect(upload.mock.calls.map(args => args[2])).toEqual(["one.png", "two.png", "two.png"]);
      expect(c.pendingCreatedEntity.value).toBeNull();
      expect(c.form.photos).toEqual([]);
      expect(c.form.name).toBe("");
    } finally {
      log.mockRestore();
    }
  });

  it.each([
    create,
    item,
    location,
    ...["TextField", "TextArea", "MarkdownEditor", "DatePicker", "Checkbox"].map(name =>
      read(`components/Form/${name}.vue`)
    ),
  ])("compiles the real form template", source => {
    const { descriptor, errors } = parse(source);
    expect(errors).toEqual([]);
    expect(compileTemplate({ source: descriptor.template!.content, filename: "Form.vue", id: "form" }).errors).toEqual(
      []
    );
  });

  it("creates items with existing parent, tags, defaults and redirects", async () => {
    const c = createContext();
    await handler(create, "create", c)();
    expect(c.api.items.create).toHaveBeenCalledWith({
      parentId: "parent",
      name: "Draft",
      quantity: 3,
      description: "Description",
      manufacturer: "Maker",
      modelNumber: "Model",
      tagIds: ["tag"],
      entityTypeId: "item-type",
    });
    expect(c.navigateTo).toHaveBeenCalledWith("/item/created");
    expect(c.form.name).toBe("");
    expect(c.form.quantity).toBe(1);
  });

  it("retains optional parent for location creation and location navigation", async () => {
    const c = createContext();
    c.selectedEntityType.value = { id: "location-type", isLocation: true };
    c.form.location = null as never;
    await handler(create, "create", c)();
    expect(c.api.items.createLocation).toHaveBeenCalledWith({
      parentId: null,
      name: "Draft",
      quantity: 1,
      description: "Description",
      tagIds: ["tag"],
      entityTypeId: "location-type",
    });
    expect(c.navigateTo).toHaveBeenCalledWith("/location/created");
  });

  it("retains template endpoint and create-and-add template restoration", async () => {
    const c = createContext();
    c.templateData.value = { id: "template" };
    await handler(create, "create", c)(false);
    expect(c.api.templates.createItem).toHaveBeenCalledWith("template", {
      name: "Draft",
      description: "Description",
      parentId: "room",
      tagIds: ["tag"],
      quantity: 3,
      entityTypeId: "item-type",
    });
    expect(c.closeDialog).not.toHaveBeenCalled();
    expect(c.navigateTo).not.toHaveBeenCalled();
    expect(c.restoreLastTemplate).toHaveBeenCalledOnce();
  });

  it("retains item location validation and failure values", async () => {
    const c = createContext();
    c.form.location = null as never;
    await handler(create, "create", c)();
    expect(c.api.items.create).not.toHaveBeenCalled();
    expect(c.toast.error).toHaveBeenCalledWith("components.entity.create_modal.toast.please_select_location");
    c.form.location = { id: "room" };
    c.api.items.create.mockResolvedValue({ error: true } as never);
    await handler(create, "create", c)();
    expect(c.form.name).toBe("Draft");
    expect(c.loading.value).toBe(false);
    expect(c.closeDialog).not.toHaveBeenCalled();
  });

  it("retains field groups, constraints, selectors and save shortcuts", () => {
    for (const source of [item, location]) {
      expect(source).toContain('ref: "name"');
      expect(source).toContain("minLength: 1");
      expect(source).toContain("maxLength: 255");
      expect(source).toContain("item.fields");
      expect(source).toContain("<LocationSelector");
      expect(source).toContain("<TagSelector");
      expect(source).toContain("<EntitySelector");
      expect(source).toContain("keyboardSave");
      expect(source).toContain("!e.shiftKey");
      expect(source).toContain("entityTypeId: item.value.entityType!.id");
    }
    for (const group of ["mainFields", "purchaseFields", "warrantyFields", "soldFields"])
      expect(item).toContain(`in ${group}`);
    expect(item).toContain('confirm.open(t("items.edit.change_entity_type_confirm"))');
    expect(item).toContain("parentId: parent.value?.id || location.value?.id || null");
    expect(location).toContain("parentId: parent.value?.id || null");
    expect(create).toContain("clearTemplate");
    expect(create).toContain("subItemCreate");
  });
});
