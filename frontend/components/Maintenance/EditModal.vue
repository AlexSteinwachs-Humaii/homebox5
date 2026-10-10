<template>
  <Dialog :dialog-id="DialogID.EditMaintenance">
    <DialogContent @close-auto-focus="restoreFocus">
      <DialogHeader>
        <DialogTitle>
          {{ entry.id ? $t("maintenance.modal.edit_title") : $t("maintenance.modal.new_title") }}
        </DialogTitle>
      </DialogHeader>

      <form class="flex min-w-0 flex-col gap-4" :aria-busy="saving" @submit.prevent="dispatchFormSubmit">
        <FormTextField v-model="entry.name" autofocus :label="$t('maintenance.modal.entry_name')" />
        <DatePicker v-model="entry.completedDate" :label="$t('maintenance.modal.completed_date')" />
        <DatePicker v-model="entry.scheduledDate" :label="$t('maintenance.modal.scheduled_date')" />
        <FormTextArea v-model="entry.description" :label="$t('maintenance.modal.notes')" />
        <FormTextField v-model="entry.cost" :label="$t('maintenance.modal.cost')" />

        <DialogFooter>
          <p v-if="saveError" role="alert" class="rounded-md border border-destructive p-3 text-popover-foreground">
            {{ saveError }}
          </p>
          <Button type="submit" :disabled="saving">
            <MdiPost />
            {{
              saving
                ? $t("global.loading")
                : entry.id
                  ? $t("maintenance.modal.edit_action")
                  : $t("maintenance.modal.new_action")
            }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { DialogID } from "@/components/ui/dialog-provider/utils";
  import { toast } from "@/components/ui/sonner";
  import MdiPost from "~icons/mdi/post";
  import DatePicker from "~~/components/Form/DatePicker.vue";
  import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
  import { useDialog } from "@/components/ui/dialog-provider";
  import FormTextField from "~/components/Form/TextField.vue";
  import FormTextArea from "~/components/Form/TextArea.vue";
  import Button from "@/components/ui/button/Button.vue";

  const { closeDialog, registerOpenDialogCallback } = useDialog();

  const { t } = useI18n();
  const api = useUserApi();

  const entry = reactive({
    id: null as string | null,
    name: "",
    completedDate: "",
    scheduledDate: "",
    description: "",
    cost: "",
    itemIds: null as string[] | null,
  });

  const saving = ref(false);
  const saveError = ref("");
  let trigger: HTMLElement | null = null;
  function restoreFocus(event: Event) {
    if (trigger?.isConnected) {
      event.preventDefault();
      trigger.focus();
    }
  }

  async function dispatchFormSubmit() {
    if (saving.value) return;
    saving.value = true;
    saveError.value = "";
    try {
      if (entry.id) await editEntry();
      else await createEntry();
    } catch {
      saveError.value = t(entry.id ? "maintenance.toast.failed_to_update" : "maintenance.toast.failed_to_create");
      toast.error(saveError.value);
    } finally {
      saving.value = false;
    }
  }

  async function createEntry() {
    if (!entry.itemIds) {
      return;
    }

    const results = await Promise.allSettled(
      entry.itemIds.map(async itemId => {
        const { error } = await api.items.maintenance.create(itemId, {
          name: entry.name,
          completedDate: entry.completedDate,
          scheduledDate: entry.scheduledDate,
          description: entry.description,
          cost: parseFloat(entry.cost) ? entry.cost : "0",
        });

        if (error) {
          throw new Error("Failed to create entry");
        }
      })
    );

    // Retain only failed targets so retrying a partial batch cannot duplicate successes.
    entry.itemIds = entry.itemIds.filter((_, index) => results[index]?.status === "rejected");
    if (entry.itemIds.length) {
      saveError.value = t("maintenance.toast.failed_to_create");
      toast.error(saveError.value);
      return;
    }
    closeDialog(DialogID.EditMaintenance, true);
  }

  async function editEntry() {
    if (!entry.id) {
      return;
    }

    const { error } = await api.maintenance.update(entry.id, {
      name: entry.name,
      completedDate: entry.completedDate,
      scheduledDate: entry.scheduledDate,
      description: entry.description,
      cost: parseFloat(entry.cost) ? entry.cost : "0",
    });

    if (error) {
      saveError.value = t("maintenance.toast.failed_to_update");
      toast.error(saveError.value);
      return;
    }

    closeDialog(DialogID.EditMaintenance, true);
  }

  onMounted(() => {
    const cleanup = registerOpenDialogCallback(DialogID.EditMaintenance, params => {
      trigger = document.activeElement as HTMLElement | null;
      saveError.value = "";
      switch (params.type) {
        case "create":
          entry.id = null;
          entry.name = "";
          entry.completedDate = "";
          entry.scheduledDate = "";
          entry.description = "";
          entry.cost = "";
          entry.itemIds = typeof params.itemId === "string" ? [params.itemId] : params.itemId;
          break;
        case "update":
          entry.id = params.maintenanceEntry.id;
          entry.name = params.maintenanceEntry.name;
          // Backend already returns YYYY-MM-DD via types.Date — keep as string
          // so JSON.stringify never converts to a UTC instant.
          entry.completedDate = (params.maintenanceEntry.completedDate as string) ?? "";
          entry.scheduledDate = (params.maintenanceEntry.scheduledDate as string) ?? "";
          entry.description = params.maintenanceEntry.description;
          entry.cost = params.maintenanceEntry.cost;
          entry.itemIds = null;
          break;
        case "duplicate":
          entry.id = null;
          entry.name = params.maintenanceEntry.name;
          entry.completedDate = "";
          entry.scheduledDate = "";
          entry.description = params.maintenanceEntry.description;
          entry.cost = params.maintenanceEntry.cost;
          entry.itemIds = [params.itemId];
          break;
      }
    });

    onUnmounted(cleanup);
  });
</script>
