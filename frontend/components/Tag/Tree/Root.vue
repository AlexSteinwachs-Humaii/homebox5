<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { Button } from "~~/components/ui/button";
  import { useDialog } from "~/components/ui/dialog-provider";
  import { DialogID } from "~/components/ui/dialog-provider/utils";
  import type { TagTreeItem } from "./types";
  import TagTreeNode from "./Node.vue";
  import RecordListExport from "~/components/Export/RecordListExport.vue";
  import { exportColumns, treeFields } from "~/lib/exports/inventory";
  import type { CsvSourceRequest } from "~/lib/exports/source-registry";

  type Props = {
    tags: TagTreeItem[];
    treeId: string;
  };

  const { openDialog } = useDialog();

  const props = defineProps<Props>();

  const { t } = useI18n();
  const exportRequest = computed<CsvSourceRequest<"tags">>(() => ({
    source: "tags",
    scope: "collection",
    filters: { showItems: false },
    boundary: { kind: "hierarchy" },
    sort: [],
    columns: exportColumns(treeFields, id => t(`csv_export.fields.${id}`)),
  }));

  const collator = new Intl.Collator(undefined, {
    numeric: true,
    sensitivity: "base",
  });

  const sortedTags = computed(() => {
    const list = props.tags ?? [];
    return [...list].sort((a, b) => collator.compare(a.name, b.name));
  });
</script>

<template>
  <div>
    <RecordListExport class="mb-3" :request="exportRequest" :scope-label="$t('csv_export.hierarchy')" />
    <div
      v-if="sortedTags.length === 0"
      class="py-6 text-center text-sm text-muted-foreground"
      role="status"
      aria-live="polite"
    >
      <p class="mx-auto max-w-xs break-words">
        {{ $t("tags.no_results") }}
      </p>
      <Button
        class="mt-3"
        variant="outline"
        size="sm"
        type="button"
        :aria-label="$t('components.tag.create_modal.title') || $t('global.create')"
        @click="openDialog(DialogID.CreateTag)"
      >
        {{ $t("components.tag.create_modal.title") || $t("global.create") }}
      </Button>
    </div>

    <ul :aria-labelledby="treeId" class="space-y-1">
      <li v-for="item in sortedTags" :key="item.id">
        <TagTreeNode :item="item" :tree-id="treeId" />
      </li>
    </ul>
  </div>
</template>
