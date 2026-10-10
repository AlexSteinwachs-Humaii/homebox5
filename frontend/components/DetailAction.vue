<template>
  <div class="flex min-w-0 flex-col gap-4 py-6 md:flex-row md:gap-10">
    <div class="min-w-0 flex-1">
      <h4 class="mb-1 text-lg font-semibold">
        <slot name="title" />
      </h4>
      <div class="text-sm text-muted-foreground">
        <slot />
      </div>
    </div>
    <div class="flex min-w-0 items-center md:max-w-sm">
      <slot v-if="$slots.control" name="control" />
      <template v-else-if="to">
        <NuxtLink
          :to="to"
          :class="buttonVariants({ size: 'lg' })"
          class="h-auto min-h-11 w-full min-w-0 grow whitespace-normal md:min-w-52"
        >
          <slot name="button">
            <slot name="title" />
          </slot>
        </NuxtLink>
      </template>
      <template v-else>
        <Button
          class="h-auto min-h-11 w-full min-w-0 grow whitespace-normal md:min-w-52"
          size="lg"
          @click="$emit('action')"
        >
          <slot name="button">
            <slot name="title" />
          </slot>
        </Button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { defineProps } from "vue";
  import { Button, buttonVariants } from "@/components/ui/button";

  defineProps<{
    to?: string;
  }>();

  defineEmits(["action"]);
</script>
