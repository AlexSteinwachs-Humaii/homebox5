<script setup lang="ts">
  import MdiChevronRight from "~icons/mdi/chevron-right";
  import { getContrastTextColor } from "~/lib/utils";
  import { getIconComponent } from "~/lib/icons";
  import { useTreeState } from "./tree-state";
  import type { TagTreeItem } from "./types";
  import TagTreeNode from "./Node.vue";

  type Props = {
    treeId: string;
    depth?: number;
    item: TagTreeItem;
  };

  const props = defineProps<Props>();

  const tagIcon = computed(() => getIconComponent(props.item.icon));

  const state = useTreeState(props.treeId);

  const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

  const sortedChildren = computed(() => {
    return [...(props.item.children ?? [])].sort((a, b) => collator.compare(a.name, b.name));
  });

  const hasChildren = computed(() => sortedChildren.value.length > 0);

  const nodeHash = computed(() => {
    return props.item.id.replace(/-/g, "").substring(0, 8);
  });

  const openRef = computed({
    get() {
      return state.value[nodeHash.value] ?? false;
    },
    set(value: boolean) {
      state.value[nodeHash.value] = value;
    },
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

      <div
        class="mr-1 flex size-6 shrink-0 items-center justify-center rounded-full"
        :style="
          item.color
            ? { backgroundColor: item.color, color: getContrastTextColor(item.color) }
            : { backgroundColor: 'hsl(var(--accent))', color: 'hsl(var(--accent-foreground))' }
        "
      >
        <component :is="tagIcon" class="size-3" />
      </div>

      <NuxtLink
        class="hb-focus min-w-0 flex-1 break-words text-sm font-medium leading-relaxed [overflow-wrap:anywhere] hover:underline"
        :to="`/tag/${item.id}`"
        @click.stop
        >{{ item.name }}</NuxtLink
      >
    </div>

    <ul v-if="openRef" class="space-y-1">
      <li v-for="child in sortedChildren" :key="child.id">
        <TagTreeNode :depth="(props.depth ?? 0) + 1" :item="child" :tree-id="treeId" />
      </li>
    </ul>
  </div>
</template>
