<template>
  <div class="w-full">
    <div class="flex w-full flex-col gap-1.5">
      <Label :for="inputId" class="flex w-full px-1">
        {{ label }}
      </Label>

      <div
        class="relative inline-block rounded-md focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background"
      >
        <Button
          type="button"
          variant="outline"
          class="w-full"
          aria-hidden="true"
          tabindex="-1"
          @click.prevent="openFilePicker"
        >
          {{ buttonLabel }}
        </Button>
        <Input
          :id="inputId"
          ref="fileInput"
          class="absolute left-0 top-0 size-full cursor-pointer opacity-0"
          type="file"
          accept="image/png,image/jpeg,image/gif,image/avif,image/webp,android/force-camera-workaround"
          multiple
          @change="onFilesSelected"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { computed, ref, useId } from "vue";
  import { useI18n } from "vue-i18n";
  import { Label } from "~/components/ui/label";
  import { Input } from "~/components/ui/input";
  import { Button } from "~/components/ui/button";
  import { toast } from "~/components/ui/sonner";
  import { filesToPhotoPreviews, type PhotoPreview } from "./photo-uploader";

  const props = withDefaults(
    defineProps<{
      label?: string;
      buttonLabel?: string;
      existingCount?: number;
    }>(),
    {
      label: undefined,
      buttonLabel: undefined,
      existingCount: 0,
    }
  );

  const emit = defineEmits<{
    (e: "selected", photos: PhotoPreview[]): void;
  }>();

  const { t } = useI18n();
  const inputId = useId();
  const fileInput = ref<HTMLInputElement | null>(null);

  const label = computed(
    () => props.label || t("components.entity.create_modal.entity_photo", { type: t("global.item") })
  );
  const buttonLabel = computed(() => props.buttonLabel || t("components.entity.create_modal.upload_photos"));

  function openFilePicker() {
    fileInput.value?.click();
  }

  async function onFilesSelected(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files || input.files.length === 0) return;

    try {
      const photos = await filesToPhotoPreviews(input.files, props.existingCount);
      emit("selected", photos);
      input.value = "";
    } catch {
      toast.error(t("items.toast.failed_upload_attachment"));
    }
  }
</script>
