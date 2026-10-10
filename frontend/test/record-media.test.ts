import { readFileSync } from "node:fs";
import { describe, expect, it, vi } from "vitest";
import { compileTemplate, parse } from "vue/compiler-sfc";
import ts from "typescript";
import { deletePhoto, setPrimaryPhoto, type PhotoPreview } from "../components/Form/photo-uploader";

const read = (path: string) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

function handler(source: string, name: string, context: Record<string, unknown>) {
  const ast = ts.createSourceFile("Media.ts", parse(source).descriptor.scriptSetup!.content, ts.ScriptTarget.Latest);
  const declaration = ast.statements.find(n => ts.isFunctionDeclaration(n) && n.name?.text === name)!;
  const js = ts.transpileModule(declaration.getText(ast), {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText;
  return new Function(...Object.keys(context), `${js}; return ${name};`)(...Object.values(context));
}

describe("record media", () => {
  it.each([
    "components/Item/AttachmentsList.vue",
    "components/Item/ImageDialog.vue",
    "components/Form/PhotoUploader.vue",
    "components/Form/PhotoUploaderPreview.vue",
    "components/global/DropZone.vue",
  ])("compiles %s", path => {
    const { descriptor, errors } = parse(read(path));
    expect(errors).toEqual([]);
    expect(compileTemplate({ source: descriptor.template!.content, filename: path, id: "media" }).errors).toEqual([]);
  });

  it("retains attachment edits on failure and enables retry with the same payload", async () => {
    const c = {
      editState: { id: "manual", title: "Draft title", type: "receipt", primary: false, loading: false },
      itemId: { value: "record" },
      item: { value: { attachments: [{ id: "manual" }] } },
      api: { items: { attachments: { update: vi.fn().mockResolvedValue({ error: new Error("offline") }) } } },
      toast: { error: vi.fn(), success: vi.fn() },
      t: (s: string) => s,
      closeDialog: vi.fn(),
      DialogID: { AttachmentEdit: "attachment-edit" },
    };
    const update = handler(read("pages/item/[id]/index/edit.vue"), "updateAttachment", c);
    await update();
    expect(c.editState).toEqual({
      id: "manual",
      title: "Draft title",
      type: "receipt",
      primary: false,
      loading: false,
    });
    expect(c.closeDialog).not.toHaveBeenCalled();
    expect(c.toast.error).toHaveBeenCalledWith("items.toast.failed_update_attachment");
    expect(c.item.value.attachments).toEqual([{ id: "manual" }]);
    c.api.items.attachments.update.mockResolvedValue({
      data: { attachments: [{ id: "manual", title: "Draft title" }] },
    });
    await update();
    expect(c.api.items.attachments.update).toHaveBeenLastCalledWith("record", "manual", {
      title: "Draft title",
      type: "receipt",
      primary: false,
    });
    expect(c.closeDialog).toHaveBeenCalledWith("attachment-edit");
    expect(c.editState.loading).toBe(false);
  });

  it("changes only primary metadata or the selected deletion, never image bytes", () => {
    const photos = [
      { photoName: "one.png", fileBase64: "original-one", primary: true },
      { photoName: "two.png", fileBase64: "original-two", primary: false },
    ] as PhotoPreview[];
    const selected = setPrimaryPhoto(photos, 1);
    expect(selected.map(p => p.primary)).toEqual([false, true]);
    expect(selected.map(p => p.fileBase64)).toEqual(photos.map(p => p.fileBase64));
    expect(deletePhoto(selected, 1)).toEqual([{ ...photos[0], primary: true }]);
    expect(photos[0]!.primary).toBe(true);
  });
});
