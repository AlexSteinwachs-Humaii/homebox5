<script setup lang="ts">
  import RecordListExport from "~/components/Export/RecordListExport.vue";
  import { maintenanceExport, administrationLabelKeys } from "~/lib/exports/administration";
  import { useI18n } from "vue-i18n";
  import type { MaintenanceEntry, MaintenanceEntryWithDetails } from "~~/lib/api/types/data-contracts";
  import { MaintenanceFilterStatus } from "~~/lib/api/types/data-contracts";
  import type { StatsFormat } from "~~/components/global/StatCard/types";
  import MdiCheck from "~icons/mdi/check";
  import MdiDelete from "~icons/mdi/delete";
  import MdiEdit from "~icons/mdi/edit";
  import MdiCalendar from "~icons/mdi/calendar";
  import MdiPlus from "~icons/mdi/plus";

  import MdiContentDuplicate from "~icons/mdi/content-duplicate";
  import MaintenanceEditModal from "~~/components/Maintenance/EditModal.vue";
  import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
  import { Badge } from "@/components/ui/badge";
  import { Button, ButtonGroup } from "@/components/ui/button";
  import StatCard from "~/components/global/StatCard/StatCard.vue";
  import BaseCard from "@/components/Base/Card.vue";
  import BaseSectionHeader from "@/components/Base/SectionHeader.vue";
  import DateTime from "~/components/global/DateTime.vue";
  import Currency from "~/components/global/Currency.vue";
  import Markdown from "~/components/global/Markdown.vue";
  import { toast } from "@/components/ui/sonner";
  import { useDialog } from "@/components/ui/dialog-provider";
  import { toDateOnlyString } from "~/lib/datelib/dateOnly";
  import { DialogID } from "../ui/dialog-provider/utils";

  const maintenanceFilterStatus = ref(MaintenanceFilterStatus.MaintenanceFilterStatusScheduled);

  const api = useUserApi();
  const { t } = useI18n();
  const exportLabel = (id: string) => t(administrationLabelKeys[id] ?? `global.${id}`);
  const confirm = useConfirm();
  const { openDialog } = useDialog();

  const props = defineProps({
    currentItemId: {
      type: String,
      default: undefined,
    },
  });

  const loadFailed = ref(false);
  const toolbar = ref<HTMLElement | null>(null);
  const {
    data: maintenanceDataList,
    refresh: refreshList,
    status,
  } = useAsyncData(
    async () => {
      const { data, error } =
        props.currentItemId !== undefined
          ? await api.items.maintenance.getLog(props.currentItemId, { status: maintenanceFilterStatus.value })
          : await api.maintenance.getAll({ status: maintenanceFilterStatus.value });
      loadFailed.value = !!error;
      return error ? [] : data;
    },
    {
      watch: [maintenanceFilterStatus],
    }
  );

  const pending = computed(() => status.value === "pending");

  const stats = computed(() => {
    if (!maintenanceDataList.value) return [];

    const count = maintenanceDataList.value ? maintenanceDataList.value.length || 0 : 0;
    let total = 0;
    maintenanceDataList.value.forEach(item => {
      total += parseFloat(item.cost);
    });

    const average = count > 0 ? total / count : 0;

    return [
      {
        id: "count",
        title: t("maintenance.total_entries"),
        value: count,
        type: "number" as StatsFormat,
      },
      {
        id: "total",
        title: t("maintenance.total_cost"),
        value: total,
        type: "currency" as StatsFormat,
      },
      {
        id: "average",
        title: t("maintenance.monthly_average"),
        value: average,
        type: "currency" as StatsFormat,
      },
    ];
  });

  async function deleteEntry(id: string) {
    const trigger = document.activeElement as HTMLElement | null;
    try {
      const result = await confirm.open(t("maintenance.modal.delete_confirmation"));
      if (result.isCanceled) return;
      const { error } = await api.maintenance.delete(id);
      if (error) {
        toast.error(t("maintenance.toast.failed_to_delete"));
        return;
      }
      await refreshList();
    } finally {
      await nextTick();
      if (trigger?.isConnected) trigger.focus();
      else toolbar.value?.querySelector<HTMLButtonElement>("button")?.focus();
    }
  }

  async function completeEntry(maintenanceEntry: MaintenanceEntry) {
    const { error } = await api.maintenance.update(maintenanceEntry.id, {
      name: maintenanceEntry.name,
      // Local YYYY-MM-DD — using a Date object would JSON-stringify to UTC and
      // shift the day for users east of UTC.
      completedDate: toDateOnlyString(new Date()),
      scheduledDate: (maintenanceEntry.scheduledDate as string) ?? "",
      description: maintenanceEntry.description,
      cost: maintenanceEntry.cost,
    });
    if (error) {
      toast.error(t("maintenance.toast.failed_to_update"));
    }
    refreshList();
  }
