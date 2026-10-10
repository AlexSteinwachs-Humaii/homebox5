<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { toast } from "@/components/ui/sonner";
  import { Button } from "@/components/ui/button";
  import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
  import { useDialog } from "@/components/ui/dialog-provider";
  import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
  import { badgeVariants } from "@/components/ui/badge";
  import FormCheckbox from "~/components/Form/Checkbox.vue";
  import FormTextField from "~/components/Form/TextField.vue";
  import DateTime from "@/components/global/DateTime.vue";
  import MdiMegaphone from "~icons/mdi/megaphone";
  import MdiDelete from "~icons/mdi/delete";
  import MdiPencil from "~icons/mdi/pencil";
  import { DialogID } from "~/components/ui/dialog-provider/utils";
  import type { NotifierCreate, NotifierOut } from "~~/lib/api/types/data-contracts";

  definePageMeta({
    middleware: ["auth"],
  });

  const { t } = useI18n();

  useHead({ title: `HomeBox | ${t("collection.tabs.notifiers")}` });

  const api = useUserApi();
  const confirm = useConfirm();
  const { openDialog, closeDialog } = useDialog();

  const notifiers = useAsyncData(async () => {
    const { data, error } = await api.notifiers.getAll();
    if (error) throw new Error(t("errors.api_failure") + String(error));
    return data ?? [];
  });

  const saving = ref(false);
  const testing = ref(false);
  const formError = ref<string | null>(null);
  const deleting = ref<Record<string, boolean>>({});
  const targetID = ref("");
  const notifier = ref<NotifierCreate | null>(null);

  const openNotifierDialog = (v: NotifierOut | null) => {
    formError.value = null;
    if (v) {
      targetID.value = v.id;
      notifier.value = {
        name: v.name,
        url: v.url,
        isActive: v.isActive,
      };
    } else {
      targetID.value = "";
      notifier.value = {
        name: "",
        url: "",
        isActive: true,
      };
    }

    openDialog(DialogID.CreateNotifier);
  };

  const createNotifier = async () => {
    if (!notifier.value || saving.value || testing.value) return;

    if (targetID.value) {
      await editNotifier();
      return;
    }

    await saveNotifier(
      () =>
        api.notifiers.create({
          name: notifier.value!.name,
          url: notifier.value!.url || "",
          isActive: notifier.value!.isActive,
        }),
      "profile.toast.failed_create_notifier"
    );
  };

  const editNotifier = async () => {
    if (!notifier.value) return;

    await saveNotifier(
      () =>
        api.notifiers.update(targetID.value, {
          name: notifier.value!.name,
          url: notifier.value!.url || "",
          isActive: notifier.value!.isActive,
        }),
      "profile.toast.failed_update_notifier"
    );
  };

  const saveNotifier = async (request: () => Promise<{ error?: unknown }>, failureKey: string) => {
    saving.value = true;
    formError.value = null;
    try {
      const result = await request();
      if (result.error) throw result.error;
      notifier.value = null;
      targetID.value = "";
      closeDialog(DialogID.CreateNotifier);
      await notifiers.refresh();
    } catch {
      formError.value = t(failureKey);
      toast.error(formError.value);
    } finally {
      saving.value = false;
    }
  };

  const deleteNotifier = async (id: string) => {
    if (deleting.value[id]) return;
    const result = await confirm.open(t("profile.delete_notifier_confirm"));

    if (result.isCanceled) return;

    deleting.value[id] = true;
    try {
      const { error } = await api.notifiers.delete(id);
      if (error) throw error;
      await notifiers.refresh();
    } catch {
      toast.error(t("profile.toast.failed_delete_notifier"));
    } finally {
      deleting.value[id] = false;
    }
  };

  const testNotifier = async () => {
    if (!notifier.value || testing.value || saving.value) return;

    testing.value = true;
    formError.value = null;
    try {
      const { error } = await api.notifiers.test(notifier.value.url);
      if (error) throw error;
      toast.success(t("profile.toast.notifier_test_success"));
    } catch {
      formError.value = t("profile.toast.failed_test_notifier");
      toast.error(formError.value);
    } finally {
      testing.value = false;
    }
  };
