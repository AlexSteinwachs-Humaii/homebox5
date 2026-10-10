<script setup lang="ts">
  import ItemCard from "@/components/Item/Card.vue";
  import type { EntitySummary } from "~/lib/api/types/data-contracts";
  import type { Table as TableType } from "@tanstack/vue-table";
  import MdiSelectSearch from "~icons/mdi/select-search";
  import { Checkbox } from "@/components/ui/checkbox";
  import DropdownAction from "./data-table-dropdown.vue";

  const preferences = useViewPreferences();

  const props = defineProps<{
    table: TableType<EntitySummary>;
    locationFlatTree?: FlatTreeItem[];
  }>();

  defineEmits<{
    (e: "refresh"): void;
  }>();

  const selectedCount = computed(() => props.table.getSelectedRowModel().rows.length);
</script>

<template>
  <Teleport to="#selectable-subtitle" defer>
    <Checkbox
      class="size-6 shrink-0 p-0"
      :model-value="
        table.getIsAllPageRowsSelected() ? true : table.getSelectedRowModel().rows.length > 0 ? 'indeterminate' : false
      "
      :aria-label="$t('components.item.view.selectable.select_all')"
      @update:model-value="table.toggleAllPageRowsSelected(!!$event)"
    />

    <div class="grow" />

    <div :class="['relative inline-flex items-center', selectedCount === 0 ? 'pointer-events-none opacity-50' : '']">
      <DropdownAction
        :disabled="selectedCount === 0"
        :multi="{ items: table.getSelectedRowModel().rows, columns: table.getAllColumns() }"
        view="card"
        :table="table"
        @refresh="$emit('refresh')"
      />

      <span v-if="selectedCount > 0" class="absolute -right-1 -top-1 flex min-w-5">
        <span
          class="pointer-events-none relative flex h-5 min-w-5 items-center justify-center whitespace-nowrap rounded-full bg-primary px-1 text-xs text-primary-foreground"
        >
          {{ String(selectedCount) }}
        </span>
      </span>
    </div>
  </Teleport>
  <div
    v-if="table.getRowModel().rows?.length === 0"
    class="hb-panel flex flex-col items-center gap-3 p-8 text-center text-muted-foreground"
    role="status"
  >
    <MdiSelectSearch class="size-10" />
    <p>{{ $t("items.no_results") }}</p>
  </div>
  <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
    <ItemCard
      v-for="item in table.getRowModel().rows"
      :key="item.original.id"
      :item="item.original"
      compact
      :table-row="preferences.quickActions.enabled ? item : undefined"
      :location-flat-tree="locationFlatTree"
    />
  </div>
</template>
