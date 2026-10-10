import { readFileSync } from "node:fs";
import { describe, expect, it, vi } from "vitest";
import { compileTemplate, parse } from "vue/compiler-sfc";
import { createTable, getCoreRowModel, getPaginationRowModel, getSortedRowModel } from "@tanstack/vue-table";
import type { VNode } from "vue";
import type { EntitySummary } from "../lib/api/types/data-contracts";

import { makeColumns } from "../components/Item/View/table/columns";

vi.mock("~/components/ui/checkbox/Checkbox.vue", () => ({ default: { name: "Checkbox" } }));
vi.mock("~/components/ui/button/Button.vue", () => ({ default: { name: "Button" } }));
vi.mock("../components/Item/View/table/data-table-dropdown.vue", () => ({ default: { name: "DropdownAction" } }));
vi.mock("~/components/global/Currency.vue", () => ({ default: { name: "Currency" } }));
vi.mock("~/components/global/DateTime.vue", () => ({ default: { name: "DateTime" } }));
vi.mock("~/lib/utils", () => ({ cn: (classes: unknown[]) => classes.filter(Boolean).join(" ") }));

const read = (file: string) => readFileSync(new URL(`../components/Item/View/${file}`, import.meta.url), "utf8");

describe("inventory browsing controls", () => {
  it.each([
    "Selectable.vue",
    "ItemChangeDetails.vue",
    "table/data-table.vue",
    "table/data-table-controls.vue",
    "table/table-view.vue",
    "table/card-view.vue",
    "table/data-table-dropdown.vue",
    "table/data-table-expanded-row.vue",
  ])("compiles %s", file => {
    const { descriptor, errors } = parse(read(file));
    expect(errors).toEqual([]);
    expect(compileTemplate({ source: descriptor.template!.content, filename: file, id: file }).errors).toEqual([]);
  });

  it("renders operable unchecked, mixed and checked selection, disables empty bulk actions, and preserves sorting", () => {
    const columns = makeColumns({ t: key => key });
    const table = createTable<EntitySummary>({
      data: [
        { id: "a", name: "Zulu" },
        { id: "b", name: "Alpha" },
      ] as EntitySummary[],
      columns,
      state: { rowSelection: {}, sorting: [], pagination: { pageIndex: 0, pageSize: 12 } },
      onStateChange: updater =>
        table.setOptions(prev => ({
          ...prev,
          state: typeof updater === "function" ? updater({ ...table.initialState, ...prev.state }) : updater,
        })),
      getCoreRowModel: getCoreRowModel(),
      getSortedRowModel: getSortedRowModel(),
      getPaginationRowModel: getPaginationRowModel(),
      renderFallbackValue: null,
    });
    table.setOptions(prev => ({ ...prev, state: { ...table.initialState, ...prev.state } }));
    const selection = () => {
      const header = table.getHeaderGroups()[0]!.headers.find(h => h.column.id === "select")!;
      return (header.column.columnDef.header as (context: unknown) => VNode)(header.getContext());
    };
    const bulk = () => {
      const header = table.getHeaderGroups()[0]!.headers.find(h => h.column.id === "actions")!;
      const wrapper = (header.column.columnDef.header as (context: unknown) => VNode)(header.getContext());
      return (wrapper.children as VNode[])[0]!;
    };
    expect(selection().props?.modelValue).toBe(false);
    expect(selection().props?.["aria-label"]).toBe("components.item.view.selectable.select_all");
    expect(bulk().props?.disabled).toBe(true);
    table.getRowModel().rows[0]!.toggleSelected(true);
    expect(selection().props?.modelValue).toBe("indeterminate");
    expect(bulk().props?.disabled).toBe(false);
    selection().props?.["onUpdate:modelValue"](true);
    expect(selection().props?.modelValue).toBe(true);
    expect(table.getSelectedRowModel().rows).toHaveLength(2);
    selection().props?.["onUpdate:modelValue"](false);
    expect(table.getSelectedRowModel().rows).toHaveLength(0);
    table.getColumn("name")!.toggleSorting(false);
    expect(table.getRowModel().rows.map(row => row.original.name)).toEqual(["Alpha", "Zulu"]);
  });

  it("retains saved state and external/internal paging handlers with wrapping controls and labelled settings", () => {
    const data = read("table/data-table.vue");
    for (const binding of [
      "preferences.value.itemsPerTablePage",
      "preferences.value.tableHeaders = headers",
      "manualPagination: !!props.externalPagination",
      "getPaginationRowModel: getPaginationRowModel()",
      "persistHeaders();",
    ]) {
      expect(data).toContain(binding);
    }
    const controls = read("table/data-table-controls.vue");
    expect(controls).toContain("props.table.resetRowSelection()");
    expect(controls).toContain("props.table.resetExpanded()");
    expect(controls).toContain("props.externalPagination.setPage(page)");
    expect(controls).toContain("props.table.setPageIndex(page - 1)");
    expect(controls).toContain("flex-wrap");
    expect(controls).toContain('role="status"');
    expect(data).toContain("`${settingsId}-page-size`");
    expect(data).toContain("move_column_up");
    expect(data).toContain("move_column_down");
    expect(read("table/card-view.vue")).toContain("compact");
    expect(read("table/data-table-expanded-row.vue")).not.toContain("/no-image.jpg");
  });
});