</script>

<template>
  <section class="min-w-0 space-y-6">
    <div class="grid grid-cols-1 gap-6 md:grid-cols-3">
      <StatCard v-for="stat in stats" :key="stat.id" :title="stat.title" :value="stat.value" :type="stat.type" />
    </div>
    <div ref="toolbar" class="flex flex-wrap items-center gap-3">
      <ButtonGroup class="flex flex-wrap gap-2">
        <Button
          size="sm"
          :aria-pressed="maintenanceFilterStatus === MaintenanceFilterStatus.MaintenanceFilterStatusScheduled"
          :variant="
            maintenanceFilterStatus == MaintenanceFilterStatus.MaintenanceFilterStatusScheduled ? 'default' : 'outline'
          "
          @click="maintenanceFilterStatus = MaintenanceFilterStatus.MaintenanceFilterStatusScheduled"
        >
          {{ $t("maintenance.filter.scheduled") }}
        </Button>
        <Button
          size="sm"
          :variant="
            maintenanceFilterStatus == MaintenanceFilterStatus.MaintenanceFilterStatusCompleted ? 'default' : 'outline'
          "
          :aria-pressed="maintenanceFilterStatus === MaintenanceFilterStatus.MaintenanceFilterStatusCompleted"
          @click="maintenanceFilterStatus = MaintenanceFilterStatus.MaintenanceFilterStatusCompleted"
        >
          {{ $t("maintenance.filter.completed") }}
        </Button>
        <Button
          size="sm"
          :variant="
            maintenanceFilterStatus == MaintenanceFilterStatus.MaintenanceFilterStatusBoth ? 'default' : 'outline'
          "
          :aria-pressed="maintenanceFilterStatus === MaintenanceFilterStatus.MaintenanceFilterStatusBoth"
          @click="maintenanceFilterStatus = MaintenanceFilterStatus.MaintenanceFilterStatusBoth"
        >
          {{ $t("maintenance.filter.both") }}
        </Button>
      </ButtonGroup>
      <RecordListExport :request="maintenanceExport(maintenanceFilterStatus, props.currentItemId, exportLabel)" />
      <Button
        v-if="props.currentItemId"
        class="ml-auto"
        size="sm"
        @click="
          openDialog(DialogID.EditMaintenance, {
            params: { type: 'create', itemId: props.currentItemId },
            onClose: result => {
              if (result) {
                refreshList();
              }
            },
          })
        "
      >
        <MdiPlus />
        {{ $t("maintenance.list.new") }}
      </Button>
    </div>
  </section>
  <section class="min-w-0" :aria-busy="pending">
    <!-- begin -->
    <MaintenanceEditModal ref="maintenanceEditModal" @changed="refreshList" />
    <div v-if="pending && !maintenanceDataList?.length" class="hb-panel p-6" role="status">
      {{ $t("global.loading") }}
    </div>
    <div v-else-if="loadFailed" class="hb-panel space-y-3 p-6" role="alert">
      <p>{{ $t("maintenance.failed_load") }}</p>
      <Button variant="outline" @click="refreshList()">{{ $t("global.retry") }}</Button>
    </div>
    <div v-else class="space-y-6">
      <BaseCard v-for="e in maintenanceDataList" :key="e.id" class="hb-panel min-w-0">
        <BaseSectionHeader class="border-b p-4 sm:p-6">
          <span class="mb-2 min-w-0 break-words [overflow-wrap:anywhere]">
            <span v-if="!props.currentItemId">
              <NuxtLink class="hover:underline" :to="`/item/${(e as MaintenanceEntryWithDetails).itemID}/maintenance`">
                {{ (e as MaintenanceEntryWithDetails).itemName }}
              </NuxtLink>
              -
            </span>
            <span>{{ e.name }}</span>
          </span>
          <template #description>
            <div class="flex flex-wrap gap-2">
              <Badge v-if="validDate(e.completedDate)" variant="outline">
                <MdiCheck class="mr-2" />
                <DateTime :date="e.completedDate" format="human" datetime-type="date" />
              </Badge>
              <Badge v-else-if="validDate(e.scheduledDate)" variant="outline">
                <MdiCalendar class="mr-2" />
                <DateTime :date="e.scheduledDate" format="human" datetime-type="date" />
              </Badge>
              <TooltipProvider :delay-duration="0">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Badge tabindex="0" :aria-label="$t('maintenance.modal.cost')">
                      <Currency :amount="e.cost" />
                    </Badge>
                  </TooltipTrigger>
                  <TooltipContent> {{ $t("maintenance.modal.cost") }} </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
          </template>
        </BaseSectionHeader>
        <div :class="{ 'break-words p-4 [overflow-wrap:anywhere] sm:p-6': e.description }">
          <Markdown :source="e.description" />
        </div>
        <ButtonGroup class="flex flex-wrap justify-end gap-2 p-4">
          <Button
            size="sm"
            @click="
              openDialog(DialogID.EditMaintenance, {
                params: { type: 'update', maintenanceEntry: e },
                onClose: result => {
                  if (result) {
                    refreshList();
                  }
                },
              })
            "
          >
            <MdiEdit />
            {{ $t("maintenance.list.edit") }}
          </Button>
          <Button v-if="!validDate(e.completedDate)" size="sm" variant="outline" @click="completeEntry(e)">
            <MdiCheck />
            {{ $t("maintenance.list.complete") }}
          </Button>
          <Button
            size="sm"
            variant="outline"
            @click="
              openDialog(DialogID.EditMaintenance, {
                params: {
                  type: 'duplicate',
                  maintenanceEntry: e,
                  itemId: props.currentItemId ?? (e as MaintenanceEntryWithDetails).itemID,
                },
                onClose: result => {
                  if (result) {
                    refreshList();
                  }
                },
              })
            "
          >
            <MdiContentDuplicate />
            {{ $t("maintenance.list.duplicate") }}
          </Button>
          <Button size="sm" variant="destructive" @click="deleteEntry(e.id)">
            <MdiDelete />
            {{ $t("maintenance.list.delete") }}
          </Button>
        </ButtonGroup>
      </BaseCard>
      <div v-if="!maintenanceDataList?.length" class="hb-panel space-y-4 p-6 text-center">
        <p class="text-muted-foreground">{{ $t("maintenance.empty") }}</p>
        <Button
          v-if="props.currentItemId"
          variant="outline"
          class="h-auto whitespace-normal"
          @click="
            openDialog(DialogID.EditMaintenance, {
              params: { type: 'create', itemId: props.currentItemId },
              onClose: result => {
                if (result) {
                  refreshList();
                }
              },
            })
          "
        >
          <MdiPlus />
          <span class="mt-2 block text-sm font-medium text-foreground">
            {{ $t("maintenance.list.create_first") }}
          </span>
        </Button>
      </div>
    </div>
  </section>
</template>
