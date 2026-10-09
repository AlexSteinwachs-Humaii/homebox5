<script setup lang="ts">
  import { cn } from "@/lib/utils";
  import { X } from "lucide-vue-next";
  import {
    DialogClose,
    DialogContent,
    type DialogContentEmits,
    type DialogContentProps,
    DialogOverlay,
    DialogPortal,
    useForwardPropsEmits,
  } from "reka-ui";
  import { computed, type HTMLAttributes } from "vue";

  const props = defineProps<DialogContentProps & { class?: HTMLAttributes["class"] }>();
  const emits = defineEmits<DialogContentEmits>();

  const delegatedProps = computed(() => {
    const { class: _, ...delegated } = props;

    return delegated;
  });

  const forwarded = useForwardPropsEmits(delegatedProps, emits);
</script>

<template>
  <DialogPortal>
    <DialogOverlay
      class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0"
    >
      <DialogContent
        :class="
          cn(
            'relative z-50 grid shrink-0 w-[calc(100%-2rem)] max-w-lg my-8 gap-4 border border-border bg-popover text-popover-foreground p-6 shadow-lg duration-200 rounded-lg',
            props.class
          )
        "
        v-bind="forwarded"
        @pointer-down-outside="
          (event: CustomEvent<{ originalEvent: PointerEvent }>) => {
            const originalEvent = event.detail.originalEvent;
            const target = originalEvent.target as HTMLElement;
            if (originalEvent.offsetX > target.clientWidth || originalEvent.offsetY > target.clientHeight) {
              event.preventDefault();
            }
          }
        "
      >
        <slot />

        <DialogClose
          class="absolute right-3 top-3 rounded-md p-1 hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <X class="size-4" />
          <span class="sr-only">{{ $t("global.close") }}</span>
        </DialogClose>
      </DialogContent>
    </DialogOverlay>
  </DialogPortal>
</template>
