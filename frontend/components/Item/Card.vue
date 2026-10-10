<template>
  <Card class="relative min-w-0 overflow-hidden" :class="{ 'ring-2 ring-primary': tableRow?.getIsSelected() }">
    <div v-if="tableRow" class="absolute left-1 top-1 z-10">
      <Checkbox
        class="size-5 bg-accent hover:bg-background-accent"
        :model-value="tableRow.getIsSelected()"
        :aria-label="`${$t('components.item.view.selectable.select_card')}: ${item.name}`"
        @update:model-value="tableRow.toggleSelected()"
      />
    </div>
    <div>
      <div class="relative bg-muted" :class="compact ? 'h-24' : 'h-[200px]'">
        <NuxtLink
          :to="`/item/${item.id}`"
          :aria-label="item.name"
          class="hb-focus absolute inset-0 block focus-visible:ring-inset focus-visible:after:absolute focus-visible:after:inset-0 focus-visible:after:rounded-sm focus-visible:after:border-4 focus-visible:after:border-background focus-visible:after:ring-2 focus-visible:after:ring-inset focus-visible:after:ring-ring"
        >
          <img
            v-if="imageUrl"
            class="absolute size-full"
            :class="objectContain ? 'object-contain' : 'object-cover'"
            loading="lazy"
            :src="imageUrl"
            :alt="item.name"
          />
          <div v-else class="flex h-full items-center justify-center text-muted-foreground" aria-hidden="true">
            <MdiImageOffOutline class="size-8" />
          </div>
        </NuxtLink>
        <div v-if="item.parent && !compact" class="absolute inset-x-1 bottom-1">
          <Badge class="text-wrap bg-secondary text-secondary-foreground hover:bg-secondary/70 hover:underline">
            <NuxtLink v-if="item.parent" :to="`/location/${item.parent.id}`" class="hb-focus">
              {{ locationString }}
            </NuxtLink>
          </Badge>
        </div>
      </div>
      <div class="col-span-4 flex grow flex-col gap-y-1 p-4 pt-2">
        <h2 class="text-wrap break-words text-lg font-bold">
          <NuxtLink :to="`/item/${item.id}`" class="hb-focus">{{ item.name }}</NuxtLink>
        </h2>
        <NuxtLink
          v-if="compact && item.parent"
          :to="`/location/${item.parent.id}`"
          class="hb-focus text-sm text-muted-foreground"
        >
          {{ locationString }}
        </NuxtLink>
        <Separator class="mb-1" />
        <TooltipProvider :delay-duration="0">
          <div class="flex items-center gap-2">
            <Tooltip v-if="item.insured">
              <TooltipTrigger as-child>
                <MdiShieldCheck class="size-5 text-primary" />
              </TooltipTrigger>
              <TooltipContent>
                {{ $t("global.insured") }}
              </TooltipContent>
            </Tooltip>
            <Tooltip v-if="item.archived">
              <TooltipTrigger as-child>
                <MdiArchive class="size-5 text-destructive" />
              </TooltipTrigger>
              <TooltipContent>
                {{ $t("global.archived") }}
              </TooltipContent>
            </Tooltip>
            <div class="grow" />
            <Tooltip>
              <TooltipTrigger as-child>
                <Badge>
                  {{ item.quantity }}
                </Badge>
              </TooltipTrigger>
              <TooltipContent>
                {{ $t("global.quantity") }}
              </TooltipContent>
            </Tooltip>
          </div>
        </TooltipProvider>
        <dl v-if="compact" class="grid grid-cols-2 gap-2 text-sm">
          <div class="min-w-0">
            <dt class="break-words text-muted-foreground">
              {{ $t("items.insured") }}
            </dt>
            <dd>{{ $t(item.insured ? "global.yes" : "global.no") }}</dd>
          </div>
          <div class="min-w-0">
            <dt class="break-words text-muted-foreground">
              {{ $t("items.purchase_price") }}
            </dt>
            <dd class="break-words">
              <Currency :amount="item.purchasePrice" />
            </dd>
          </div>
        </dl>
        <Markdown v-if="!compact" class="mb-2 line-clamp-3 text-ellipsis" :source="item.description" />
        <div class="-mr-1 mt-auto flex flex-wrap justify-end gap-2">
          <TagChip v-for="tag in itemTags" :key="tag.id" :tag="tag" size="sm" :ancestors="tag.ancestors" />
        </div>
      </div>
    </div>
  </Card>
</template>

<script setup lang="ts">
  import type { EntityOut, EntitySummary } from "~~/lib/api/types/data-contracts";
  import MdiShieldCheck from "~icons/mdi/shield-check";
  import Currency from "@/components/global/Currency.vue";
  import MdiImageOffOutline from "~icons/mdi/image-off-outline";
  import MdiArchive from "~icons/mdi/archive";
  import { Badge } from "@/components/ui/badge";
  import { Card } from "@/components/ui/card";
  import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
  import { Separator } from "@/components/ui/separator";
  import Markdown from "@/components/global/Markdown.vue";
  import TagChip from "@/components/Tag/Chip.vue";
  import type { Row } from "@tanstack/vue-table";
  import { Checkbox } from "@/components/ui/checkbox";

  const api = useUserApi();
  const preferences = useViewPreferences();

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

  const itemTags = computed(() => {
    return useTagStore().withAncestors(props.item.tags);
  });

  const props = defineProps({
    item: {
      type: Object as () => EntityOut | EntitySummary,
      required: true,
    },
    compact: {
      type: Boolean,
      default: false,
    },
    locationFlatTree: {
      type: Array as () => FlatTreeItem[],
      required: false,
      default: () => [],
    },
    tableRow: {
      type: Object as () => Row<EntitySummary>,
      required: false,
      default: () => null,
    },
  });

  const objectContain = computed(() => !preferences.value.legacyImageFit);

  const locationString = computed(
    () => props.locationFlatTree.find(l => l.id === props.item.parent?.id)?.treeString || props.item.parent?.name
  );
</script>

<style lang="css"></style>
