<template>
  <div class="pb-3">
    <CardTitle class="flex flex-wrap items-center gap-2 text-lg font-medium leading-snug tracking-tight">
      <slot />
    </CardTitle>
    <slot name="subtitle" />
    <CardDescription v-if="$slots.description">
      <slot name="description" />
    </CardDescription>
    <div v-if="$slots.after">
      <slot name="after" />
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { CardDescription, CardTitle } from "@/components/ui/card";
</script>
