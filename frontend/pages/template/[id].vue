<script setup lang="ts">
  import RecordListExport from "~/components/Export/RecordListExport.vue";
  import { templateFieldsExport, administrationLabelKeys } from "~/lib/exports/administration";
  import { useI18n } from "vue-i18n";
  import { toast } from "@/components/ui/sonner";
  import MdiPencil from "~icons/mdi/pencil";
  import MdiDelete from "~icons/mdi/delete";
  import MdiPlus from "~icons/mdi/plus";
  import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
  import { useDialog } from "@/components/ui/dialog-provider";
  import { Card } from "@/components/ui/card";
  import { Button } from "@/components/ui/button";
  import { Separator } from "@/components/ui/separator";
  import { Switch } from "@/components/ui/switch";
  import { Label } from "@/components/ui/label";
  import { DialogID } from "~/components/ui/dialog-provider/utils";
  import FormTextField from "~/components/Form/TextField.vue";
  import FormTextArea from "~/components/Form/TextArea.vue";
  import BaseContainer from "@/components/Base/Container.vue";
  import DateTime from "~/components/global/DateTime.vue";
  import Markdown from "~/components/global/Markdown.vue";
  import LocationSelector from "~/components/Location/Selector.vue";
  import TagSelector from "~/components/Tag/Selector.vue";
  import { useTagStore } from "~/stores/tags";
  import type { EntityOut } from "~~/lib/api/types/data-contracts";

  definePageMeta({
    middleware: ["auth"],
  });

  const { openDialog, closeDialog } = useDialog();
  const route = useRoute();
  const api = useUserApi();
  const confirm = useConfirm();

  const tagStore = useTagStore();
  const tags = computed(() => tagStore.tags);

  const templateId = computed<string>(() => route.params.id as string);

  const { t } = useI18n();
  const exportLabel = (id: string) => t(administrationLabelKeys[id] ?? `global.${id}`);

  const { data: template, refresh } = useAsyncData(templateId.value, async () => {
    const { data, error } = await api.templates.get(templateId.value);
    if (error) {
      toast.error(t("components.template.toast.load_failed"));
      navigateTo("/templates");
      return;
    }
    return data;
  });

  async function confirmDelete() {
    const { isCanceled } = await confirm.open(t("components.template.confirm_delete"));
    if (isCanceled) return;

    const { error } = await api.templates.delete(templateId.value);
    if (error) {
      toast.error(t("components.template.toast.delete_failed"));
      return;
    }
    toast.success(t("components.template.toast.deleted"));
    navigateTo("/templates");
  }

  const updating = ref(false);
  const updateData = reactive({
    id: "",
    name: "",
    description: "",
    notes: "",
    defaultName: "",
    defaultDescription: "",
    defaultQuantity: 1,
    defaultInsured: false,
    defaultManufacturer: "",
    defaultModelNumber: "",
    defaultLifetimeWarranty: false,
    defaultWarrantyDetails: "",
    defaultLocation: null as EntityOut | null,
    defaultTagIds: [] as string[],
    includeWarrantyFields: false,
    includePurchaseFields: false,
    includeSoldFields: false,
    fields: [] as Array<{ id: string; name: string; type: "text"; textValue: string }>,
  });

  function openUpdate() {
    if (!template.value) return;
    Object.assign(updateData, {
      id: template.value.id,
      name: template.value.name,
      description: template.value.description,
      notes: template.value.notes,
      defaultName: template.value.defaultName ?? "",
      defaultDescription: template.value.defaultDescription ?? "",
      defaultQuantity: template.value.defaultQuantity,
      defaultInsured: template.value.defaultInsured,
      defaultManufacturer: template.value.defaultManufacturer,
      defaultModelNumber: template.value.defaultModelNumber ?? "",
      defaultLifetimeWarranty: template.value.defaultLifetimeWarranty,
      defaultWarrantyDetails: template.value.defaultWarrantyDetails,
      defaultLocation: template.value.defaultLocation ?? null,
      defaultTagIds: template.value.defaultTags?.map(l => l.id) ?? [],
      includeWarrantyFields: template.value.includeWarrantyFields,
      includePurchaseFields: template.value.includePurchaseFields,
      includeSoldFields: template.value.includeSoldFields,
      fields: template.value.fields.map(f => ({
        id: f.id,
        name: f.name,
        type: "text" as const,
        textValue: f.textValue,
      })),
    });
    openDialog(DialogID.UpdateTemplate);
  }

  async function update() {
    if (updating.value) return;
    updating.value = true;

    // Prepare the data with proper format for API
    const payload = {
      ...updateData,
      defaultLocationId: updateData.defaultLocation?.id ?? null,
    };

    const { error, data } = await api.templates.update(templateId.value, payload);
    if (error) {
      updating.value = false;
      toast.error(t("components.template.toast.update_failed"));
      return;
    }
    toast.success(t("components.template.toast.updated"));
    template.value = data;
    closeDialog(DialogID.UpdateTemplate);
    updating.value = false;
    refresh();
  }

  const NIL_UUID = "00000000-0000-0000-0000-000000000000";
