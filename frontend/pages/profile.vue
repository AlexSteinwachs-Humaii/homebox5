<script setup lang="ts">
  import HistoryListExport from "~/components/Export/HistoryListExport.vue";
  import RecordListExport from "~/components/Export/RecordListExport.vue";
  import { administrationExport, administrationLabelKeys } from "~/lib/exports/administration";
  import { useI18n } from "vue-i18n";
  import { toast } from "@/components/ui/sonner";
  import type { Detail } from "~~/components/global/DetailsSection/types";
  import MdiLoading from "~icons/mdi/loading";
  import MdiAccount from "~icons/mdi/account";
  import MdiDelete from "~icons/mdi/delete";
  import MdiFill from "~icons/mdi/fill";
  import MdiKeyVariant from "~icons/mdi/key-variant";
  import MdiContentCopy from "~icons/mdi/content-copy";
  import { Button } from "@/components/ui/button";
  import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
  import { useDialog } from "@/components/ui/dialog-provider";
  import LanguageSelector from "~/components/App/LanguageSelector.vue";
  import { DialogID } from "~/components/ui/dialog-provider/utils";
  import ThemePicker from "~/components/App/ThemePicker.vue";
  import ItemDuplicateSettings from "~/components/Item/DuplicateSettings.vue";
  import FormPassword from "~/components/Form/Password.vue";
  import FormTextField from "~/components/Form/TextField.vue";
  import FormCheckbox from "~/components/Form/Checkbox.vue";
  import BaseContainer from "@/components/Base/Container.vue";
  import BaseCard from "@/components/Base/Card.vue";
  import BaseSectionHeader from "@/components/Base/SectionHeader.vue";
  import DetailsSection from "@/components/global/DetailsSection/DetailsSection.vue";
  import DateTime from "@/components/global/DateTime.vue";
  import PasswordScore from "~/components/global/PasswordScore.vue";
  import { PASSWORD_MIN_LENGTH, PASSWORD_RULES } from "~/lib/passwords";
  import type { APIKeyOut } from "~~/lib/api/types/data-contracts";

  const { t } = useI18n();
  const exportLabel = (id: string) => t(administrationLabelKeys[id] ?? `global.${id}`);

  definePageMeta({
    middleware: ["auth"],
  });
  useHead({
    title: "HomeBox | " + t("menu.profile"),
  });

  const api = useUserApi();
  const confirm = useConfirm();

  const { openDialog, closeDialog, activeDialog } = useDialog();

  const preferences = useViewPreferences();
  function setDisplayHeader() {
    preferences.value.displayLegacyHeader = !preferences.value.displayLegacyHeader;
  }
  function setLegacyImageFit() {
    preferences.value.legacyImageFit = !preferences.value.legacyImageFit;
  }

  const auth = useAuthContext();

  const details = computed(() => {
    return [
      {
        name: "global.name",
        text: auth.user?.name || t("global.unknown"),
      },
      {
        name: "global.email",
        text: auth.user?.email || t("global.unknown"),
      },
    ] as Detail[];
  });

  async function deleteProfile() {
    const result = await confirm.open(t("profile.delete_account_confirm"));

    if (result.isCanceled) {
      return;
    }

    const { response } = await api.user.delete();

    if (response?.status === 204) {
      toast.success(t("profile.toast.account_deleted"));
      auth.logout(api);
      navigateTo("/");
      return;
    }

    toast.error(t("profile.toast.failed_delete_account"));
  }

  const passwordChange = reactive({
    loading: false,
    current: "",
    new: "",
    isValid: false,
  });

  async function changePassword() {
    passwordError.value = "";
    passwordChange.loading = true;
    if (!passwordChange.isValid) {
      passwordChange.loading = false;
      return;
    }

    const { error } = await api.user.changePassword(passwordChange.current, passwordChange.new);

    if (error) {
      passwordError.value = t("profile.toast.failed_change_password");
      toast.error(passwordError.value);
      passwordChange.loading = false;
      return;
    }

    toast.success(t("profile.toast.password_changed"));
    closeDialog(DialogID.ChangePassword);
    passwordChange.new = "";
    passwordChange.current = "";
    passwordChange.loading = false;
  }

  async function logoutAllDevices() {
    const result = await confirm.open(t("profile.logout_all_confirm"));
    if (result.isCanceled) {
      return;
    }

    const { error } = await api.user.logoutAll();
    if (error) {
      toast.error(t("profile.toast.failed_logout_all"));
      return;
    }

    // Every session token was revoked server-side, including this one, so clear
    // the local session and send the user back to the login flow.
    toast.success(t("profile.toast.logout_all_success"));
    auth.invalidateSession();
    navigateTo("/");
  }

  // ---------------------------------------------------------------------------
  // API keys

  const apiKeys = ref<APIKeyOut[]>([]);
  const apiKeysLoading = ref(false);
  const apiKeysError = ref("");
  const apiKeyFormError = ref("");
  const passwordError = ref("");

  async function loadApiKeys() {
    apiKeysError.value = "";
    apiKeysLoading.value = true;
    const { data, error } = await api.user.listApiKeys();
    apiKeysLoading.value = false;
    if (error) {
      apiKeysError.value = t("errors.api_failure");
      toast.error(apiKeysError.value);
      return;
    }
    apiKeys.value = data ?? [];
  }

  onMounted(() => {
    void loadApiKeys();
  });

  const apiKeyForm = reactive({
    name: "",
    setExpiration: false,
    expiresAt: "",
    submitting: false,
  });

  function openCreateApiKey() {
    apiKeyFormError.value = "";
    apiKeyForm.name = "";
    apiKeyForm.setExpiration = false;
    apiKeyForm.expiresAt = "";
    apiKeyForm.submitting = false;
    openDialog(DialogID.CreateApiKey);
  }

  const newApiKeyToken = ref<string | null>(null);

  // Escape, overlay and close-icon dismissal must clear the one-time secret too.
  watch(activeDialog, dialog => {
    if (dialog !== DialogID.CreateApiKeyResult) newApiKeyToken.value = null;
  });

  async function submitCreateApiKey() {
    if (apiKeyForm.submitting || !apiKeyForm.name.trim()) return;
    apiKeyFormError.value = "";
    apiKeyForm.submitting = true;

    let expiresAt: string | null = null;
    if (apiKeyForm.setExpiration && apiKeyForm.expiresAt) {
      // <input type="date"> gives YYYY-MM-DD; treat as end-of-day UTC so the
      // key remains valid through the chosen calendar day.
      expiresAt = new Date(apiKeyForm.expiresAt + "T23:59:59Z").toISOString();
    }

    const { data, error } = await api.user.createApiKey({
      name: apiKeyForm.name.trim(),
      expiresAt,
    });

    apiKeyForm.submitting = false;

    if (error || !data) {
      apiKeyFormError.value = t("profile.toast.failed_api_key_create");
      toast.error(apiKeyFormError.value);
      return;
    }

    closeDialog(DialogID.CreateApiKey);
    newApiKeyToken.value = data.token;
    openDialog(DialogID.CreateApiKeyResult);
    toast.success(t("profile.toast.api_key_created"));
    await loadApiKeys();
  }

  async function copyApiKey() {
    if (!newApiKeyToken.value) return;
    try {
      await navigator.clipboard.writeText(newApiKeyToken.value);
      toast.success(t("profile.api_key_copied"));
    } catch {
      // Clipboard API can be blocked by the browser; surface no error toast,
      // the user can still copy manually from the visible field.
    }
  }

  function dismissApiKeyResult() {
    closeDialog(DialogID.CreateApiKeyResult);
    newApiKeyToken.value = null;
  }

  async function revokeApiKey(key: APIKeyOut) {
    const result = await confirm.open(t("profile.api_key_delete_confirm"));
    if (result.isCanceled) return;

    const { error } = await api.user.deleteApiKey(key.id);
    if (error) {
      toast.error(t("profile.toast.failed_api_key_delete"));
      return;
    }
    toast.success(t("profile.toast.api_key_deleted"));
    await loadApiKeys();
  }
