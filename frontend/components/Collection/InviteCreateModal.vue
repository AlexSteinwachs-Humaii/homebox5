<template>
  <BaseModal
    keyboard-focus
    :dialog-id="DialogID.CreateGroupInvite"
    :title="$t('collection.create_invite')"
    :hide-footer="true"
  >
    <form class="flex min-w-0 flex-col gap-4" @submit.prevent="create">
      <FormTextField v-model="form.uses" :label="$t('collection.uses')" type="number" :required="true" />

      <div class="flex w-full flex-col gap-1.5">
        <Label for="dp-input-collection-invite-expiry" class="break-words">{{ $t("collection.expires_at") }}</Label>
        <VueDatePicker
          v-model="form.expiresAt"
          uid="collection-invite-expiry"
          :aria-labels="{ input: $t('collection.expires_at') }"
          :enable-time-picker="true"
          clearable
          :dark="isDark"
          :format="formatDateTime"
        />
      </div>

      <div class="mt-4 flex min-w-0 flex-wrap justify-end gap-2 border-t pt-4">
        <div class="flex min-w-0 flex-wrap justify-end gap-2">
          <Button :disabled="loading" class="h-auto min-h-10 max-w-full whitespace-normal" type="submit">
            {{ $t("global.create") }}
          </Button>
        </div>
      </div>
      <p v-if="errorMessage" role="alert" class="break-words text-sm text-destructive">
        {{ errorMessage }}
      </p>
    </form>
  </BaseModal>
</template>

<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import VueDatePicker from "@vuepic/vue-datepicker";
  import "@vuepic/vue-datepicker/dist/main.css";
  import { DialogID } from "@/components/ui/dialog-provider/utils";
  import { useDialog } from "~/components/ui/dialog-provider";
  import BaseModal from "@/components/App/CreateModal.vue";
  import FormTextField from "~/components/Form/TextField.vue";
  import { Button } from "~/components/ui/button";
  import { Label } from "~/components/ui/label";
  import { toast } from "@/components/ui/sonner";
  import { useUserApi } from "~/composables/use-api";
  import { darkThemes } from "~/lib/data/themes";

  const { t } = useI18n();
  const { activeDialog, closeDialog } = useDialog();
  const api = useUserApi();

  const loading = ref(false);
  const errorMessage = ref("");

  function reportError(message: string) {
    errorMessage.value = message;
    toast.error(message);
  }
  const form = reactive<{ uses: number; expiresAt: Date | null }>({
    uses: 1,
    expiresAt: defaultExpiry(),
  });

  const isDark = useIsThemeInList(darkThemes);

  const formatDateTime = (date: Date | string | number) => fmtDate(date, "human", "datetime");

  function defaultExpiry(): Date {
    return new Date(Date.now() + 7 * 24 * 60 * 60 * 1000);
  }

  watch(
    () => activeDialog.value,
    active => {
      if (active && active === DialogID.CreateGroupInvite) {
        form.uses = 1;
        form.expiresAt = defaultExpiry();
        errorMessage.value = "";
        loading.value = false;
      }
    }
  );

  async function create() {
    if (loading.value) {
      return;
    }

    const parsedUses = Number(form.uses ?? 0);
    if (!Number.isFinite(parsedUses) || parsedUses < 1 || parsedUses > 100) {
      reportError(t("components.collection.invite_create_modal.toast.invalid_uses"));
      return;
    }
    const uses = parsedUses;

    if (!form.expiresAt) {
      reportError(t("components.collection.invite_create_modal.toast.invalid_expiry_missing"));
      return;
    }

    const now = new Date();
    const exp = new Date(form.expiresAt);
    if (exp.getTime() <= now.getTime()) {
      reportError(t("components.collection.invite_create_modal.toast.invalid_expiry_past"));
      return;
    }

    const expiresAtToSend: Date = exp;

    errorMessage.value = "";
    loading.value = true;

    try {
      const res = await api.group.createInvitation({
        expiresAt: expiresAtToSend,
        uses,
      });

      if (res.error) {
        const msg = t("errors.api_failure") + String(res.error);
        reportError(msg);
        loading.value = false;
        return;
      }

      const data = res.data ?? undefined;
      closeDialog(DialogID.CreateGroupInvite, data);
    } catch (e) {
      const msg = (e as Error).message ?? String(e);
      reportError(msg);
    } finally {
      loading.value = false;
    }
  }
</script>
