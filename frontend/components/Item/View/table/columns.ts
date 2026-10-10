import type { Column, ColumnDef } from "@tanstack/vue-table";
import { h } from "vue";
import DropdownAction from "./data-table-dropdown.vue";
import { ArrowDown, ArrowUpDown, Check, X } from "lucide-vue-next";
import Button from "~/components/ui/button/Button.vue";
import Checkbox from "~/components/ui/checkbox/Checkbox.vue";
import type { EntitySummary } from "~/lib/api/types/data-contracts";

import Currency from "~/components/global/Currency.vue";
import DateTime from "~/components/global/DateTime.vue";
import { cn } from "~/lib/utils";

/**
 * Create columns with i18n support.
 * Pass `t` from useI18n() when creating the columns in your component.
 */
export function makeColumns({
  t,
  refresh,
  disableSort,
}: {
  t: (key: string) => string;
  refresh?: () => void;
  disableSort?: boolean;
}): ColumnDef<EntitySummary>[] {
  const sortable = (column: Column<EntitySummary, unknown>, key: string) => {
    const sortState = column.getIsSorted(); // 'asc' | 'desc' | false
    if (!sortState) {
      // show the neutral up/down icon when not sorted
      return [t(key), h(ArrowUpDown, { class: cn(["ml-2 h-4 w-4 opacity-40", disableSort && "opacity-0"]) })];
    }
    // show a single arrow that points up for asc (rotate-180) and down for desc
    return [
      t(key),
      h(ArrowDown, {
        class: cn([
          "ml-2 h-4 w-4 transition-transform opacity-100",
          sortState === "asc" ? "rotate-180" : "",
          disableSort && "opacity-0",
        ]),
      }),
    ];
  };

  return [
    {
      id: "select",
      header: ({ table }) =>
        h(Checkbox, {
          class: "size-5",
          modelValue: table.getIsAllPageRowsSelected()
            ? true
            : table.getSelectedRowModel().rows.length > 0
              ? "indeterminate"
              : false,
          "onUpdate:modelValue": (value: boolean | "indeterminate") => table.toggleAllPageRowsSelected(!!value),
          "aria-label": t("components.item.view.selectable.select_all"),
        }),
      cell: ({ row }) =>
        h(Checkbox, {
          class: "size-5",
          modelValue: row.getIsSelected(),
          "onUpdate:modelValue": (value: boolean | "indeterminate") => row.toggleSelected(!!value),
          "aria-label": `${t("components.item.view.selectable.select_row")}: ${row.original.name}`,
        }),
      enableHiding: false,
    },
    {
      id: "assetId",
      accessorKey: "assetId",
      header: ({ column }) =>
        h(
          Button,
          {
            variant: "ghost",
            class: "h-auto min-h-10 whitespace-normal px-2 py-2 text-left",
            onClick: () => !disableSort && column.toggleSorting(column.getIsSorted() === "asc"),
          },
          () => sortable(column, "items.asset_id")
        ),
      cell: ({ row }) => h("div", { class: "text-sm" }, String(row.getValue("assetId") ?? "")),
    },
    {
      id: "name",
      accessorKey: "name",
      header: ({ column }) =>
        h(
          Button,
          {
            variant: "ghost",
            class: "h-auto min-h-10 whitespace-normal px-2 py-2 text-left",
            onClick: () => !disableSort && column.toggleSorting(column.getIsSorted() === "asc"),
          },
          () => sortable(column, "items.name")
        ),
      cell: ({ row }) =>
        h(
          "span",
          { class: "block min-w-40 max-w-sm whitespace-normal break-words text-sm font-medium" },
          row.getValue("name")
        ),
    },
    {
      id: "quantity",
      accessorKey: "quantity",
      header: ({ column }) =>
        h(
          Button,
          {
            variant: "ghost",
            class: "h-auto min-h-10 whitespace-normal px-2 py-2 text-left",
            onClick: () => !disableSort && column.toggleSorting(column.getIsSorted() === "asc"),
          },
          () => sortable(column, "items.quantity")
        ),
      cell: ({ row }) => h("div", { class: "text-center" }, String(row.getValue("quantity") ?? "")),
    },
    {
      id: "insured",
      accessorKey: "insured",
      header: ({ column }) =>
        h(
          Button,
          {
            variant: "ghost",
            class: "h-auto min-h-10 whitespace-normal px-2 py-2 text-left",
            onClick: () => !disableSort && column.toggleSorting(column.getIsSorted() === "asc"),
          },
          () => sortable(column, "items.insured")
        ),
      cell: ({ row }) => {
        const val = row.getValue("insured");
        return h(
          "div",
          { class: "block mx-auto w-min", role: "img", "aria-label": t(val ? "global.yes" : "global.no") },
          val
            ? h(Check, { class: "h-4 w-4 text-primary", "aria-hidden": "true" })
            : h(X, { class: "h-4 w-4 text-foreground", "aria-hidden": "true" })
        );
      },
    },
    {
      id: "purchasePrice",
      accessorKey: "purchasePrice",
      header: ({ column }) =>
        h(
          Button,
          {
            variant: "ghost",
            class: "h-auto min-h-10 whitespace-normal px-2 py-2 text-left",
            onClick: () => !disableSort && column.toggleSorting(column.getIsSorted() === "asc"),
          },
          () => sortable(column, "items.purchase_price")
        ),
      cell: ({ row }) =>
        h("div", { class: "text-center" }, h(Currency, { amount: Number(row.getValue("purchasePrice")) })),
    },
    {
      id: "location",
      accessorKey: "location",
      header: ({ column }) =>
        h(
          Button,
          {
            variant: "ghost",
            class: "h-auto min-h-10 whitespace-normal px-2 py-2 text-left",
            onClick: () => !disableSort && column.toggleSorting(column.getIsSorted() === "asc"),
          },
          () => sortable(column, "items.location")
        ),
      cell: ({ row }) => {
        const item = row.original as EntitySummary;
        const loc = (item.location || item.parent) as { id: string; name: string } | null;
        if (loc) {
          return h(
            "a",
            {
              href: `/location/${loc.id}`,
              class: "hb-focus block min-w-32 max-w-xs whitespace-normal break-words hover:underline text-sm",
            },
            loc.name
          );
        }
        return h("div", { class: "text-sm text-muted-foreground" }, "");
      },
    },
    {
      id: "archived",
      accessorKey: "archived",
      header: ({ column }) =>
        h(
          Button,
          {
            variant: "ghost",
            class: "h-auto min-h-10 whitespace-normal px-2 py-2 text-left",
            onClick: () => !disableSort && column.toggleSorting(column.getIsSorted() === "asc"),
          },
          () => sortable(column, "items.archived")
        ),
      cell: ({ row }) => {
        const val = row.getValue("archived");
        return h(
          "div",
          { class: "block mx-auto w-min", role: "img", "aria-label": t(val ? "global.yes" : "global.no") },
          val
            ? h(Check, { class: "h-4 w-4 text-primary", "aria-hidden": "true" })
            : h(X, { class: "h-4 w-4 text-foreground", "aria-hidden": "true" })
        );
      },
    },
    {
      id: "createdAt",
      accessorKey: "createdAt",
      header: ({ column }) =>
        h(
          Button,
          {
            variant: "ghost",
            class: "h-auto min-h-10 whitespace-normal px-2 py-2 text-left",
            onClick: () => !disableSort && column.toggleSorting(column.getIsSorted() === "asc"),
          },
          () => sortable(column, "items.created_at")
        ),
      cell: ({ row }) =>
        h(
          "div",
          { class: "text-center text-sm" },
          h(DateTime, { date: row.getValue("createdAt") as Date, datetimeType: "date" })
        ),
    },
    {
      id: "updatedAt",
      accessorKey: "updatedAt",
      header: ({ column }) =>
        h(
          Button,
          {
            variant: "ghost",
            class: "h-auto min-h-10 whitespace-normal px-2 py-2 text-left",
            onClick: () => !disableSort && column.toggleSorting(column.getIsSorted() === "asc"),
          },
          () => sortable(column, "items.updated_at")
        ),
      cell: ({ row }) =>
        h(
          "div",
          { class: "text-center text-sm" },
          h(DateTime, { date: row.getValue("updatedAt") as Date, datetimeType: "date" })
        ),
    },
    {
      id: "actions",
      enableHiding: false,
      header: ({ table }) => {
        const selectedCount = table.getSelectedRowModel().rows.length;
        return h(
          "div",
          {
            class: [
              "relative inline-flex items-center",
              selectedCount === 0 ? "opacity-50 pointer-events-none" : "",
            ].join(" "),
          },
          [
            h(DropdownAction, {
              disabled: selectedCount === 0,
              multi: {
                items: table.getSelectedRowModel().rows,
                columns: table.getAllColumns(),
              },
              onExpand: () => {
                table.getSelectedRowModel().rows.forEach(row => row.toggleExpanded());
              },
              view: "table",
              onRefresh: () => refresh?.(),
              table,
            }),
            selectedCount > 0 &&
              h(
                "span",
                {
                  class: "-right-2 -top-2 absolute flex min-w-5",
                },
                h(
                  "span",
                  {
                    class:
                      "relative flex min-w-5 h-5 items-center justify-center rounded-full bg-primary px-1 text-primary-foreground text-xs pointer-events-none whitespace-nowrap",
                  },
                  String(selectedCount)
                )
              ),
          ]
        );
      },
      cell: ({ row, table }) => {
        const item = row.original;
        return h(
          "div",
          { class: "relative" },
          h(DropdownAction, {
            item,
            onExpand: row.toggleExpanded,
            view: "table",
            onRefresh: () => refresh?.(),
            table,
          })
        );
      },
    },
  ];
}
