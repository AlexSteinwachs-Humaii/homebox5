<template>
  <ul role="list" class="divide-y rounded-md border">
    <li
      v-for="attachment in attachments"
      :key="attachment.id"
      class="flex min-w-0 flex-wrap items-center justify-between gap-3 p-3 text-sm"
    >
      <template v-if="isExternalURLAttachment(attachment)">
        <div class="flex w-0 flex-1 items-center">
          <MdiLinkVariant class="size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
          <a
            class="ml-2 min-w-0 flex-1 break-words text-primary underline [overflow-wrap:anywhere]"
            :href="attachment.path"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ attachment.title }}
          </a>
        </div>
        <div class="ml-4 flex shrink-0 gap-2">
          <TooltipProvider :delay-duration="0">
            <Tooltip>
              <TooltipTrigger as-child>
                <a
                  :class="buttonVariants({ size: 'icon' })"
                  :href="attachment.path"
                  :aria-label="$t('components.item.attachments_list.open_new_tab') + ': ' + attachment.title"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <MdiOpenInNew />
                </a>
              </TooltipTrigger>
              <TooltipContent> {{ $t("components.item.attachments_list.open_new_tab") }} </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
      </template>
      <template v-else>
        <div class="flex w-0 flex-1 items-center">
          <MdiPaperclip class="size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
          <span class="ml-2 min-w-0 flex-1 break-words [overflow-wrap:anywhere]"> {{ attachment.title }}</span>
        </div>
        <div class="ml-4 flex shrink-0 gap-2">
          <TooltipProvider :delay-duration="0">
            <Tooltip>
              <TooltipTrigger as-child>
                <a
                  :class="buttonVariants({ size: 'icon' })"
                  :href="attachmentURL(attachment.id)"
                  :download="attachment.title"
                  :aria-label="$t('components.item.attachments_list.download') + ': ' + attachment.title"
                >
                  <MdiDownload />
                </a>
              </TooltipTrigger>
              <TooltipContent> {{ $t("components.item.attachments_list.download") }} </TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <a
                  :class="buttonVariants({ size: 'icon' })"
                  :href="attachmentURL(attachment.id)"
                  target="_blank"
                  rel="noopener noreferrer"
                  :aria-label="$t('components.item.attachments_list.open_new_tab') + ': ' + attachment.title"
                >
                  <MdiOpenInNew />
                </a>
              </TooltipTrigger>
              <TooltipContent> {{ $t("components.item.attachments_list.open_new_tab") }} </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
      </template>
    </li>
  </ul>
</template>

<script setup lang="ts">
  import type { ItemAttachment } from "~~/lib/api/types/data-contracts";
  import MdiPaperclip from "~icons/mdi/paperclip";
  import MdiLinkVariant from "~icons/mdi/link-variant";
  import MdiDownload from "~icons/mdi/download";
  import MdiOpenInNew from "~icons/mdi/open-in-new";
  import { buttonVariants } from "@/components/ui/button";
  import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";

  const props = defineProps({
    attachments: {
      type: Object as () => ItemAttachment[],
      required: true,
    },
    itemId: {
      type: String,
      required: true,
    },
  });

  const api = useUserApi();

  function attachmentURL(attachmentId: string) {
    return api.authURL(`/entities/${props.itemId}/attachments/${attachmentId}`);
  }

  function isExternalURLAttachment(attachment: ItemAttachment) {
    if (attachment.mimeType === "link/url") {
      return true;
    }
    return attachment.path.startsWith("http://") || attachment.path.startsWith("https://");
  }
</script>

<style scoped></style>
