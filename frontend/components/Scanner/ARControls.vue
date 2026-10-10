<template>
  <div class="pointer-events-none absolute inset-0 z-20 flex flex-col justify-between">
    <!-- Top bar -->
    <div
      class="pointer-events-auto flex flex-wrap items-center gap-3 border-b border-border bg-card p-3 text-card-foreground"
    >
      <Button
        variant="ghost"
        size="icon"
        :aria-label="t('scanner_ar.back')"
        :title="t('scanner_ar.back')"
        @click="$emit('back')"
      >
        <MdiArrowLeft class="size-5" />
      </Button>
      <span class="min-w-0 break-words font-editorial text-lg">{{ t("scanner_ar.title") }}</span>
      <div class="grow" />
      <div v-if="isScanning" class="flex items-center gap-1.5" role="status">
        <span class="size-2 rounded-full bg-primary" aria-hidden="true" />
        <span class="text-xs text-muted-foreground">{{ t("scanner_ar.scanning") }}</span>
      </div>
    </div>

    <!-- Bottom bar -->
    <div
      class="pointer-events-auto flex flex-wrap items-center justify-center gap-4 border-t border-border bg-card p-3 text-card-foreground"
    >
      <Button variant="outline" class="h-auto min-h-11 whitespace-normal" @click="$emit('switchCamera')">
        <MdiCameraFlip class="mr-1.5 size-4" />
        {{ t("scanner_ar.switch_camera") }}
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { Button } from "@/components/ui/button";
  import MdiArrowLeft from "~icons/mdi/arrow-left";
  import MdiCameraFlip from "~icons/mdi/camera-flip";

  defineProps<{
    isScanning: boolean;
  }>();

  defineEmits<{
    back: [];
    switchCamera: [];
  }>();

  const { t } = useI18n();
</script>
