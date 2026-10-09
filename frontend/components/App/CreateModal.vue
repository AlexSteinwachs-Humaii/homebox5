<template>
  <Dialog v-if="isDesktop" :dialog-id="dialogId">
    <DialogScrollContent @keydown.esc.capture="onEscape">
      <DialogHeader>
        <div class="mr-6 flex min-w-0 flex-wrap items-center justify-between gap-2">
          <DialogTitle>
            <slot name="title">
              {{ title }}
            </slot>
          </DialogTitle>
          <slot name="header-actions" />
        </div>
      </DialogHeader>

      <slot />

      <DialogFooter v-if="!hideFooter">
        <i18n-t
          keypath="components.app.create_modal.createAndAddAnother"
          tag="span"
          class="flex flex-wrap items-center gap-1 text-sm"
        >
          <template #shiftKey>
            <Shortcut size="sm" :keys="[$t('components.app.create_modal.shift')]" />
          </template>
          <template #enterKey>
            <Shortcut size="sm" :keys="[$t('components.app.create_modal.enter')]" />
          </template>
        </i18n-t>
      </DialogFooter>
    </DialogScrollContent>
  </Dialog>

  <Drawer v-else :dialog-id="dialogId">
    <DrawerContent class="max-h-[90%]">
      <DrawerHeader>
        <DrawerTitle>
          <slot name="title">
            {{ title }}
          </slot>
        </DrawerTitle>
      </DrawerHeader>
      <div class="flex justify-center">
        <slot name="header-actions" />
      </div>

      <div class="m-2 min-h-0 overflow-y-auto p-2">
        <slot />
      </div>
    </DrawerContent>
  </Drawer>
</template>

<script setup lang="ts">
  import { useMediaQuery } from "@vueuse/core";
  import type { DialogID } from "@/components/ui/dialog-provider/utils";
  import { Drawer, DrawerContent, DrawerHeader, DrawerTitle } from "@/components/ui/drawer";
  import { Dialog, DialogFooter, DialogHeader, DialogScrollContent, DialogTitle } from "@/components/ui/dialog";
  import { Shortcut } from "@/components/ui/shortcut";
  import { useDialog } from "~/components/ui/dialog-provider";

  const isDesktop = useMediaQuery("(min-width: 768px)");

  const props = defineProps<{
    dialogId: DialogID;
    title?: string;
    hideFooter?: boolean;
  }>();
  const { closeDialog } = useDialog();
  function onEscape(event: KeyboardEvent) {
    const content = event.currentTarget as HTMLElement;
    // Leave Escape to an expanded nested selector; otherwise close the global
    // dialog even if an unmounted menu/selector retains Reka's layer priority.
    if (content.querySelector('[aria-haspopup][aria-expanded="true"]')) return;
    event.preventDefault();
    event.stopPropagation();
    closeDialog(props.dialogId);
  }
</script>
