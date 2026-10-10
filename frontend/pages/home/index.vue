<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { statCardData } from "./statistics";
  import { itemsTable } from "./table";
  import { useTagStore } from "~/stores/tags";
  import { useLocationStore } from "~~/stores/locations";
  import BaseContainer from "@/components/Base/Container.vue";
  import BaseCard from "@/components/Base/Card.vue";
  import Subtitle from "~/components/global/Subtitle.vue";
  import StatCard from "~/components/global/StatCard/StatCard.vue";
  import ItemCard from "~/components/Item/Card.vue";
  import LocationCard from "~/components/Location/Card.vue";
  import TagChip from "~/components/Tag/Chip.vue";
  import Table from "~/components/Item/View/Table.vue";
  import DashboardHero from "~/components/App/DashboardHero.vue";
  import RecordListExport from "~/components/Export/RecordListExport.vue";
  import { inventoryExport, captureInventoryView, exportColumns, locationCardFields } from "~/lib/exports/inventory";
  import type { CsvSourceRequest } from "~/lib/exports/source-registry";

  const { t } = useI18n();

  definePageMeta({
    middleware: ["auth"],
  });
  useHead({
    title: "HomeBox | " + t("menu.home"),
  });

  const api = useUserApi();
  const breakpoints = useBreakpoints();

  const locationStore = useLocationStore();
  const locations = computed(() => locationStore.parentLocations);

  const tagsStore = useTagStore();
  const tags = computed(() => tagsStore.tags);

  const itemTable = itemsTable(api);
  const stats = statCardData(api);
  const recentExport = computed<CsvSourceRequest<"entities">>(() => ({
    ...inventoryExport({ orderBy: "createdAt" }),
    boundary: { kind: "recent", limit: 5 },
  }));
  const recentCardsExport = computed(() =>
    captureInventoryView(recentExport.value, [], [], "card", id => t(`csv_export.fields.${id}`))
  );
  const locationsExport = computed<CsvSourceRequest<"locations">>(() => ({
    source: "locations",
    scope: "collection",
    filters: {},
    boundary: { kind: "roots" },
    sort: [{ column: "name", direction: "asc" }],
    columns: exportColumns(locationCardFields, id => t(`csv_export.fields.${id}`)),
  }));
  const tagsExport = computed<CsvSourceRequest<"tags">>(() => ({
    source: "tags",
    scope: "collection",
    filters: {},
    boundary: { kind: "all" },
    sort: [],
    columns: exportColumns(["name"], () => t("items.name")),
  }));
</script>

<template>
  <div data-dashboard>
    <BaseContainer class="flex flex-col gap-6">
      <DashboardHero />
      <section>
        <Subtitle> {{ $t("home.quick_statistics") }} </Subtitle>
        <div class="grid grid-cols-2 gap-2 md:gap-4 lg:grid-cols-4">
          <StatCard v-for="(stat, i) in stats" :key="i" :title="stat.label" :value="stat.value" :type="stat.type" />
        </div>
      </section>

      <section>
        <Subtitle> {{ $t("home.recently_added") }} </Subtitle>

        <RecordListExport
          v-if="!breakpoints.lg"
          class="mb-3"
          :request="recentCardsExport"
          :scope-label="$t('csv_export.recent', { limit: 5 })"
        />
        <p v-if="itemTable.items.length === 0" class="ml-2 text-sm">
          {{ $t("items.no_results") }}
        </p>
        <BaseCard v-if="breakpoints.lg">
          <Table
            :items="itemTable.items"
            :export-request="recentExport"
            :export-scope-label="$t('csv_export.recent', { limit: 5 })"
          />
        </BaseCard>
        <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <ItemCard v-for="item in itemTable.items" :key="item.id" :item="item" compact />
        </div>
      </section>

      <section>
        <Subtitle> {{ $t("home.storage_locations") }} </Subtitle>
        <RecordListExport class="mb-3" :request="locationsExport" />
        <p v-if="locations.length === 0" class="ml-2 text-sm">
          {{ $t("locations.no_results") }}
        </p>
        <div v-else class="grid grid-cols-1 gap-4 lg:grid-cols-2 xl:grid-cols-3">
          <LocationCard v-for="location in locations" :key="location.id" :location="location" />
        </div>
      </section>

      <section>
        <Subtitle> {{ $t("home.tags") }} </Subtitle>
        <RecordListExport class="mb-3" :request="tagsExport" />
        <p v-if="tags.length === 0" class="ml-2 text-sm">
          {{ $t("tags.no_results") }}
        </p>
        <div v-else class="flex flex-wrap gap-4">
          <TagChip v-for="tag in tags" :key="tag.id" size="lg" :tag="tag" />
        </div>
      </section>
    </BaseContainer>
  </div>
</template>

<style scoped>
  /* Table cell links use the same keyboard treatment as cards and chips. */
  section :deep(a:focus-visible) {
    @apply rounded-sm outline-none ring-2 ring-ring ring-offset-2 ring-offset-background;
  }
</style>
