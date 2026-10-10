<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { Button } from "~/components/ui/button";
  import RecordListExport from "./RecordListExport.vue";
  import type { CsvArtifact } from "~/lib/api/types/csv-contracts.generated";
  import { administrationLabelKeys, csvHistoryExport } from "~/lib/exports/administration";

  // Export-only integration. S8 owns retained-history presentation/management.
  const props = defineProps<{ scope: "collection" | "private" }>();
  const { t } = useI18n();
  const api = useUserApi();
  const rows = ref<CsvArtifact[]>([]);
  const loading = ref(false);
  const failed = ref(false);
  const ready = ref(false);
  const exportLabel = (id: string) => t(administrationLabelKeys[id] ?? `global.${id}`);
  const request = computed(() =>
    props.scope === "private"
      ? csvHistoryExport("private-csv-history", rows.value, exportLabel)
      : csvHistoryExport("collection-csv-history", rows.value, exportLabel)
  );

  async function captureHistory() {
    if (loading.value) return;
    loading.value = true;
    failed.value = false;
    ready.value = false;
    rows.value = [];
    try {
      const result = await api.csvExports.list(props.scope);
      if (result.error) throw new Error("history");
      rows.value = result.data.items;
      ready.value = true;
    } catch {
      failed.value = true;
    } finally {
      loading.value = false;
    }
  }
</script>

<template>
  <section class="min-w-0 space-y-2 print:hidden" data-csv-history-export>
    <Button
      type="button"
      variant="outline"
      size="sm"
      class="h-auto min-h-9 max-w-full whitespace-normal break-words text-start"
      :disabled="loading"
      @click="captureHistory"
    >
      {{ $t(scope === "private" ? "csv_export.private_history_export" : "csv_export.history_export") }}
    </Button>
    <p v-if="loading" role="status">{{ $t("global.loading") }}</p>
    <p v-if="failed" role="alert">{{ $t("csv_export.history_load_failed") }}</p>
    <RecordListExport v-if="ready" :request="request" :scope-label="$t('csv_export.history_snapshot')" />
  </section>
</template>
