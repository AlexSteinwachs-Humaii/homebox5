<script setup lang="ts">
  import { useTreeState } from "./tree-state";
  import type { TreeItem } from "~~/lib/api/types/data-contracts";
  import MdiChevronRight from "~icons/mdi/chevron-right";
  import MdiMapMarker from "~icons/mdi/map-marker";
  import MdiPackageVariant from "~icons/mdi/package-variant";
  import LocationTreeNode from "./Node.vue";

  type Props = {
    treeId: string;
    depth?: number;
    item: TreeItem;
    showItems?: boolean;
  };
  const props = withDefaults(defineProps<Props>(), {
    showItems: true,
    depth: 0,
  });

  const link = computed(() => {
    return props.item.type === "location" ? `/location/${props.item.id}` : `/item/${props.item.id}`;
  });

  const state = useTreeState(props.treeId);

  const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

  const filteredChildren = computed(() => {
    const children = props.item.children ?? [];

    if (props.showItems) {
      return children;
    }

    return children.filter(child => child.type === "location");
  });

  const sortedChildren = computed(() => {
    return [...filteredChildren.value].sort((a, b) => collator.compare(a.name, b.name));
  });

  const hasChildren = computed(() => filteredChildren.value.length > 0);

  const openRef = computed({
    get() {
      return state.value[nodeHash.value] ?? false;
    },
    set(value: boolean) {
      state.value[nodeHash.value] = value;
    },
  });

  const nodeHash = computed(() => {
    // converts a UUID to a short hash
    return props.item.id.replace(/-/g, "").substring(0, 8);
  });
</script>

<template>
  <div>
    <div
      class="hb-tree-row"
      :style="{ paddingLeft: `${Math.min(props.depth ?? 0, 4) * 0.75 + 0.5}rem` }"
      @click="hasChildren && (openRef = !openRef)"
    >
      <button
        v-if="hasChildren"
        type="button"
        class="hb-focus flex size-8 shrink-0 items-center justify-center rounded-md hover:bg-accent"
        :aria-expanded="openRef"
        :aria-label="`${$t(openRef ? 'locations.collapse_tree' : 'locations.expand_tree')}: ${item.name}`"
        @click.stop="openRef = !openRef"
      >
        <MdiChevronRight class="size-5" :class="{ 'rotate-90': openRef }" />
      </button>
      <span v-else class="size-8 shrink-0" />

      <MdiMapMarker v-if="item.type === 'location'" class="size-4 shrink-0 text-muted-foreground" />
      <MdiPackageVariant v-else class="size-4 shrink-0 text-muted-foreground" />
      <NuxtLink
        class="hb-focus min-w-0 flex-1 break-words text-sm font-medium leading-relaxed [overflow-wrap:anywhere] hover:underline"
        :to="link"
        @click.stop
        >{{ item.name }}
      </NuxtLink>
    </div>
    <ul v-if="openRef" class="space-y-1">
      <li v-for="child in sortedChildren" :key="child.id">
        <LocationTreeNode :depth="(props.depth ?? 0) + 1" :item="child" :tree-id="treeId" :show-items="showItems" />
      </li>
    </ul>
  </div>
</template>
