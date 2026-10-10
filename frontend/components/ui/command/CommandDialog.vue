<script setup lang="ts">
  import type { DialogRootEmits, DialogRootProps } from "reka-ui";
  import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
  import { useForwardPropsEmits } from "reka-ui";
  import { useDialog } from "@/components/ui/dialog-provider";
  import Command from "./Command.vue";
  import type { DialogID } from "@/components/ui/dialog-provider/utils";

  const props = defineProps<DialogRootProps & { dialogId: DialogID; title: string }>();
  const emits = defineEmits<DialogRootEmits>();

  const forwarded = useForwardPropsEmits(props, emits);
  const { closeDialog } = useDialog();
  function onKeydown(event: KeyboardEvent) {
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      closeDialog(props.dialogId);
    }
  }
</script>

<template>
  <Dialog v-bind="forwarded">
    <!-- https://github.com/unovue/reka-ui/issues/1743 -->
    <DialogContent class="overflow-hidden p-0 shadow-lg" disable-portal @keydown.capture="onKeydown">
      <DialogTitle class="sr-only">{{ title }}</DialogTitle>
      <Command
        class="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:text-muted-foreground [&_[cmdk-group]:not([hidden])_~[cmdk-group]]:pt-0 [&_[cmdk-group]]:px-2 [&_[cmdk-input-wrapper]_svg]:size-5 [&_[cmdk-input]]:h-12 [&_[cmdk-item]]:px-2 [&_[cmdk-item]]:py-3 [&_[cmdk-item]_svg]:size-5"
      >
        <slot />
      </Command>
    </DialogContent>
  </Dialog>
</template>
