<script setup lang="ts">
  import RecordListExport from "~/components/Export/RecordListExport.vue";
  import { selectorExport } from "~/lib/exports/auxiliary";
  import { computed } from "vue";
  import type { EntitySummary } from "~/lib/api/types/data-contracts";
  import TagChip from "@/components/Tag/Chip.vue";
  import MdiImageOffOutline from "~icons/mdi/image-off-outline";
  import Badge from "~/components/ui/badge/Badge.vue";

  const props = defineProps<{
    item: EntitySummary;
  }>();

  const itemTags = computed(() => {
    return useTagStore().withAncestors(props.item.tags);
  });

  const api = useUserApi();

  const imageUrl = computed(() => {
    if (!props.item.imageId) {
      return null;
    }
    if (props.item.thumbnailId) {
      return api.authURL(`/entities/${props.item.id}/attachments/${props.item.thumbnailId}`);
    } else {
      return api.authURL(`/entities/${props.item.id}/attachments/${props.item.imageId}`);
    }
  });
</script>

<template>
  <div class="flex flex-col items-start gap-4 sm:flex-row">
    <div class="shrink-0">
      <img v-if="imageUrl" :src="imageUrl" :alt="item.name" class="size-20 rounded-lg bg-muted object-contain" />
      <div
        v-else
        class="flex size-20 items-center justify-center rounded-lg bg-muted text-muted-foreground"
        aria-hidden="true"
      >
        <MdiImageOffOutline class="size-6" />
      </div>
    </div>
    <div class="flex min-w-0 flex-1 flex-col gap-2">
      <h2 class="break-words text-lg font-semibold">{{ item.name }}</h2>
      <Badge
        class="max-w-full text-wrap break-words bg-secondary text-secondary-foreground hover:bg-secondary/70 hover:underline"
      >
        <NuxtLink v-if="item.location" :to="`/location/${item.location.id}`" class="hb-focus">
          {{ item.location.name }}
        </NuxtLink>
      </Badge>
      <RecordListExport
        :request="
          selectorExport(
            'tags',
            itemTags.map(r => r.id),
            id => $t(`csv_export.fields.${id}`)
          )
        "
      />
      <div class="flex flex-wrap gap-2">
        <TagChip v-for="tag in itemTags" :key="tag.id" :tag="tag" size="sm" :ancestors="tag.ancestors" />
      </div>
      <p class="whitespace-pre-line break-words text-sm text-muted-foreground">
        {{ item.description || $t("components.item.no_description") }}
      </p>
    </div>
  </div>
</template>
