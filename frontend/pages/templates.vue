<script setup lang="ts">
  import RecordListExport from "~/components/Export/RecordListExport.vue";
  import { administrationExport, administrationLabelKeys } from "~/lib/exports/administration";
  import { useI18n } from "vue-i18n";
  import { toast } from "@/components/ui/sonner";
  import MdiPlus from "~icons/mdi/plus";
  import { Button } from "@/components/ui/button";
  import { useDialog } from "@/components/ui/dialog-provider";
  import { DialogID } from "~/components/ui/dialog-provider/utils";
  import BaseContainer from "@/components/Base/Container.vue";
  import BaseSectionHeader from "@/components/Base/SectionHeader.vue";
  import TemplateCard from "~/components/Template/Card.vue";
  import TemplateCreateModal from "~/components/Template/CreateModal.vue";

  definePageMeta({
    middleware: ["auth"],
  });

  const { t } = useI18n();
  const exportLabel = (id: string) => t(administrationLabelKeys[id] ?? `global.${id}`);

  useHead({
    title: computed(() => `HomeBox | ${t("pages.templates.title")}`),
  });

  const api = useUserApi();
  const { openDialog } = useDialog();

  const loadFailed = ref(false);
  const {
    data: templates,
    refresh,
    status,
  } = useAsyncData("templates", async () => {
    const { data, error } = await api.templates.getAll();
    loadFailed.value = !!error;
    if (error) {
      toast.error(t("components.template.toast.load_failed"));
      return [];
    }
    return data;
  });

  const pending = computed(() => status.value === "pending");

  // Wrapper functions to match event signatures
  const handleRefresh = () => refresh();
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const handleDuplicated = (_id: string) => refresh();
</script>
<template>
  <BaseContainer>
    <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
      <BaseSectionHeader>{{ $t("pages.templates.title") }}</BaseSectionHeader>
      <Button @click="openDialog(DialogID.CreateTemplate)">
        <MdiPlus class="mr-2" />
        {{ $t("global.create") }}
      </Button>
    </div>

    <RecordListExport :request="administrationExport('templates', exportLabel)" />
    <TemplateCreateModal @created="handleRefresh" />

    <p v-if="pending" role="status" class="hb-panel p-6 text-center text-muted-foreground">
      {{ $t("global.loading") }}
    </p>
    <div v-else-if="loadFailed" role="alert" class="hb-panel flex flex-col items-center gap-4 p-6 text-center">
      <p class="rounded-md border border-destructive px-4 py-3 text-card-foreground">
        {{ $t("components.template.toast.load_failed") }}
      </p>
      <Button variant="outline" @click="handleRefresh">{{ $t("global.retry") }}</Button>
    </div>
    <div v-else-if="templates && templates.length > 0" class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      <TemplateCard
        v-for="tpl in templates"
        :key="tpl.id"
        :template="tpl"
        @deleted="handleRefresh"
        @duplicated="handleDuplicated"
      />
    </div>

    <div v-else role="status" class="hb-panel flex flex-col items-center justify-center p-6 py-12 text-center">
      <p class="mb-4 text-muted-foreground">{{ $t("pages.templates.no_templates") }}</p>
      <Button @click="openDialog(DialogID.CreateTemplate)">
        <MdiPlus class="mr-2" />
        {{ $t("components.template.create_modal.title") }}
      </Button>
    </div>
  </BaseContainer>
</template>
