<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { toast } from "@/components/ui/sonner";
  import type {
    EntityTypeCreate,
    EntityTypeSummary,
    EntityTypeUpdate,
    EntityTemplateSummary,
  } from "~~/lib/api/types/data-contracts";
  import MdiPlus from "~icons/mdi/plus";
  import MdiPencil from "~icons/mdi/pencil";
  import MdiDelete from "~icons/mdi/delete";
  import MdiMapMarkerOutline from "~icons/mdi/map-marker-outline";
  import MdiPackageVariantClosed from "~icons/mdi/package-variant-closed";
  import { Button } from "@/components/ui/button";
  import { Badge } from "@/components/ui/badge";
  import { Card } from "@/components/ui/card";
  import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
  import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
  import { useDialog } from "@/components/ui/dialog-provider";
  import { DialogID } from "~/components/ui/dialog-provider/utils";
  import FormTextField from "~/components/Form/TextField.vue";
  import FormCheckbox from "~/components/Form/Checkbox.vue";
  import TemplateSelector from "~/components/Template/Selector.vue";
  import { useEntityTypeStore } from "~/stores/entityTypes";

  const { t } = useI18n();

  useHead({ title: `HomeBox | ${t("collection.tabs.entity_types")}` });

  const api = useUserApi();
  const confirm = useConfirm();
  const { openDialog, closeDialog } = useDialog();
  const entityTypeStore = useEntityTypeStore();

  const entityTypes = computed(() => entityTypeStore.allTypes);
  const loading = ref(false);
  const loadError = ref<string | null>(null);
  const formError = ref<string | null>(null);

  async function loadTypes() {
    loading.value = true;
    loadError.value = null;
    try {
      await entityTypeStore.refresh();
    } catch {
      loadError.value = t("errors.api_failure");
    } finally {
      loading.value = false;
    }
  }

  onMounted(loadTypes);

  // Create form
  const createForm = reactive({
    name: "",
    icon: "",
    isLocation: false,
  });
  const createTemplate = ref<EntityTemplateSummary | null>(null);

  function resetCreateForm() {
    createForm.name = "";
    createForm.icon = "";
    createForm.isLocation = false;
    createTemplate.value = null;
  }

  async function create() {
    formError.value = null;
    if (!createForm.name.trim()) {
      formError.value = t("components.entityTypes.toasts.name_required");
      toast.error(formError.value);
      return;
    }

    const payload = {
      name: createForm.name,
      icon: createForm.icon,
      isLocation: createForm.isLocation,
      ...(createTemplate.value?.id ? { defaultTemplateId: createTemplate.value.id } : {}),
    } as EntityTypeCreate;

    const { error } = await api.entityTypes.create(payload);
    if (error) {
      formError.value = t("components.entityTypes.toasts.create_failed");
      toast.error(formError.value);
      return;
    }

    toast.success(t("components.entityTypes.toasts.create_success"));
    resetCreateForm();
    closeDialog(DialogID.CreateEntityType);
    await loadTypes();
  }

  // Update form
  const updateForm = reactive({
    id: "",
    name: "",
    icon: "",
    isLocation: false,
    originalIsItem: false,
  });
  const updateTemplate = ref<EntityTemplateSummary | null>(null);

  function openEdit(et: EntityTypeSummary) {
    formError.value = null;
    updateForm.id = et.id;
    updateForm.name = et.name;
    updateForm.icon = et.icon;
    updateForm.isLocation = et.isLocation;
    updateForm.originalIsItem = !et.isLocation;
    updateTemplate.value = et.defaultTemplate
      ? ({
          id: et.defaultTemplate.id,
          name: et.defaultTemplate.name,
          description: et.defaultTemplate.description,
        } as EntityTemplateSummary)
      : null;
    openDialog(DialogID.UpdateEntityType);
  }

  async function update() {
    formError.value = null;
    if (!updateForm.name.trim()) {
      formError.value = t("components.entityTypes.toasts.name_required");
      toast.error(formError.value);
      return;
    }

    const payload = {
      id: updateForm.id,
      name: updateForm.name,
      icon: updateForm.icon,
      isLocation: updateForm.isLocation,
      ...(updateTemplate.value?.id ? { defaultTemplateId: updateTemplate.value.id } : {}),
    } as EntityTypeUpdate;

    if (updateForm.originalIsItem && updateForm.isLocation) {
      const { isCanceled } = await confirm.open(t("components.entityTypes.confirm.convert_item_to_location"));
      if (isCanceled) return;
    }

    const { error } = await api.entityTypes.update(updateForm.id, payload);
    if (error) {
      formError.value = t("components.entityTypes.toasts.update_failed");
      toast.error(formError.value);
      return;
    }

    toast.success(t("components.entityTypes.toasts.update_success"));
    closeDialog(DialogID.UpdateEntityType);
    await loadTypes();
  }

  async function deleteEntityType(et: EntityTypeSummary) {
    const { isCanceled } = await confirm.open(
      t("components.entityTypes.confirm.delete_entity_type", { name: et.name })
    );
    if (isCanceled) return;

    const { error } = await api.entityTypes.delete(et.id);
    if (error) {
      toast.error(t("components.entityTypes.toasts.delete_confirm_failed"));
      return;
    }

    toast.success(t("components.entityTypes.toasts.delete_success"));
    await loadTypes();
  }
</script>

