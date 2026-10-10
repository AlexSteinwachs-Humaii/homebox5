<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
  import { Button, buttonVariants } from "@/components/ui/button";
  import { useDialog } from "@/components/ui/dialog-provider";
  import { DialogID } from "~/components/ui/dialog-provider/utils";
  import { useConfirm } from "@/composables/use-confirm";
  import { toast } from "@/components/ui/sonner";
  import MdiClose from "~icons/mdi/close";
  import MdiDownload from "~icons/mdi/download";
  import MdiDelete from "~icons/mdi/delete";

  const { t } = useI18n();
  const confirm = useConfirm();

  const { closeDialog, registerOpenDialogCallback } = useDialog();

  const api = useUserApi();

  const image = reactive<{
    attachmentId: string;
    itemId: string;
    originalSrc: string;
    originalType?: string;
    thumbnailSrc?: string;
  }>({
    attachmentId: "",
    itemId: "",
    originalSrc: "",
  });

  const failed = ref(false);
  const opener = ref<HTMLElement | null>(null);

  function restoreFocus(event: Event) {
    event.preventDefault();
    if (opener.value?.isConnected) opener.value.focus();
  }

  function resetImage() {
    image.originalSrc = "";
    image.originalType = undefined;
    image.thumbnailSrc = "";
    failed.value = false;
  }

  function closeImage() {
    closeDialog(DialogID.ItemImage);
    resetImage();
  }

  onMounted(() => {
    const cleanup = registerOpenDialogCallback(DialogID.ItemImage, params => {
      opener.value = document.activeElement as HTMLElement;
      failed.value = false;
      image.attachmentId = params.attachmentId;
      image.itemId = params.itemId;
      if (params.type === "preloaded") {
        image.originalSrc = params.originalSrc;
        image.originalType = params.originalType;
        image.thumbnailSrc = params.thumbnailSrc;
      } else if (params.type === "attachment") {
        image.originalSrc = api.authURL(`/entities/${params.itemId}/attachments/${params.attachmentId}`);
        image.originalType = params.mimeType;
        image.thumbnailSrc = params.thumbnailId
          ? api.authURL(`/entities/${params.itemId}/attachments/${params.thumbnailId}`)
          : image.originalSrc;
      }
    });

    onUnmounted(cleanup);
  });

  async function deleteAttachment() {
    const confirmed = await confirm.open(t("items.delete_attachment_confirm"));

    if (confirmed.isCanceled) {
      return;
    }

    const { error } = await api.items.attachments.delete(image.itemId, image.attachmentId);

    if (error) {
      toast.error(t("items.toast.failed_delete_attachment"));
      return;
    }

    closeDialog(DialogID.ItemImage, {
      action: "delete",
      id: image.attachmentId,
    });
    resetImage();
    toast.success(t("items.toast.attachment_deleted"));
  }
</script>

<template>
  <Dialog :dialog-id="DialogID.ItemImage">
    <DialogContent
      class="max-h-[90svh] w-[calc(100vw_-_2rem)] max-w-3xl gap-3 p-4"
      disable-close
      :aria-describedby="undefined"
      @close-auto-focus="restoreFocus"
    >
      <div class="flex flex-wrap items-center justify-between gap-2">
        <DialogTitle>{{ $t("items.photo") }}</DialogTitle>
        <div class="flex gap-2">
          <Button variant="destructive" size="icon" :aria-label="$t('global.delete')" @click="deleteAttachment">
            <MdiDelete />
          </Button>
          <a
            :class="buttonVariants({ size: 'icon' })"
            :href="image.originalSrc"
            download
            :aria-label="$t('components.item.attachments_list.download')"
          >
            <MdiDownload />
          </a>
          <Button size="icon" :aria-label="$t('global.close')" @click="closeImage">
            <MdiClose />
          </Button>
        </div>
      </div>
      <p v-if="failed" role="status" class="text-muted-foreground">{{ $t("items.photo_unavailable") }}</p>
      <a
        :href="image.originalSrc"
        target="_blank"
        rel="noopener noreferrer"
        class="min-w-0 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        :aria-label="$t('components.item.attachments_list.open_new_tab')"
      >
        <picture v-show="!failed">
          <source :srcset="image.originalSrc" :type="image.originalType" />
          <img
            :src="image.thumbnailSrc || image.originalSrc"
            :alt="$t('items.photo')"
            class="mx-auto max-h-[65svh] max-w-full object-contain"
            @error="failed = true"
          />
        </picture>
        <span class="mt-2 block text-center text-sm underline">{{ $t("items.view_original_photo") }}</span>
      </a>
    </DialogContent>
  </Dialog>
</template>
