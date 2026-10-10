<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { Button } from "~/components/ui/button";
  import type { CsvArtifact } from "~/lib/api/types/csv-contracts.generated";
  import type { CsvExportRequest } from "~/lib/exports/source-registry";

  const props = defineProps<{
    request: CsvExportRequest;
    disabled?: boolean;
    scopeLabel?: string;
  }>();
  const api = useUserApi();
  const { t } = useI18n();
  const open = ref(false);
  const busy = ref(false);
  const artifact = ref<CsvArtifact>();
  const captured = ref<CsvExportRequest>();
  const message = ref("");
  const panelId = useId();
  let timer: ReturnType<typeof setTimeout> | undefined;
  let disposed = false;
  let trigger: HTMLElement | undefined;

  function toggle(event: MouseEvent) {
    trigger = event.currentTarget as HTMLElement;
    if (open.value) return close();
    // Later filter/column edits cannot rewrite the initiated export.
    captured.value = JSON.parse(JSON.stringify(props.request)) as CsvExportRequest;
    artifact.value = undefined;
    message.value = "";
    open.value = true;
  }

  function close() {
    open.value = false;
    clearTimeout(timer);
    // Closing stops observation, not the retained server job.
    trigger?.focus();
  }

  async function observe() {
    if (!artifact.value || !captured.value || disposed || !open.value) return;
    try {
      const result = await api.csvExports.get(captured.value.scope, artifact.value.id);
      if (disposed || !open.value) return;
      if (result.error) throw new Error("status");
      artifact.value = result.data;
      if (["pending", "running"].includes(result.data.status)) {
        timer = setTimeout(observe, 1000);
      } else {
        busy.value = false;
        message.value = t(result.data.status === "completed" ? "csv_export.ready" : "csv_export.generation_failed");
      }
    } catch {
      busy.value = false;
      message.value = t("csv_export.status_failed");
    }
  }

  async function generate() {
    if (!captured.value || busy.value) return;
    busy.value = true;
    message.value = t("csv_export.processing");
    try {
      const result = await api.csvExports.create(captured.value);
      if (disposed || !open.value) return;
      if (result.error) throw new Error("create");
      artifact.value = result.data;
      await observe();
    } catch {
      busy.value = false;
      message.value = t("csv_export.generation_failed");
    }
  }

  async function download() {
    if (!artifact.value || !captured.value || busy.value) return;
    busy.value = true;
    try {
      const result = await api.csvExports.download(captured.value.scope, artifact.value.id);
      if (result.error) {
        message.value = t("csv_export.unavailable");
        return;
      }
      const blob = await result.response.blob();
      if (disposed) return;
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `${artifact.value.source}.csv`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 60000);
      message.value = t("csv_export.download_started");
    } catch {
      // Retain artifact ID: local failure must never regenerate the CSV.
      message.value = t("csv_export.download_failed");
    } finally {
      busy.value = false;
    }
  }

  onBeforeUnmount(() => {
    disposed = true;
    clearTimeout(timer);
  });
</script>

<template>
  <div class="min-w-0 print:hidden" data-csv-export>
    <Button
      type="button"
      variant="outline"
      size="sm"
      :disabled="disabled || busy || request.columns.length === 0"
      :aria-expanded="open"
      :aria-controls="panelId"
      @click.stop="toggle"
      >{{ $t("csv_export.action") }}</Button
    >
    <div v-if="open" :id="panelId" class="hb-panel mt-2 max-w-xl space-y-3 p-4 text-sm" @click.stop>
      <p class="break-words">
        {{ scopeLabel || $t("csv_export.all_matching") }}
      </p>
      <p>{{ $t("csv_export.retained") }}</p>
      <p v-if="artifact?.expiresAt">
        {{ $t("csv_export.expires", { date: artifact.expiresAt }) }}
      </p>
      <p role="status" aria-live="polite" class="break-words">{{ message }}</p>
      <div class="flex flex-wrap gap-2">
        <Button v-if="!artifact" type="button" size="sm" :disabled="busy" @click="generate">
          {{ $t("csv_export.generate") }}
        </Button>
        <Button v-else-if="artifact.status === 'completed'" type="button" size="sm" :disabled="busy" @click="download">
          {{ $t("csv_export.download") }}
        </Button>
        <Button
          v-else-if="!busy && ['pending', 'running'].includes(artifact.status)"
          type="button"
          size="sm"
          @click="observe"
          >{{ $t("global.retry") }}</Button
        >
        <Button type="button" variant="outline" size="sm" :disabled="busy" @click="close">
          {{ $t("global.close") }}
        </Button>
      </div>
    </div>
  </div>
</template>