<template>
  <div class="min-w-0 space-y-4">
    <!-- Create Dialog -->
    <Dialog :dialog-id="DialogID.CreateEntityType">
      <DialogContent class="hb-panel">
        <DialogHeader>
          <DialogTitle>{{ t("components.entityTypes.create_dialog.title") }}</DialogTitle>
        </DialogHeader>
        <form class="flex flex-col gap-3" @submit.prevent="create">
          <FormTextField
            v-model="createForm.name"
            :autofocus="true"
            :label="t('components.entityTypes.create_dialog.name_label')"
            :max-length="255"
            :min-length="1"
          />
          <FormCheckbox
            v-model="createForm.isLocation"
            :label="t('components.entityTypes.create_dialog.is_container_location_type_label')"
          />
          <TemplateSelector v-if="!createForm.isLocation" v-model="createTemplate" />

          <p
            v-if="formError"
            role="alert"
            class="break-words rounded-md bg-destructive p-3 text-sm text-destructive-foreground"
          >
            {{ formError }}
          </p>
          <DialogFooter>
            <Button type="submit">{{ t("components.entityTypes.create_dialog.button") }}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <!-- Update Dialog -->
    <Dialog :dialog-id="DialogID.UpdateEntityType">
      <DialogContent class="hb-panel">
        <DialogHeader>
          <DialogTitle>{{ t("components.entityTypes.update_dialog.title") }}</DialogTitle>
        </DialogHeader>
        <form class="flex flex-col gap-3" @submit.prevent="update">
          <FormTextField
            v-model="updateForm.name"
            :autofocus="true"
            :label="t('components.entityTypes.update_dialog.name_label')"
            :max-length="255"
            :min-length="1"
          />
          <FormCheckbox
            v-model="updateForm.isLocation"
            :label="t('components.entityTypes.update_dialog.is_container_location_type_label')"
          />
          <TemplateSelector v-if="!updateForm.isLocation" v-model="updateTemplate" />

          <p
            v-if="formError"
            role="alert"
            class="break-words rounded-md bg-destructive p-3 text-sm text-destructive-foreground"
          >
            {{ formError }}
          </p>
          <DialogFooter>
            <Button type="submit">{{ t("components.entityTypes.update_dialog.button") }}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <!-- Page Content -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="hb-section-title">
        {{ t("components.entityTypes.page.title") }}
      </h2>
      <Button
        size="sm"
        class="h-auto min-h-10 whitespace-normal text-start"
        @click="
          formError = null;
          openDialog(DialogID.CreateEntityType);
        "
      >
        <MdiPlus class="mr-1 size-4" />
        {{ t("components.entityTypes.page.create") }}
      </Button>
    </div>

    <p v-if="loading" role="status" class="hb-panel p-4 text-sm text-muted-foreground">
      {{ t("global.loading") }}
    </p>
    <p
      v-else-if="loadError"
      role="alert"
      class="break-words rounded-md bg-destructive p-3 text-sm text-destructive-foreground"
    >
      {{ loadError }}
    </p>
    <div v-else-if="entityTypes && entityTypes.length > 0" class="flex flex-col gap-2">
      <Card v-for="et in entityTypes" :key="et.id" class="hb-panel min-w-0 p-4 sm:p-6">
        <div class="flex flex-wrap items-center gap-3">
          <div
            class="flex size-10 shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground"
          >
            <MdiMapMarkerOutline v-if="et.isLocation" class="size-5" />
            <MdiPackageVariantClosed v-else class="size-5" />
          </div>

          <div class="mr-auto min-w-0 flex-1 basis-32">
            <div class="flex flex-wrap items-center gap-2">
              <span class="break-words font-medium [overflow-wrap:anywhere]">{{ t(et.name) }}</span>
              <Badge v-if="et.isLocation" variant="secondary" class="text-xs">
                {{ t("components.entityTypes.card.badge_container") }}
              </Badge>
            </div>
            <p
              v-if="et.defaultTemplate && !et.isLocation"
              class="break-words text-sm text-muted-foreground [overflow-wrap:anywhere]"
            >
              {{
                t("components.entityTypes.card.default_template", {
                  name: et.defaultTemplate.name,
                })
              }}
            </p>
          </div>

          <TooltipProvider :delay-duration="0">
            <div class="flex gap-1">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button
                    variant="ghost"
                    size="icon"
                    class="size-10"
                    :aria-label="t('components.entityTypes.card.tooltip.edit')"
                    @click="openEdit(et)"
                  >
                    <MdiPencil class="size-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{{ t("components.entityTypes.card.tooltip.edit") }}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button
                    variant="destructive"
                    size="icon"
                    class="size-10"
                    :aria-label="t('components.entityTypes.card.tooltip.delete')"
                    @click="deleteEntityType(et)"
                  >
                    <MdiDelete class="size-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{{ t("components.entityTypes.card.tooltip.delete") }}</TooltipContent>
              </Tooltip>
            </div>
          </TooltipProvider>
        </div>
      </Card>
    </div>

    <div v-else role="status" class="hb-panel flex flex-col items-center justify-center p-6 text-center">
      <p class="mb-4 text-muted-foreground">
        {{ t("components.entityTypes.page.empty_title") }}
      </p>
      <Button
        @click="
          formError = null;
          openDialog(DialogID.CreateEntityType);
        "
      >
        <MdiPlus class="mr-2" />
        {{ t("components.entityTypes.page.empty_button") }}
      </Button>
    </div>
  </div>
</template>