</script>

<template>
  <div class="min-w-0 space-y-4">
    <Dialog :dialog-id="DialogID.CreateNotifier">
      <DialogContent class="hb-panel">
        <DialogHeader>
          <DialogTitle>{{ $t("profile.notifier_modal", { type: Boolean(targetID) }) }}</DialogTitle>
        </DialogHeader>

        <form class="min-w-0 space-y-4" :aria-busy="saving || testing" @submit.prevent="createNotifier">
          <template v-if="notifier">
            <FormTextField v-model="notifier.name" :label="$t('global.name')" class="mb-2" />
            <FormTextField v-model="notifier.url" :label="$t('profile.url')" class="mb-2" />
            <div>
              <FormCheckbox v-model="notifier.isActive" :label="$t('profile.enabled')" />
            </div>
          </template>
          <p v-if="formError" role="alert" class="break-words text-sm text-destructive">
            {{ formError }}
          </p>
          <div class="mt-4 flex flex-wrap gap-2">
            <DialogFooter class="w-full gap-2">
              <Button
                variant="secondary"
                :disabled="!(notifier && notifier.url) || saving || testing"
                type="button"
                @click="testNotifier"
              >
                {{ $t("profile.test") }}
              </Button>
              <div class="grow" />
              <Button type="submit" :disabled="saving || testing">{{ $t("global.submit") }}</Button>
            </DialogFooter>
          </div>
        </form>
      </DialogContent>
    </Dialog>

    <div class="hb-panel min-w-0 p-4 sm:p-6">
      <header class="mb-2 flex items-center gap-2">
        <MdiMegaphone class="-mt-1" />
        <div>
          <h2 class="hb-section-title">{{ $t("profile.notifiers") }}</h2>
          <p class="text-sm text-muted-foreground">
            {{ $t("profile.notifiers_sub") }}
          </p>
        </div>
      </header>

      <p v-if="notifiers.status.value === 'pending'" role="status" class="py-4 text-sm text-muted-foreground">
        {{ $t("global.loading") }}
      </p>
      <p v-else-if="notifiers.error.value" role="alert" class="break-words py-4 text-sm text-destructive">
        {{ $t("errors.api_failure") }}
      </p>
      <div v-else-if="notifiers.data.value" class="mt-4 divide-y rounded-lg border">
        <p v-if="notifiers.data.value.length === 0" role="status" class="p-4 text-sm text-muted-foreground">
          {{ $t("profile.no_notifiers") }}
        </p>
        <article v-for="n in notifiers.data.value" v-else :key="n.id" class="p-4">
          <div class="flex flex-wrap items-center gap-2">
            <p class="mr-auto min-w-0 break-words font-medium [overflow-wrap:anywhere]">
              {{ n.name }}
            </p>
            <TooltipProvider :delay-duration="0" class="flex justify-end gap-2">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button
                    variant="destructive"
                    size="icon"
                    :aria-label="$t('global.delete')"
                    :disabled="deleting[n.id]"
                    @click="deleteNotifier(n.id)"
                  >
                    <MdiDelete />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{{ $t("global.delete") }}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button variant="outline" size="icon" :aria-label="$t('global.edit')" @click="openNotifierDialog(n)">
                    <MdiPencil />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{{ $t("global.edit") }}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          <div class="flex flex-wrap justify-between gap-2 py-2 text-sm">
            <p>
              <span v-if="n.isActive" :class="badgeVariants()">{{ $t("profile.active") }}</span>
              <span v-else :class="badgeVariants({ variant: 'destructive' })">{{ $t("profile.inactive") }}</span>
            </p>
            <p>
              {{ $t("global.created") }}
              <DateTime format="relative" datetime-type="time" :date="n.createdAt" />
            </p>
          </div>
        </article>
      </div>

      <div class="mt-4">
        <Button
          class="h-auto min-h-10 whitespace-normal"
          :disabled="notifiers.status.value === 'pending'"
          @click="openNotifierDialog(null)"
        >
          {{ $t("global.create") }}
        </Button>
      </div>
    </div>
  </div>
</template>