</script>

<template>
  <div>
    <Dialog :dialog-id="DialogID.DuplicateSettings">
      <DialogContent class="hb-panel">
        <DialogHeader>
          <DialogTitle>{{ $t("items.duplicate.title") }}</DialogTitle>
        </DialogHeader>
        <ItemDuplicateSettings v-model="preferences.duplicateSettings" />
        <p class="text-sm text-muted-foreground">
          {{ $t("items.duplicate.override_instructions") }}
        </p>
      </DialogContent>
    </Dialog>

    <Dialog :dialog-id="DialogID.CreateApiKey">
      <DialogContent class="hb-panel">
        <DialogHeader>
          <DialogTitle>{{ $t("profile.api_key_create") }}</DialogTitle>
        </DialogHeader>

        <form class="space-y-4" @submit.prevent="submitCreateApiKey">
          <p
            v-if="apiKeyFormError"
            role="alert"
            class="rounded-md bg-destructive p-3 text-sm text-destructive-foreground"
          >
            {{ apiKeyFormError }}
          </p>
          <FormTextField
            v-model="apiKeyForm.name"
            :label="$t('profile.api_key_name')"
            :placeholder="$t('profile.api_key_name_placeholder')"
            :required="true"
            class="mb-2"
          />
          <div class="mb-2 max-w-[260px]">
            <FormCheckbox v-model="apiKeyForm.setExpiration" :label="$t('profile.api_key_set_expiration')" />
          </div>
          <p v-if="!apiKeyForm.setExpiration" class="mb-2 text-sm text-muted-foreground">
            {{ $t("profile.api_key_default_expiration") }}
          </p>
          <div v-if="apiKeyForm.setExpiration" class="mb-2">
            <label for="api-key-expiration" class="mb-1 block text-sm font-medium">{{
              $t("profile.api_key_expires_at")
            }}</label>
            <input
              id="api-key-expiration"
              v-model="apiKeyForm.expiresAt"
              type="date"
              class="hb-focus w-full rounded-md border bg-background px-3 py-2 text-sm"
              :required="apiKeyForm.setExpiration"
            />
          </div>

          <DialogFooter>
            <Button :disabled="apiKeyForm.submitting || !apiKeyForm.name.trim()" type="submit">
              <MdiLoading v-if="apiKeyForm.submitting" class="animate-spin" />
              {{ $t("global.create") }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <Dialog :dialog-id="DialogID.CreateApiKeyResult">
      <DialogContent class="hb-panel">
        <DialogHeader>
          <DialogTitle>{{ $t("profile.api_key_token_label") }}</DialogTitle>
        </DialogHeader>

        <p class="text-sm text-muted-foreground">{{ $t("profile.api_key_token_warning") }}</p>

        <div class="flex items-center gap-2">
          <input
            :aria-label="$t('profile.api_key_token_label')"
            :value="newApiKeyToken ?? ''"
            readonly
            class="hb-focus min-w-0 flex-1 rounded-md border bg-muted px-3 py-2 font-mono text-sm"
            @focus="($event.target as HTMLInputElement).select()"
          />
          <Button
            class="shrink-0"
            variant="secondary"
            size="icon"
            :aria-label="$t('profile.api_key_copy')"
            @click="copyApiKey"
          >
            <MdiContentCopy />
          </Button>
        </div>

        <DialogFooter>
          <Button @click="dismissApiKeyResult">{{ $t("global.close") }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog :dialog-id="DialogID.ChangePassword">
      <DialogContent class="hb-panel">
        <DialogHeader>
          <DialogTitle> {{ $t("profile.change_password") }} </DialogTitle>
        </DialogHeader>

        <p v-if="passwordError" role="alert" class="rounded-md bg-destructive p-3 text-sm text-destructive-foreground">
          {{ passwordError }}
        </p>
        <form id="change-password-form" name="change-password" method="post" @submit.prevent="changePassword">
          <FormPassword
            id="current-password"
            v-model="passwordChange.current"
            :label="$t('profile.current_password')"
            name="current-password"
            autocomplete="current-password"
            placeholder=""
            :required="true"
            class="mb-2"
          />
          <FormPassword
            id="new-password"
            v-model="passwordChange.new"
            :label="$t('profile.new_password')"
            name="new-password"
            autocomplete="new-password"
            placeholder=""
            :min-length="PASSWORD_MIN_LENGTH"
            :passwordrules="PASSWORD_RULES"
            :required="true"
          />
          <PasswordScore v-model:valid="passwordChange.isValid" :password="passwordChange.new" />

          <DialogFooter>
            <Button :disabled="!passwordChange.isValid || passwordChange.loading" type="submit">
              <MdiLoading v-if="passwordChange.loading" class="animate-spin" />
              {{ $t("global.submit") }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <BaseContainer class="flex min-w-0 flex-col gap-6">
      <h1 class="hb-section-title text-2xl">{{ $t("menu.profile") }}</h1>
      <BaseCard>
        <template #title>
          <BaseSectionHeader>
            <MdiAccount class="-mt-1 mr-2" />
            <span> {{ $t("profile.user_profile") }} </span>
            <template #description> {{ $t("profile.user_profile_sub") }} </template>
          </BaseSectionHeader>
        </template>

        <DetailsSection :details="details" />

        <div class="p-4">
          <div class="flex flex-wrap gap-2">
            <Button
              class="h-auto min-h-9 max-w-full whitespace-normal break-words text-start"
              variant="secondary"
              size="sm"
              @click="openDialog(DialogID.ChangePassword)"
            >
              {{ $t("profile.change_password") }}
            </Button>
            <Button
              class="h-auto min-h-9 max-w-full whitespace-normal break-words text-start"
              variant="secondary"
              size="sm"
              @click="logoutAllDevices"
            >
              {{ $t("profile.logout_all_devices") }}
            </Button>
            <Button
              class="h-auto min-h-9 max-w-full whitespace-normal break-words text-start"
              variant="secondary"
              size="sm"
              @click="openDialog(DialogID.DuplicateSettings)"
            >
              {{ $t("items.duplicate.title") }}
            </Button>
          </div>
        </div>
        <LanguageSelector />
      </BaseCard>

      <BaseCard>
        <template #title>
          <BaseSectionHeader>
            <MdiKeyVariant class="-mt-1 mr-2" />
            <span>{{ $t("profile.api_keys") }}</span>
            <template #description>{{ $t("profile.api_keys_sub") }}</template>
          </BaseSectionHeader>
        </template>

        <div class="px-4 pb-4">
          <p
            v-if="apiKeysError"
            role="alert"
            class="mb-3 rounded-md bg-destructive p-3 text-sm text-destructive-foreground"
          >
            {{ apiKeysError }}
          </p>
          <p v-if="apiKeysLoading" role="status" class="mb-3 text-sm text-muted-foreground">
            {{ $t("global.loading") }}
          </p>
          <RecordListExport
            :request="administrationExport('api-keys', exportLabel)"
            :disabled="apiKeysLoading || !!apiKeysError"
          />
          <div class="mx-1 divide-y rounded-md border">
            <p v-if="!apiKeysLoading && !apiKeysError && apiKeys.length === 0" class="p-2 text-center text-sm">
              {{ $t("profile.no_api_keys") }}
            </p>
            <article v-for="k in apiKeys" :key="k.id" class="p-2">
              <div class="flex flex-wrap items-center gap-2">
                <p class="mr-auto min-w-0 flex-1 break-words text-lg [overflow-wrap:anywhere]">{{ k.name }}</p>
                <Button variant="destructive" size="icon" :aria-label="$t('global.delete')" @click="revokeApiKey(k)">
                  <MdiDelete />
                </Button>
              </div>
              <div class="flex flex-wrap justify-between gap-x-4 gap-y-1 py-1 text-sm text-muted-foreground">
                <p>
                  {{ $t("profile.api_key_created") }}:
                  <DateTime format="relative" datetime-type="time" :date="k.createdAt" />
                </p>
                <p>
                  {{ $t("profile.api_key_last_used") }}:
                  <template v-if="k.lastUsedAt">
                    <DateTime format="relative" datetime-type="time" :date="k.lastUsedAt" />
                  </template>
                  <template v-else>{{ $t("profile.api_key_never_used") }}</template>
                </p>
                <p>
                  {{ $t("profile.api_key_expires_at") }}:
                  <template v-if="k.expiresAt">
                    <DateTime format="human" datetime-type="date" :date="k.expiresAt" />
                  </template>
                  <template v-else>{{ $t("profile.api_key_no_expiration") }}</template>
                </p>
              </div>
            </article>
          </div>

          <div class="mt-4">
            <Button
              class="h-auto min-h-9 max-w-full whitespace-normal break-words text-start"
              variant="secondary"
              size="sm"
              @click="openCreateApiKey"
            >
              {{ $t("profile.api_key_create") }}
            </Button>
          </div>
        </div>
      </BaseCard>

      <div class="hb-panel p-4">
        <HistoryListExport scope="private" />
      </div>
      <!-- TODO: Remove this notice once users are familiar with the collection-based settings. -->
      <BaseCard>
        <template #title>
          <BaseSectionHeader>
            <span> {{ $t("profile.moved_notice_title") }} </span>
            <template #description>
              {{ $t("profile.moved_notice_description") }}
            </template>
          </BaseSectionHeader>
        </template>

        <div class="space-y-2 px-4 pb-4 text-sm text-muted-foreground">
          <p>
            {{ $t("profile.moved_notice_body") }}
          </p>
          <div class="flex flex-wrap gap-2">
            <NuxtLink to="/collection/settings" class="text-primary underline">
              {{ $t("profile.moved_notice_link_settings") }}
            </NuxtLink>
            <NuxtLink to="/collection/notifiers" class="text-primary underline">
              {{ $t("profile.moved_notice_link_notifiers") }}
            </NuxtLink>
            <NuxtLink to="/collection/invites" class="text-primary underline">
              {{ $t("profile.moved_notice_link_invites") }}
            </NuxtLink>
          </div>
        </div>
      </BaseCard>
      <BaseCard>
        <template #title>
          <BaseSectionHeader>
            <MdiFill class="mr-2" />
            <span> {{ $t("profile.theme_settings") }} </span>
            <template #description>
              {{ $t("profile.theme_settings_sub") }}
            </template>
          </BaseSectionHeader>
        </template>

        <div class="px-4 pb-4">
          <div class="mb-3 flex flex-wrap gap-2">
            <Button
              class="h-auto min-h-9 max-w-full whitespace-normal break-words text-start"
              variant="secondary"
              size="sm"
              :aria-pressed="preferences.displayLegacyHeader"
              @click="setDisplayHeader"
            >
              {{ $t("profile.display_legacy_header", { currentValue: preferences.displayLegacyHeader }) }}
            </Button>
            <Button
              class="h-auto min-h-9 max-w-full whitespace-normal break-words text-start"
              variant="secondary"
              size="sm"
              :aria-pressed="preferences.legacyImageFit"
              @click="setLegacyImageFit"
            >
              {{ $t("profile.legacy_image_fit", { currentValue: preferences.legacyImageFit }) }}
            </Button>
          </div>
          <ThemePicker />
        </div>
      </BaseCard>

      <BaseCard>
        <template #title>
          <BaseSectionHeader>
            <MdiDelete class="-mt-1 mr-2" />
            <span> {{ $t("profile.delete_account") }} </span>
            <template #description> {{ $t("profile.delete_account_sub") }} </template>
          </BaseSectionHeader>
        </template>
        <div class="border-t-2 p-4 px-6">
          <Button size="sm" variant="destructive" @click="deleteProfile">
            {{ $t("profile.delete_account") }}
          </Button>
        </div>
      </BaseCard>
    </BaseContainer>
  </div>
</template>

<style scoped></style>
