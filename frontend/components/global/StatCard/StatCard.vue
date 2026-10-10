<template>
  <Card class="flex min-w-0 flex-col gap-2 bg-secondary p-4 text-secondary-foreground">
    <CardHeader class="p-0">
      <CardTitle class="break-words text-sm font-medium text-secondary-foreground">{{ title }}</CardTitle>
    </CardHeader>
    <CardContent class="break-words p-0 text-xl font-medium tabular-nums xl:text-3xl">
      <Currency v-if="type === 'currency'" :amount="value" />
      <template v-if="type === 'number'">{{ value }}</template>
    </CardContent>
    <CardFooter v-if="subtitle">{{ subtitle }}</CardFooter>
  </Card>
</template>

<script setup lang="ts">
  import Currency from "../Currency.vue";
  import type { StatsFormat } from "./types";
  import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";

  type Props = {
    title: string;
    value: number;
    subtitle?: string;
    type?: StatsFormat;
  };

  withDefaults(defineProps<Props>(), {
    type: "number",
    subtitle: undefined,
  });
</script>

<style></style>
