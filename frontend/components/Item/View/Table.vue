<script setup lang="ts">
  import { computed } from "vue";
  import type { EntitySummary } from "~/lib/api/types/data-contracts";
  import DataTable from "./table/data-table.vue";
  import { makeColumns } from "./table/columns";
  import { useI18n } from "vue-i18n";

  defineProps<{
    items: EntitySummary[];
  }>();

  const { t } = useI18n();

  const columns = computed(() => makeColumns({ t }).filter(c => c.enableHiding !== false));
</script>

<template>
  <DataTable data-recent-table view="table" :data="items" :columns="columns" disable-controls />
</template>

<style scoped>
  /* Keep the existing table at tablet widths without pushing prices off-screen. */
  [data-recent-table] :deep(table) {
    table-layout: fixed;
  }

  [data-recent-table] :deep(th button) {
    height: auto;
    min-height: 2.5rem;
    white-space: normal;
    overflow-wrap: anywhere;
    padding-inline: 0.5rem;
    gap: 0.25rem;
  }

  [data-recent-table] :deep(th svg) {
    width: 0.75rem;
    flex-shrink: 0;
    margin-left: 0;
  }

  [data-recent-table] :deep(td) {
    overflow-wrap: anywhere;
  }

  [data-recent-table] :deep(td > a) {
    padding: 0.75rem;
  }
</style>