</script>

<template>
  <Dialog :dialog-id="DialogID.UpdateTemplate">
    <DialogContent class="max-h-[90dvh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>{{ $t("components.template.edit_modal.title") }}</DialogTitle>
      </DialogHeader>

      <form v-if="template" class="flex min-w-0 flex-col gap-4" :aria-busy="updating" @submit.prevent="update">
        <FormTextField
          v-model="updateData.name"
          :autofocus="true"
          :label="$t('components.template.form.template_name')"
          :max-length="255"
        />
        <FormTextArea
          v-model="updateData.description"
          :label="$t('components.template.form.template_description')"
          :max-length="1000"
        />

        <Separator class="my-2" />
        <h3 class="text-sm font-medium">{{ $t("components.template.form.default_item_values") }}</h3>
        <div class="grid gap-2">
          <FormTextField
            v-model="updateData.defaultName"
            :label="$t('components.template.form.item_name')"
            :max-length="255"
          />
          <FormTextArea
            v-model="updateData.defaultDescription"
            :label="$t('components.template.form.item_description')"
            :max-length="1000"
          />
          <div class="grid gap-4 sm:grid-cols-2">
            <FormTextField
              v-model.number="updateData.defaultQuantity"
              :label="$t('global.quantity')"
              type="number"
              :min="1"
              step="any"
            />
            <FormTextField
              v-model="updateData.defaultModelNumber"
              :label="$t('components.template.form.model_number')"
              :max-length="255"
            />
          </div>
          <FormTextField
            v-model="updateData.defaultManufacturer"
            :label="$t('components.template.form.manufacturer')"
            :max-length="255"
          />
          <LocationSelector
            v-model="updateData.defaultLocation"
            :label="$t('components.template.form.default_location')"
          />
          <TagSelector v-model="updateData.defaultTagIds" :tags="tags ?? []" />
          <div class="flex flex-wrap items-start gap-4">
            <div class="flex items-center gap-2">
              <Switch id="editInsured" v-model="updateData.defaultInsured" />
              <Label for="editInsured" class="text-sm">{{ $t("global.insured") }}</Label>
            </div>
            <div class="flex items-center gap-2">
              <Switch id="editWarranty" v-model="updateData.defaultLifetimeWarranty" />
              <Label for="editWarranty" class="text-sm">{{ $t("components.template.form.lifetime_warranty") }}</Label>
            </div>
          </div>
        </div>

        <Separator class="my-2" />
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-medium">{{ $t("components.template.form.custom_fields") }}</h3>
          <Button
            type="button"
            size="sm"
            variant="outline"
            @click="updateData.fields.push({ id: NIL_UUID, name: '', type: 'text', textValue: '' })"
          >
            <MdiPlus class="mr-1 size-4" />
            {{ $t("global.add") }}
          </Button>
        </div>
        <div v-if="updateData.fields.length > 0" class="flex flex-col gap-2">
          <div
            v-for="(field, idx) in updateData.fields"
            :key="idx"
            class="hb-panel flex min-w-0 flex-wrap items-end gap-3 p-3"
          >
            <FormTextField
              v-model="field.name"
              :label="$t('components.template.form.field_name')"
              :max-length="255"
              class="min-w-0 basis-full sm:flex-1 sm:basis-0"
            />
            <FormTextField
              v-model="field.textValue"
              :label="$t('components.template.form.default_value')"
              class="min-w-0 basis-full sm:flex-1 sm:basis-0"
            />
            <Button
              type="button"
              size="icon"
              variant="ghost"
              :aria-label="$t('global.delete')"
              @click="updateData.fields.splice(idx, 1)"
            >
              <MdiDelete class="size-4" />
            </Button>
          </div>
        </div>

        <DialogFooter>
          <Button type="submit" :disabled="updating">
            {{ updating ? $t("global.loading") : $t("global.update") }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>

  <BaseContainer v-if="template">
    <Title>{{ template.name }}</Title>

    <Card class="hb-panel min-w-0 p-4 sm:p-6">
      <header :class="{ 'mb-2': template.description }">
        <div class="flex flex-wrap items-end gap-2">
          <div class="min-w-0 flex-1 basis-64">
            <h1 class="break-words pb-2 font-editorial text-3xl leading-tight tracking-tight sm:text-4xl">
              {{ template.name }}
            </h1>
            <div class="flex flex-wrap gap-1 text-xs text-muted-foreground">
              <span>{{ $t("global.created") }} <DateTime :date="template.createdAt" /></span>
              <span>•</span>
              <span>{{ $t("components.template.detail.updated") }} <DateTime :date="template.updatedAt" /></span>
            </div>
          </div>
          <div class="ml-auto flex flex-wrap gap-2">
            <Button @click="openUpdate">
              <MdiPencil class="mr-1" />
              {{ $t("global.edit") }}
            </Button>
            <Button variant="destructive" @click="confirmDelete">
              <MdiDelete class="mr-1" />
              {{ $t("global.delete") }}
            </Button>
          </div>
        </div>
      </header>

      <Separator v-if="template.description" class="my-3" />
      <div v-if="template.description" class="min-w-0 break-words">
        <Markdown :source="template.description" />
      </div>

      <Separator class="my-3" />
      <div class="grid gap-4 text-sm md:grid-cols-2">
        <div>
          <h3 class="mb-2 font-medium">{{ $t("components.template.detail.default_values") }}</h3>
          <dl class="flex flex-col gap-1">
            <div v-if="template.defaultName" class="grid min-w-0 grid-cols-2 gap-3 py-1">
              <dt class="min-w-0 break-words text-muted-foreground">{{ $t("components.template.form.item_name") }}</dt>
              <dd class="min-w-0 break-words text-right">{{ template.defaultName }}</dd>
            </div>
            <div v-if="template.defaultDescription" class="grid min-w-0 grid-cols-2 gap-3 py-1">
              <dt class="min-w-0 break-words text-muted-foreground">
                {{ $t("components.template.form.item_description") }}
              </dt>
              <dd class="min-w-0 break-words text-right">{{ template.defaultDescription }}</dd>
            </div>
            <div class="grid min-w-0 grid-cols-2 gap-3 py-1">
              <dt class="min-w-0 break-words text-muted-foreground">{{ $t("global.quantity") }}</dt>
              <dd class="min-w-0 break-words text-right">{{ template.defaultQuantity }}</dd>
            </div>
            <div v-if="template.defaultModelNumber" class="grid min-w-0 grid-cols-2 gap-3 py-1">
              <dt class="min-w-0 break-words text-muted-foreground">
                {{ $t("components.template.form.model_number") }}
              </dt>
              <dd class="min-w-0 break-words text-right">{{ template.defaultModelNumber }}</dd>
            </div>
            <div v-if="template.defaultManufacturer" class="grid min-w-0 grid-cols-2 gap-3 py-1">
              <dt class="min-w-0 break-words text-muted-foreground">
                {{ $t("components.template.form.manufacturer") }}
              </dt>
              <dd class="min-w-0 break-words text-right">{{ template.defaultManufacturer }}</dd>
            </div>
            <div v-if="template.defaultLocation" class="grid min-w-0 grid-cols-2 gap-3 py-1">
              <dt class="min-w-0 break-words text-muted-foreground">{{ $t("components.template.form.location") }}</dt>
              <dd class="min-w-0 break-words text-right">{{ template.defaultLocation.name }}</dd>
            </div>
            <div
              v-if="template.defaultTags && template.defaultTags.length > 0"
              class="grid min-w-0 grid-cols-2 gap-3 py-1"
            >
              <dt class="min-w-0 break-words text-muted-foreground">{{ $t("global.tags") }}</dt>
              <dd class="min-w-0 break-words text-right">{{ template.defaultTags.map(t => t.name).join(", ") }}</dd>
            </div>
            <div class="grid min-w-0 grid-cols-2 gap-3 py-1">
              <dt class="min-w-0 break-words text-muted-foreground">{{ $t("global.insured") }}</dt>
              <dd class="min-w-0 break-words text-right">
                {{ template.defaultInsured ? $t("global.yes") : $t("global.no") }}
              </dd>
            </div>
            <div class="grid min-w-0 grid-cols-2 gap-3 py-1">
              <dt class="min-w-0 break-words text-muted-foreground">
                {{ $t("components.template.form.lifetime_warranty") }}
              </dt>
              <dd class="min-w-0 break-words text-right">
                {{ template.defaultLifetimeWarranty ? $t("global.yes") : $t("global.no") }}
              </dd>
            </div>
          </dl>
        </div>
        <div>
          <h3 class="mb-2 font-medium">{{ $t("components.template.form.custom_fields") }}</h3>
          <RecordListExport :request="templateFieldsExport(templateId, exportLabel)" />
          <dl class="flex flex-col gap-1">
            <div v-for="field in template.fields" :key="field.id" class="grid min-w-0 grid-cols-2 gap-3 py-1">
              <dt class="min-w-0 break-words text-muted-foreground">{{ field.name }}</dt>
              <dd class="min-w-0 break-words text-right">{{ field.textValue || "—" }}</dd>
            </div>
          </dl>
        </div>
      </div>
    </Card>
  </BaseContainer>
</template>
