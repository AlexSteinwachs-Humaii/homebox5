<template>
  <Dialog :dialog-id="DialogID.ProductImport">
    <DialogContent class="min-w-0 md:max-w-xl lg:max-w-4xl">
      <DialogHeader>
        <DialogTitle class="font-editorial text-2xl font-normal">{{
          $t("components.item.product_import.title")
        }}</DialogTitle>
      </DialogHeader>

      <div v-if="errorMessage" class="hb-scanner-feedback" role="alert">
        <MdiAlertCircleOutline class="size-5 shrink-0" aria-hidden="true" />
        <span class="text-sm font-medium">{{ errorMessage }}</span>
      </div>

      <div class="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-end" :aria-busy="searching">
        <FormTextField
          v-model="barcode"
          :disabled="searching"
          class="min-w-0 flex-1"
          :label="$t('components.item.product_import.barcode')"
          @keyup.enter="retrieveProductInfo(barcode)"
        />
        <Button
          :variant="searching ? 'destructive' : 'default'"
          class="h-auto min-h-11 whitespace-normal"
          @click="retrieveProductInfo(barcode)"
        >
          <MdiLoading v-if="searching" class="animate-spin" />
          <div v-if="!searching" class="relative mx-2">
            <div class="absolute inset-0 flex items-center justify-center">
              <MdiBarcode class="size-5 group-hover:hidden" />
            </div>
          </div>
          {{ searching ? $t("global.cancel") : $t("components.item.product_import.search_item") }}
        </Button>
      </div>

      <Separator />

      <RecordListExport
        v-if="(products?.length ?? 0) <= 1000"
        :request="
          snapshotExport(
            'product-results',
            (products ?? []).map(p => ({
              name: p.item.name,
              manufacturer: p.manufacturer,
              modelNumber: p.modelNumber,
              searchEngineName: p.search_engine_name,
            })),
            id => t(`csv_export.fields.${id}`)
          )
        "
      />
      <p v-else role="alert">{{ $t("csv_export.snapshot_limit") }}</p>
      <BaseCard class="min-w-0 overflow-hidden">
        <Table class="w-full whitespace-nowrap">
          <TableHeader>
            <TableRow>
              <TableHead v-for="h in headers" :key="h.value" class="bg-muted text-sm font-medium text-muted-foreground">
                <div
                  class="flex items-center gap-1"
                  :class="{
                    'justify-center': h.align === 'center',
                  }"
                >
                  <template v-if="typeof h === 'string'">{{ h }}</template>
                  <template v-else>{{ $t(h.text) }}</template>
                </div>
              </TableHead>
            </TableRow>
          </TableHeader>

          <TableBody>
            <TableRow
              v-for="(p, index) in products"
              :key="index"
              class="cursor-pointer"
              :class="{
                'bg-accent text-accent-foreground hover:bg-accent': selectedRow === index,
              }"
              @click="selectProduct(index)"
            >
              <TableCell
                v-for="h in headers"
                :key="h.value"
                :class="{
                  'text-center': h.align === 'center',
                }"
              >
                <template v-if="h.type === 'name'">
                  <div class="flex items-center space-x-4">
                    <img :src="p.imageBase64" class="w-16 rounded object-fill shadow-sm" alt="Product's photo" />
                    <button
                      type="button"
                      class="hb-focus min-h-11 text-left text-sm font-medium underline underline-offset-4"
                      :aria-pressed="selectedRow === index"
                      :aria-label="
                        $t('components.item.product_import.select_product', {
                          name: p.item.name,
                        })
                      "
                      @click.stop="selectProduct(index)"
                    >
                      {{ p.item.name }}
                    </button>
                  </div>
                </template>
                <template v-else-if="h.type === 'url'">
                  <NuxtLink
                    class="hb-focus underline"
                    :to="'https://' + extractValue(p, h.value)"
                    target="_blank"
                    rel="noopener noreferrer"
                    >{{ extractValue(p, h.value) }}</NuxtLink
                  >
                </template>

                <slot v-else :name="cell(h)">
                  {{ extractValue(p, h.value) }}
                </slot>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </BaseCard>

      <DialogFooter>
        <Button type="button" :disabled="selectedRow === -1" @click="createItem">
          {{ $t("components.item.product_import.import_selected") }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
  import RecordListExport from "~/components/Export/RecordListExport.vue";
  import { snapshotExport } from "~/lib/exports/auxiliary";
  import { useI18n } from "vue-i18n";
  import { DialogID } from "@/components/ui/dialog-provider/utils";
  import { Button } from "~/components/ui/button";
  import type { BarcodeProduct } from "~~/lib/api/types/data-contracts";
  import { useDialog } from "~/components/ui/dialog-provider";
  import MdiAlertCircleOutline from "~icons/mdi/alert-circle-outline";
  import MdiBarcode from "~icons/mdi/barcode";
  import MdiLoading from "~icons/mdi/loading";
  import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
  import { Separator } from "@/components/ui/separator";
  import BaseCard from "@/components/Base/Card.vue";
  import FormTextField from "@/components/Form/TextField.vue";

  const { openDialog, registerOpenDialogCallback } = useDialog();
  const { t } = useI18n();

  const searching = ref(false);
  const barcode = ref<string>("");
  const products = ref<BarcodeProduct[] | null>(null);
  const selectedRow = ref(-1);
  const errorMessage = ref<string | null>(null);

  type BarcodeTableHeader = {
    text: string;
    value: string;
    align?: "left" | "center" | "right";
    type?: "name" | "url";
  };

  const defaultHeaders = [
    {
      text: "items.name",
      value: "name",
      align: "center",
      type: "name",
    },
    { text: "items.manufacturer", value: "manufacturer", align: "center" },
    { text: "items.model_number", value: "modelNumber", align: "center" },
    {
      text: "components.item.product_import.db_source",
      value: "search_engine_name",
      align: "center",
      type: "url",
    },
  ] satisfies BarcodeTableHeader[];

  // Need for later filtering
  const headers = defaultHeaders;

  onMounted(() => {
    const cleanup = registerOpenDialogCallback(DialogID.ProductImport, params => {
      selectedRow.value = -1;
      searching.value = false;
      errorMessage.value = null;

      if (params?.barcode) {
        // Reset if the barcode is different
        if (params.barcode !== barcode.value) {
          barcode.value = params.barcode;

          retrieveProductInfo(barcode.value).then(() => {
            console.log("Processing finished");
          });
        }
      } else {
        barcode.value = "";
        products.value = null;
      }
    });

    onUnmounted(cleanup);
  });

  const api = useUserApi();

  function createItem() {
    if (
      products.value !== null &&
      products.value.length > 0 &&
      selectedRow.value >= 0 &&
      selectedRow.value < products.value.length
    ) {
      const p = products.value![selectedRow.value];
      openDialog(DialogID.CreateEntity, {
        params: {
          baseType: "item",
          product: p,
        },
      });
    }
  }

  async function retrieveProductInfo(barcode: string) {
    errorMessage.value = null;

    if (!barcode || barcode.trim().length === 0 || !/^[0-9]+$/.test(barcode)) {
      errorMessage.value = t("components.item.product_import.error_invalid_barcode");
      console.error(errorMessage.value);
      return;
    }

    products.value = null;
    searching.value = true;

    try {
      const result = await api.products.searchFromBarcode(barcode.trim());
      if (result.error) {
        errorMessage.value = t("errors.api_failure") + result.error;
        console.error(errorMessage.value);
      } else {
        if (result.data === undefined || result.data.length === undefined || result.data.length === 0) {
          errorMessage.value = t("components.item.product_import.error_not_found");
        }

        products.value = result.data;
      }
    } catch (error) {
      errorMessage.value = t("components.item.product_import.error_exception") + error;
      console.error(errorMessage.value);
    } finally {
      searching.value = false;
    }
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function extractValue(data: Record<string, any>, value: string) {
    const parts = value.split(".");
    let current = data;
    for (const part of parts) {
      current = current[part];
    }
    return current;
  }

  function cell(h: BarcodeTableHeader) {
    return `cell-${h.value.replace(".", "_")}`;
  }

  function selectProduct(index: number) {
    // Unselect if already selected
    if (selectedRow.value === index) {
      selectedRow.value = -1;
      return;
    }

    selectedRow.value = index;
  }
</script>
