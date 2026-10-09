<script setup lang="ts">
  import { themes } from "~~/lib/data/themes";
  import { useTheme } from "~/composables/use-theme";

  const { theme: selectedTheme, setTheme } = useTheme();
</script>

<template>
  <div class="grid grid-cols-1 gap-4 font-sans sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
    <button
      v-for="theme in themes"
      :key="theme.value"
      type="button"
      class="min-w-0 rounded-lg border p-1 text-left ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
      :class="{ 'ring-2 ring-ring ring-offset-2': selectedTheme === theme.value }"
      :aria-label="theme.label"
      :aria-pressed="selectedTheme === theme.value"
      :data-set-theme="theme.value"
      @click="setTheme(theme.value)"
    >
      <div
        :class="'theme-' + theme.value"
        :data-theme="theme.value"
        class="w-full overflow-hidden rounded-md bg-background-accent text-foreground"
      >
        <div class="grid grid-cols-5 grid-rows-3">
          <div class="col-start-1 row-start-1 bg-background" />
          <div class="col-start-1 row-start-2 bg-sidebar" />
          <div class="col-start-1 row-start-3 bg-background-accent" />
          <div class="col-span-4 col-start-2 row-span-3 row-start-1 flex flex-col gap-1 bg-background p-2">
            <div class="font-bold">{{ theme.label }}</div>
            <div aria-hidden="true" class="flex flex-wrap gap-1">
              <div class="flex size-5 items-center justify-center rounded bg-primary lg:size-6">
                <div class="text-sm font-bold text-primary-foreground">A</div>
              </div>
              <div class="flex size-5 items-center justify-center rounded bg-secondary lg:size-6">
                <div class="text-sm font-bold text-secondary-foreground">A</div>
              </div>
              <div class="flex size-5 items-center justify-center rounded bg-accent lg:size-6">
                <div class="text-sm font-bold text-accent-foreground">A</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </button>
  </div>
</template>
