<script setup lang="ts">
  import type { EntityOut } from "~~/lib/api/types/data-contracts";
  import BaseContainer from "@/components/Base/Container.vue";
  import MaintenanceListView from "~/components/Maintenance/ListView.vue";

  const props = defineProps<{
    item: EntityOut;
  }>();
</script>

<template>
  <BaseContainer class="flex min-w-0 flex-col gap-6">
    <MaintenanceListView :current-item-id="props.item.id" />
  </BaseContainer>
</template>
