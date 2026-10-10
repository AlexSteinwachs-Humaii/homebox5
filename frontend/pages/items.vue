<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { toast } from "@/components/ui/sonner";
  import { Input } from "~/components/ui/input";
  import type { EntitySummary, TagSummary } from "~~/lib/api/types/data-contracts";
  import { useTagStore } from "~/stores/tags";
  import { useLocationStore } from "~~/stores/locations";
  import MdiLoading from "~icons/mdi/loading";
  import MdiMagnify from "~icons/mdi/magnify";
  import MdiDelete from "~icons/mdi/delete";
  import { Button } from "@/components/ui/button";
  import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
  import { Label } from "@/components/ui/label";
  import { Switch } from "@/components/ui/switch";
  import { Separator } from "@/components/ui/separator";
  import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
  import BaseContainer from "@/components/Base/Container.vue";
  import SearchFilter from "~/components/Search/Filter.vue";
  import ItemViewSelectable from "~/components/Item/View/Selectable.vue";
  import { inventoryExport } from "~/lib/exports/inventory";
  import type { LocationQueryRaw } from "vue-router";

  const { t } = useI18n();

  definePageMeta({
    middleware: ["auth"],
  });

  useHead({
    title: "HomeBox | " + t("global.items"),
  });

  const searchLocked = ref(false);
  const queryParamsInitialized = ref(false);
  const initialSearch = ref(true);
  const searchFailed = ref(false);

  const api = useUserApi();
  const loading = useMinLoader(500);
  const items = ref<EntitySummary[]>([]);
  const total = ref(0);

  // Using useRouteQuery directly has two downsides
  // 1. It persists the default value in the query string
  // 2. The ref returned by useRouteQuery updates asynchronously after calling the setter.
  //    This can cause unintuitive behaviors.
  // -> We copy query parameters into separate refs on page load and update the query explicitly via `router.push`.
  type QueryParamValue = string | string[] | number | boolean;
  type QueryRef = Ref<boolean | string | string[] | number, boolean | string | string[] | number>;
  const queryParamDefaultValues: Record<string, QueryParamValue> = {};
  function useOptionalRouteQuery(key: string, defaultValue: string): Ref<string>;
  function useOptionalRouteQuery(key: string, defaultValue: string[]): Ref<string[]>;
  function useOptionalRouteQuery(key: string, defaultValue: number): Ref<number>;
  function useOptionalRouteQuery(key: string, defaultValue: boolean): Ref<boolean>;
  function useOptionalRouteQuery(key: string, defaultValue: QueryParamValue): QueryRef {
    queryParamDefaultValues[key] = defaultValue;
    if (typeof defaultValue === "string") {
      const val = useRouteQuery(key, defaultValue);
      return ref(val.value);
    }
    if (Array.isArray(defaultValue)) {
      const val = useRouteQuery(key, defaultValue);
      return ref(val.value);
    }
    if (typeof defaultValue === "number") {
      const val = useRouteQuery(key, defaultValue);
      return ref(val.value);
    }
    if (typeof defaultValue === "boolean") {
      const val = useRouteQuery(key, defaultValue);
      return ref(val.value);
    }

    throw Error(`Invalid query value type ${typeof defaultValue}`);
  }

  const page1 = useOptionalRouteQuery("page", 1);

  const page = computed({
    get: () => page1.value,
    set: value => {
      page1.value = value;
    },
  });

  const query = useOptionalRouteQuery("q", "");
  const includeArchived = useOptionalRouteQuery("archived", false);
  const fieldSelector = useOptionalRouteQuery("fieldSelector", false);
  const negateTags = useOptionalRouteQuery("negateTags", false);
  const onlyWithoutPhoto = useOptionalRouteQuery("onlyWithoutPhoto", false);
  const onlyWithPhoto = useOptionalRouteQuery("onlyWithPhoto", false);
  const orderBy = useOptionalRouteQuery("orderBy", "name");
  const qLoc = useOptionalRouteQuery("loc", []);
  const qTag = useOptionalRouteQuery("tag", []);

  const preferences = useViewPreferences();
  const pageSize = computed(() => preferences.value.itemsPerTablePage);

  const route = useRoute();
  const router = useRouter();

  onMounted(async () => {
    loading.value = true;
    searchLocked.value = true;
    await Promise.all([locationsStore.ensureLocationsFetched(), tagStore.ensureAllTagsFetched()]);
    if (qLoc) {
      selectedLocations.value = locations.value.filter(l => qLoc.value.includes(l.id));
    }

    if (qTag) {
      selectedTags.value = tags.value.filter(l => qTag.value.includes(l.id));
    }

    queryParamsInitialized.value = true;
    searchLocked.value = false;

    const qFields = route.query.fields as string[];
    if (qFields) {
      fieldTuples.value = qFields.map(f => f.split("=") as [string, string]);

      for (const t of fieldTuples.value) {
        if (t[0] && t[1]) {
          await fetchValues(t[0]);
        }
      }
    }

    // trigger search if no changes
    if (!qTag && !qLoc) {
      search();
    }

    loading.value = false;
    window.scroll({
      top: 0,
      left: 0,
      behavior: "smooth",
    });
  });

  const locationsStore = useLocationStore();

  const locationFlatTree = useFlatLocations();

  const locations = computed(() => locationsStore.allLocations);

  const tagStore = useTagStore();
  const tags = computed(() => tagStore.tags);

  const selectedLocations = ref<EntitySummary[]>([]);
  const selectedTags = ref<TagSummary[]>([]);

  const locIDs = computed(() => selectedLocations.value.map(l => l.id));
  const tagIDs = computed(() => selectedTags.value.map(l => l.id));

  function parseAssetIDString(d: string) {
    d = d.replace(/"/g, "").replace(/-/g, "");

    const aidInt = parseInt(d);
    if (isNaN(aidInt)) {
      return [-1, false];
    }

    return [aidInt, true];
  }

  const byAssetId = computed(() => query.value?.startsWith("#") || false);
  const parsedAssetId = computed(() => {
    if (!byAssetId.value) {
      return "";
    } else {
      const [aid, valid] = parseAssetIDString(query.value.replace("#", ""));
      if (!valid) {
        return t("items.invalid_asset_id");
      } else {
        return aid;
      }
    }
  });

  const fieldTuples = ref<[string, string][]>([]);
  const fieldValuesCache = ref<Record<string, string[]>>({});

  const { data: allFields } = useAsyncData(async () => {
    const { data, error } = await api.items.fields.getAll();

    if (error) {
      return [];
    }

    return data;
  });

  watch(includeArchived, (newV, oldV) => {
    if (newV !== oldV) {
      search();
    }
  });

  watch(fieldSelector, (newV, oldV) => {
    if (newV === false && oldV === true) {
      fieldTuples.value = [];
    }
  });

  watch(negateTags, (newV, oldV) => {
    if (newV !== oldV) {
      search();
    }
  });

  watch(onlyWithoutPhoto, (newV, oldV) => {
    if (newV && onlyWithPhoto.value) {
      // this triggers the watch on onlyWithPhoto
      onlyWithPhoto.value = false;
    } else if (newV !== oldV) {
      search();
    }
  });

  watch(onlyWithPhoto, (newV, oldV) => {
    if (newV && onlyWithoutPhoto.value) {
      // this triggers the watch on onlyWithoutPhoto
      onlyWithoutPhoto.value = false;
    } else if (newV !== oldV) {
      search();
    }
  });

  watch(orderBy, (newV, oldV) => {
    if (newV !== oldV) {
      search();
    }
  });

  watch(
    () => useRoute().query.q,
    (newV, oldV) => {
      if (newV !== oldV) {
        query.value = (newV as string) || "";
      }
    }
  );

  async function fetchValues(field: string): Promise<string[]> {
    if (fieldValuesCache.value[field]) {
      return fieldValuesCache.value[field];
    }

    const { data, error } = await api.items.fields.getAllValues(field);

    if (error) {
      return [];
    }

    fieldValuesCache.value[field] = data;

    return data;
  }

  const exportRequest = computed(() =>
    inventoryExport({
      q: query.value,
      parentIds: locIDs.value,
      tags: tagIDs.value,
      negateTags: negateTags.value,
      onlyWithoutPhoto: onlyWithoutPhoto.value,
      onlyWithPhoto: onlyWithPhoto.value,
      includeArchived: includeArchived.value,
      orderBy: orderBy.value,
      fields: fieldTuples.value.filter(([name, value]) => name && value).map(([name, value]) => `${name}=${value}`),
    })
  );

  async function search() {
    if (searchLocked.value) {
      return;
    }

    loading.value = true;
    searchFailed.value = false;

    const fields = [];

    for (const t of fieldTuples.value) {
      if (t[0] && t[1]) {
        fields.push(`${t[0]}=${t[1]}`);
      }
    }

    const push_query: Record<string, string | string[] | number | boolean | undefined> = {
      archived: includeArchived.value,
      fieldSelector: fieldSelector.value,
      negateTags: negateTags.value,
      onlyWithoutPhoto: onlyWithoutPhoto.value,
      onlyWithPhoto: onlyWithPhoto.value,
      orderBy: orderBy.value,
      page: page.value,
      q: query.value,
      loc: locIDs.value,
      tag: tagIDs.value,
      fields: fields,
    };

    for (const key in push_query) {
      const val = push_query[key];
      const defaultVal = queryParamDefaultValues[key];
      if (
        (Array.isArray(val) &&
          Array.isArray(defaultVal) &&
          val.length == defaultVal.length &&
          val.every(v => (defaultVal as string[]).includes(v))) ||
        val === queryParamDefaultValues[key]
      ) {
        push_query[key] = undefined;
      }

      // Empirically seen to be unnecessary but according to router.push types,
      // booleans are not supported. This might be more stable.
      if (typeof push_query[key] === "boolean") {
        push_query[key] = String(val);
      }
    }

    await router.push({ query: push_query as LocationQueryRaw });

    const { data, error } = await api.items.getAll({
      q: query.value || "",
      parentIds: locIDs.value,
      tags: tagIDs.value,
      negateTags: negateTags.value,
      onlyWithoutPhoto: onlyWithoutPhoto.value,
      onlyWithPhoto: onlyWithPhoto.value,
      includeArchived: includeArchived.value,
      page: page.value,
      pageSize: pageSize.value,
      orderBy: orderBy.value,
      fields,
    });

    function resetItems() {
      page.value = Math.max(1, page.value - 1);
      loading.value = false;
      total.value = 0;
      items.value = [];
    }

    if (error) {
      resetItems();
      searchFailed.value = true;
      toast.error(t("items.toast.failed_search_items"));
      return;
    }

    if (!data.items || data.items.length === 0) {
      resetItems();
      return;
    }

    total.value = data.total;
    items.value = data.items;

    loading.value = false;
    initialSearch.value = false;
  }

  watchDebounced([page, pageSize, query, selectedTags, selectedLocations], search, { debounce: 250, maxWait: 1000 });

  async function submit() {
    // Set URL Params
    const fields = [];
    for (const t of fieldTuples.value) {
      if (t[0] && t[1]) {
        fields.push(`${t[0]}=${t[1]}`);
      }
    }

    // Reset Pagination
    page.value = 1;

    // Perform Search
    await search();
  }

  async function reset() {
    // Set URL Params
    const fields = [];
    for (const t of fieldTuples.value) {
      if (t[0] && t[1]) {
        fields.push(`${t[0]}=${t[1]}`);
      }
    }

    await search();
  }

  const pagination = proxyRefs({
    page,
    pageSize,
    totalSize: total,
    setPage: (newPage: number) => {
      page.value = newPage;
    },
  });
</script>

<template>
  <BaseContainer>
    <div v-if="locations && tags" class="hb-panel mb-4 min-w-0 space-y-3 p-4 sm:p-6" data-inventory-search>
      <div class="flex flex-wrap items-end gap-3 md:flex-nowrap">
        <div class="min-w-0 flex-1 basis-full md:basis-auto">
          <Label for="inventory-query" class="mb-2 block">{{ $t("global.search") }}</Label>
          <Input
            id="inventory-query"
            v-model:model-value="query"
            type="search"
            :placeholder="$t('global.search')"
            :aria-describedby="byAssetId ? 'inventory-asset-query' : undefined"
            class="h-12"
          />
          <div v-if="byAssetId" id="inventory-asset-query" role="status" class="break-words pt-2 text-sm">
            <p>{{ $t("items.query_id", { id: parsedAssetId }) }}</p>
          </div>
        </div>
        <Button class="h-auto min-h-12 w-full whitespace-normal md:w-auto" @click.prevent="submit">
          <MdiLoading v-if="loading" class="animate-spin" />
          <MdiMagnify v-else />
          {{ $t("global.search") }}
        </Button>
      </div>

      <div class="flex min-w-0 flex-wrap items-center gap-2">
        <SearchFilter v-model="selectedLocations" :label="$t('global.locations')" :options="locationFlatTree" />
        <SearchFilter v-model="selectedTags" :label="$t('global.tags')" :options="tags" />
        <Popover>
          <PopoverTrigger as-child>
            <Button size="sm" variant="outline" class="h-auto min-h-9 whitespace-normal">
              {{ $t("items.options") }}</Button
            >
          </PopoverTrigger>
          <PopoverContent
            class="z-40 flex max-h-[70vh] w-80 max-w-[calc(100vw-2rem)] flex-col gap-3 overflow-y-auto break-words"
            align="start"
          >
            <Label for="inventory-archived" class="flex cursor-pointer items-center gap-3">
              <Switch id="inventory-archived" v-model="includeArchived" class="shrink-0" />
              <span class="min-w-0">{{ $t("items.include_archive") }}</span>
            </Label>
            <Label for="inventory-fields" class="flex cursor-pointer items-center gap-3">
              <Switch id="inventory-fields" v-model="fieldSelector" class="shrink-0" />
              <span class="min-w-0">{{ $t("items.field_selector") }}</span>
            </Label>
            <Label for="inventory-negate-tags" class="flex cursor-pointer items-center gap-3">
              <Switch id="inventory-negate-tags" v-model="negateTags" class="shrink-0" />
              <span class="min-w-0">{{ $t("items.negate_tags") }}</span>
            </Label>
            <Label for="inventory-without-photo" class="flex cursor-pointer items-center gap-3">
              <Switch id="inventory-without-photo" v-model="onlyWithoutPhoto" class="shrink-0" />
              <span class="min-w-0">{{ $t("items.only_without_photo") }}</span>
            </Label>
            <Label for="inventory-with-photo" class="flex cursor-pointer items-center gap-3">
              <Switch id="inventory-with-photo" v-model="onlyWithPhoto" class="shrink-0" />
              <span class="min-w-0">{{ $t("items.only_with_photo") }}</span>
            </Label>
            <Label for="inventory-order" class="flex flex-col gap-2">
              <span class="text-right">
                <span class="text-right"> {{ $t("items.order_by") }} </span>
              </span>

              <Select v-model="orderBy">
                <SelectTrigger
                  id="inventory-order"
                  class="h-auto min-h-10 [&>span]:min-w-0 [&>span]:whitespace-normal [&>span]:break-words"
                >
                  <SelectValue :placeholder="$t('items.order_by')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="name"> {{ $t("items.name") }} </SelectItem>
                  <SelectItem value="createdAt">
                    {{ $t("items.created_at") }}
                  </SelectItem>
                  <SelectItem value="updatedAt">
                    {{ $t("items.updated_at") }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </Label>
            <Separator />
            <Button class="h-auto min-h-9 whitespace-normal" @click="reset">
              {{ $t("items.reset_search") }}
            </Button>
          </PopoverContent>
        </Popover>
        <div class="grow" />
        <Popover>
          <PopoverTrigger as-child>
            <Button size="sm" variant="outline" class="h-auto min-h-9 whitespace-normal">
              {{ $t("items.tips") }}</Button
            >
          </PopoverTrigger>
          <PopoverContent
            class="z-40 max-h-[70vh] w-[325px] max-w-[calc(100vw-2rem)] overflow-y-auto break-words"
            align="end"
          >
            <p class="text-base">{{ $t("items.tips_sub") }}</p>
            <ul class="mt-1 list-disc pl-6 text-sm">
              <li>
                {{ $t("items.tip_1") }}
              </li>
              <li>
                {{ $t("items.tip_2") }}
              </li>
              <li>
                {{ $t("items.tip_3") }}
              </li>
            </ul>
          </PopoverContent>
        </Popover>
      </div>
      <div v-if="fieldSelector" class="flex min-w-0 flex-col gap-3 border-t pt-4">
        <p class="hb-section-title">{{ $t("items.custom_fields") }}</p>
        <div v-for="(f, idx) in fieldTuples" :key="idx" class="flex flex-wrap gap-2">
          <div class="flex min-w-0 flex-1 basis-full flex-col gap-1 md:basis-0">
            <Label :for="`inventory-field-${idx}`">
              {{ $t("items.field") }}
            </Label>
            <Select v-model="fieldTuples[idx]![0]" @update:model-value="fetchValues(f[0])">
              <SelectTrigger
                :id="`inventory-field-${idx}`"
                class="h-auto min-h-10 [&>span]:min-w-0 [&>span]:whitespace-normal [&>span]:break-words"
              >
                <SelectValue :placeholder="$t('items.select_field')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="field in allFields" :key="field" :value="field">
                  {{ field }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="flex min-w-0 flex-1 basis-full flex-col gap-1 md:basis-0">
            <Label :for="`inventory-field-value-${idx}`">
              {{ $t("items.field_value") }}
            </Label>
            <Select v-model="fieldTuples[idx]![1]">
              <SelectTrigger
                :id="`inventory-field-value-${idx}`"
                class="h-auto min-h-10 [&>span]:min-w-0 [&>span]:whitespace-normal [&>span]:break-words"
              >
                <SelectValue :placeholder="$t('items.select_value')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="value in fieldValuesCache[f[0]]" :key="value" :value="value">
                  {{ value }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Button
            variant="destructive"
            type="button"
            size="icon"
            class="my-auto shrink-0"
            :aria-label="$t('global.delete')"
            @click="fieldTuples.splice(idx, 1)"
          >
            <MdiDelete />
          </Button>
        </div>
        <Button
          type="button"
          size="sm"
          class="mt-2 h-auto min-h-9 whitespace-normal"
          @click="() => fieldTuples.push(['', ''])"
        >
          {{ $t("items.add") }}
        </Button>
      </div>
    </div>

    <section :aria-busy="loading">
      <p v-if="loading" role="status" class="hb-panel mb-3 flex items-center gap-2 p-4">
        <MdiLoading class="shrink-0 animate-spin" aria-hidden="true" />
        {{ $t("global.loading") }}
      </p>
      <p v-else-if="searchFailed" role="alert" class="hb-panel mb-3 break-words border-destructive p-4">
        {{ $t("items.toast.failed_search_items") }}
      </p>
      <ItemViewSelectable
        v-show="items.length > 0 || (!loading && !searchFailed)"
        :items="items"
        :export-request="exportRequest"
        :location-flat-tree="locationFlatTree"
        :pagination="pagination"
        disable-sort
        @refresh="async () => search()"
      />
    </section>
  </BaseContainer>
</template>
