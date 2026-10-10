<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import type { TreeItem } from "~~/lib/api/types/data-contracts";
  import LocationTreeNode from "./Node.vue";
  import RecordListExport from "~/components/Export/RecordListExport.vue";
  import { exportColumns, treeFields } from "~/lib/exports/inventory";
  import type { CsvSourceRequest } from "~/lib/exports/source-registry";
  import { Button } from "~~/components/ui/button";
  import { useDialog } from "~/components/ui/dialog-provider";
  import { DialogID } from "~/components/ui/dialog-provider/utils";

  type Props = {
    locs: TreeItem[];
    treeId: string;
    showItems?: boolean;
  };

  const { openDialog } = useDialog();

  const props = defineProps<Props>();

  const { t } = useI18n();
  const exportRequest = computed<CsvSourceRequest<"locations">>(() => ({
    source: "locations",
    scope: "collection",
    filters: { showItems: props.showItems ?? true },
    boundary: { kind: "hierarchy" },
    sort: [],
    columns: exportColumns(treeFields, id => t(`csv_export.fields.${id}`)),
  }));

  const collator = new Intl.Collator(undefined, {
    numeric: true,
    sensitivity: "base",
  });

  const sortedLocs = computed(() => {
    const list = props.locs ?? [];
    return [...list].sort((a, b) => collator.compare(a.name, b.name));
  });
</script>

<template>
  <div>
    <RecordListExport class="mb-3" :request="exportRequest" :scope-label="$t('csv_export.hierarchy')" />
    <div
      v-if="sortedLocs.length === 0"
      class="py-6 text-center text-sm text-muted-foreground"
      role="status"
      aria-live="polite"
    >
      <p class="mx-auto max-w-xs break-words">
        {{ $t("components.location.tree.no_locations") }}
      </p>
      <Button
        class="mt-3"
        variant="outline"
        size="sm"
        type="button"
        :aria-label="$t('global.create')"
        @click="
          openDialog(DialogID.CreateEntity, {
            params: { baseType: 'location' },
          })
        "
      >
        {{ $t("global.create") }}
      </Button>
    </div>

    <ul :aria-labelledby="treeId" class="space-y-1">
      <li v-for="item in sortedLocs" :key="item.id">
        <LocationTreeNode :item="item" :tree-id="treeId" :show-items="props.showItems ?? true" />
      </li>
    </ul>
  </div>
</template>
