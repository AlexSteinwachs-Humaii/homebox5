<script lang="ts" setup>
  import { cn } from "~/lib/utils";

  const props = defineProps({
    cmp: {
      type: String,
      default: "div",
    },
    class: {
      type: String,
      default: "",
    },
  });

  const cmpClass = computed(() => cn("container mx-auto my-section-gap max-w-7xl px-page-gutter", props.class));
</script>

<template>
  <component :is="cmp" :class="cmpClass">
    <slot />
  </component>
</template>
