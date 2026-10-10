<template>
  <BaseModal
    keyboard-focus
    :dialog-id="DialogID.JoinCollection"
    :title="$t('components.collection.join_modal.title')"
    hide-footer
  >
    <form class="flex min-w-0 flex-col gap-4" @submit.prevent="join()">
      <FormTextField
        v-model="form.inviteCode"
        :trigger-focus="focused"
        :autofocus="true"
        :required="true"
        :label="$t('components.collection.join_modal.invite_code_label')"
      />

      <div class="mt-2 flex flex-col">
        <span class="text-sm text-muted-foreground">
          {{ $t("components.collection.join_modal.invites_should_look_like") }}
        </span>
        <div class="mt-2 flex min-w-0 flex-col items-start gap-2">
          <Badge variant="outline" class="max-w-full break-all font-mono"> AYQ4W4K5MT4CZOPB2PZRCZ4PTY </Badge>
          <Badge variant="outline" class="max-w-full break-all font-mono">
            {{ `${domain}?token=AYQ4W4K5MT4CZOPB2PZRCZ4PTY` }}
          </Badge>
        </div>
      </div>

      <Button
        type="button"
        class="mt-2 h-auto min-h-10 whitespace-normal text-start"
        variant="outline"
        @click="openDialog(DialogID.CreateCollection)"
      >
        {{ $t("components.collection.join_modal.no_invite_code_create_one") }}
      </Button>

      <div class="mt-4 flex min-w-0 flex-wrap justify-end gap-2 border-t pt-4">
        <Button :disabled="loading" class="h-auto min-h-10 max-w-full whitespace-normal" type="submit">{{
          $t("components.collection.join_modal.join")
        }}</Button>
      </div>
      <p
        v-if="errorMessage"
        role="alert"
        class="break-words rounded-md bg-destructive p-3 text-sm text-destructive-foreground"
      >
        {{ errorMessage }}
      </p>
    </form>
  </BaseModal>
</template>

<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { DialogID } from "@/components/ui/dialog-provider/utils";
  import { toast } from "@/components/ui/sonner";
  import BaseModal from "@/components/App/CreateModal.vue";
  import { useDialog } from "~/components/ui/dialog-provider";
  import FormTextField from "~/components/Form/TextField.vue";
  import { Button } from "~/components/ui/button";
  import { Badge } from "~/components/ui/badge";
  import { useUserApi } from "~/composables/use-api";
  import { useCollections } from "~/composables/use-collections";

  const { t } = useI18n();

  const { activeDialog, closeDialog, openDialog, registerOpenDialogCallback } = useDialog();

  const loading = ref(false);
  const errorMessage = ref("");

  function reportError(message: string) {
    errorMessage.value = message;
    toast.error(message);
  }
  const focused = ref(false);

  const form = reactive({ inviteCode: "" });

  const api = useUserApi();
  const collections = useCollections();

  const domain = window.location.protocol + "//" + window.location.host;

  const redirectTo = ref("");
  // Holds invite code from dialog params until the watcher applies it on open
  const pendingInviteCode = ref("");

  onMounted(() => {
    const cleanup = registerOpenDialogCallback(DialogID.JoinCollection, params => {
      redirectTo.value = params?.redirectTo || "";
      pendingInviteCode.value = params?.inviteCode || "";
    });

    onUnmounted(cleanup);
  });

  watch(
    () => activeDialog.value,
    active => {
      if (active && active === DialogID.JoinCollection) {
        form.inviteCode = pendingInviteCode.value;
        pendingInviteCode.value = "";
        errorMessage.value = "";
        focused.value = true;
      }
    }
  );

  async function join() {
    if (loading.value) {
      reportError(t("components.collection.join_modal.toast.already_joining"));
      return;
    }

    if (!form.inviteCode || form.inviteCode.trim().length === 0) {
      reportError(t("components.collection.join_modal.toast.please_enter_valid_join_code"));
      return;
    }

    // remove everything before the first '?' character
    const code = form.inviteCode.includes("?")
      ? form.inviteCode.split("?")[1]?.replace("token=", "").trim()
      : form.inviteCode.trim();

    if (!code || code.length !== 26) {
      reportError(t("components.collection.join_modal.toast.please_enter_valid_join_code"));
      return;
    }

    errorMessage.value = "";
    loading.value = true;

    const { data, error } = await api.group.acceptInvitation(code);

    if (error) {
      loading.value = false;
      reportError(t("components.collection.join_modal.toast.join_failed"));
      return;
    }

    if (data) {
      toast.success(t("components.collection.join_modal.toast.join_success"));
    }

    form.inviteCode = "";
    loading.value = false;

    closeDialog(DialogID.JoinCollection);
    if (data) {
      const joinedId = data.id;
      collections.set(joinedId);
      if (redirectTo.value === "") {
        // reload page to reflect joined collection
        window.location.reload();
      } else {
        window.location.href = redirectTo.value;
      }
    }
  }
</script>
