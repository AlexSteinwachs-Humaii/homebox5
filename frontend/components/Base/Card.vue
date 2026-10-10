<template>
  <Card class="hb-panel overflow-hidden shadow-sm">
    <CardHeader v-if="$slots.title" class="px-4 py-5 sm:px-6">
      <component
        :is="collapsable ? 'button' : 'div'"
        :type="collapsable ? 'button' : undefined"
        :aria-expanded="collapsable ? !collapsed : undefined"
        class="hb-focus text-left"
        v-on="collapsable ? { click: toggle } : {}"
      >
        <h3 class="hb-section-title flex items-center gap-2">
          <slot name="title" />
          <template v-if="collapsable">
            <span class="ml-2 transition-transform" :class="{ 'rotate-180': collapsed }">
              <MdiChevronDown class="size-6" />
            </span>
          </template>
        </h3>
      </component>
      <div>
        <p v-if="$slots.subtitle" class="mt-1 max-w-2xl text-sm text-muted-foreground">
          <slot name="subtitle" />
        </p>
        <template v-if="$slots['title-actions']">
          <slot name="title-actions" />
        </template>
      </div>
    </CardHeader>
    <CardContent
      :class="{
        'max-h-[9000px]': collapsable && !collapsed,
        'max-h-0 overflow-hidden': collapsed,
      }"
      class="p-0 transition-[max-height] duration-200"
    >
      <slot />
    </CardContent>
  </Card>
</template>

<script setup lang="ts">
  import MdiChevronDown from "~icons/mdi/chevron-down";
  import { Card, CardContent, CardHeader } from "@/components/ui/card";

  defineProps<{
    collapsable?: boolean;
  }>();

  function toggle() {
    collapsed.value = !collapsed.value;
  }

  const collapsed = ref(false);
</script>
