<script setup lang="ts">
  import type { HTMLAttributes } from "vue";
  import { cn } from "@/lib/utils";

  const props = defineProps<{ class?: HTMLAttributes["class"] }>();
</script>

<template>
  <div
    :class="
      cn(
        'flex flex-col-reverse gap-2 sm:flex-row sm:flex-wrap sm:justify-end [&>button]:h-auto [&>button]:min-h-9 [&>button]:whitespace-normal [&>button]:break-words',
        props.class
      )
    "
  >
    <slot />
  </div>
</template>
