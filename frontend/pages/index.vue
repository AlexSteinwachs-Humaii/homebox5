<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { toast } from "@/components/ui/sonner";
  import MdiGithub from "~icons/mdi/github";
  import MdiDiscord from "~icons/mdi/discord";
  import MdiFolder from "~icons/mdi/folder";
  import MdiAccount from "~icons/mdi/account";
  import MdiAccountPlus from "~icons/mdi/account-plus";
  import MdiLogin from "~icons/mdi/login";
  import MdiArrowRight from "~icons/mdi/arrow-right";
  import MdiLock from "~icons/mdi/lock";
  import MdiMastodon from "~icons/mdi/mastodon";
  import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
  import { Button } from "@/components/ui/button";
  import LanguageSelector from "~/components/App/LanguageSelector.vue";
  import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
  import AppLogo from "~/components/App/Logo.vue";
  import FormTextField from "~/components/Form/TextField.vue";
  import FormPassword from "~/components/Form/Password.vue";
  import FormCheckbox from "~/components/Form/Checkbox.vue";
  import PasswordScore from "~/components/global/PasswordScore.vue";
  import { PASSWORD_MIN_LENGTH, PASSWORD_RULES } from "~/lib/passwords";

  const { t } = useI18n();

  useHead({
    title: "HomeBox | " + t("index.title"),
  });

  definePageMeta({
    layout: "empty",
    middleware: [
      () => {
        const ctx = useAuthContext();
        const route = useRoute();
        if (ctx.isAuthorized()) {
          // Preserve invitation token when redirecting authenticated users
          if (route.query.token) {
            return `/home?token=${encodeURIComponent(route.query.token as string)}`;
          }
          return "/home";
        } else {
          console.log("Logged out, clearing collectionId preference");
          const prefs = useViewPreferences();
          if (prefs.value.collectionId) {
            prefs.value.collectionId = null;
          }
        }
      },
    ],
  });

  const ctx = useAuthContext();

  const api = usePublicApi();
  // Use ref for OIDC error state management
  const oidcError = ref<string | null>(null);
  const shownErrorMessage = ref(false);

  const { data: status } = useAsyncData(async () => {
    const { data } = await api.status();

    if (data.demo) {
      username.value = "demo@example.com";
      password.value = "demodemo";
    }
    return data;
  });

  whenever(status, status => {
    if (status?.demo) {
      email.value = "demo@example.com";
      loginPassword.value = "demodemo";
    }

    // Auto-redirect to OIDC if autoRedirect is enabled, but not if there's an OIDC initialization error
    if (status?.oidc?.enabled && status?.oidc?.autoRedirect && !oidcError.value && !shownErrorMessage.value) {
      loginWithOIDC();
    }
  });

  const route = useRoute();
  const router = useRouter();

  const username = ref("");
  const email = ref("");
  const password = ref("");
  const canRegister = ref(false);
  const remember = ref(false);

  const groupToken = computed<string>({
    get() {
      const params = route.query.token;

      if (typeof params === "string") {
        return params;
      }

      return "";
    },
    set(v) {
      router.push({
        query: {
          token: v,
        },
      });
    },
  });

  async function registerUser() {
    loading.value = true;

    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

    if (!emailRegex.test(email.value)) {
      toast.error(t("index.toast.invalid_email"));
      loading.value = false;
      return;
    }

    const { error } = await api.register({
      name: username.value,
      email: email.value,
      password: password.value,
      token: groupToken.value,
    });

    if (error) {
      toast.error(t("index.toast.problem_registering"), {
        classes: {
          title: "login-error",
        },
      });
      return;
    }

    toast.success(t("index.toast.user_registered"));

    loading.value = false;
    registerForm.value = false;
  }

  onMounted(() => {
    if (groupToken.value !== "") {
      registerForm.value = true;
    }

    // Handle OIDC error notifications from URL parameters
    const oidcErrorParam = route.query.oidc_error;
    if (typeof oidcErrorParam === "string" && oidcErrorParam.startsWith("oidc_")) {
      // Set the error state to prevent auto-redirect
      oidcError.value = oidcErrorParam;
      shownErrorMessage.value = true;

      const translationKey = `index.toast.${oidcErrorParam}`;
      let errorMessage = t(translationKey);

      // If there are additional details, append them
      const details = route.query.details;
      if (typeof details === "string" && details.trim() !== "") {
        errorMessage += `: ${details}`;
      }

      toast.error(errorMessage);

      // Clean up the URL by removing the error parameters
      const newQuery = { ...route.query };
      delete newQuery.oidc_error;
      delete newQuery.details;
      router.replace({ query: newQuery });

      // Clear the error state after showing the message
      oidcError.value = null;
    }
  });

  const loading = ref(false);
  const loginPassword = ref("");
  const redirectTo = useState("authRedirect");

  async function login() {
    loading.value = true;
    const { error } = await ctx.login(api, email.value, loginPassword.value, remember.value);

    if (error) {
      toast.error(t("index.toast.invalid_email_password"), {
        classes: {
          title: "login-error",
        },
      });
      loading.value = false;
      return;
    }

    toast.success(t("index.toast.login_success"));

    navigateTo(redirectTo.value || "/home");
    redirectTo.value = null;
    loading.value = false;
  }

  function loginWithOIDC() {
    window.location.href = "/api/v1/users/login/oidc";
  }

  const [registerForm, toggleLogin] = useToggle();
</script>

<template>
  <div class="relative flex min-h-dvh flex-col bg-background text-foreground">
    <div class="relative">
      <header
        class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-6 border-b border-border px-page-gutter py-8 sm:py-12"
      >
        <div class="min-w-0">
          <h1 class="flex items-center text-4xl font-semibold tracking-tight sm:text-5xl">
            HomeB
            <AppLogo class="w-12" />
            x
          </h1>
          <p class="mt-3 text-lg text-muted-foreground">{{ $t("index.tagline") }}</p>
        </div>
        <TooltipProvider :delay-duration="0">
          <div class="flex flex-wrap items-center gap-4 text-muted-foreground">
            <Tooltip>
              <TooltipTrigger as-child>
                <a
                  class="hb-focus hover:text-foreground"
                  :aria-label="$t('global.github')"
                  href="https://github.com/sysadminsmedia/homebox"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <MdiGithub class="size-8" />
                </a>
              </TooltipTrigger>
              <TooltipContent>{{ $t("global.github") }}</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger as-child>
                <a
                  class="hb-focus hover:text-foreground"
                  :aria-label="$t('global.follow_dev')"
                  href="https://noc.social/@sysadminszone"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <MdiMastodon class="size-8" />
                </a>
              </TooltipTrigger>
              <TooltipContent>{{ $t("global.follow_dev") }}</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger as-child>
                <a
                  class="hb-focus hover:text-foreground"
                  :aria-label="$t('global.join_discord')"
                  href="https://discord.gg/aY4DCkpNA9"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <MdiDiscord class="size-8" />
                </a>
              </TooltipTrigger>
              <TooltipContent>{{ $t("global.join_discord") }}</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger as-child>
                <a
                  class="hb-focus hover:text-foreground"
                  :aria-label="$t('global.read_docs')"
                  href="https://homebox.software/en/"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <MdiFolder class="size-8" />
                </a>
              </TooltipTrigger>
              <TooltipContent>{{ $t("global.read_docs") }}</TooltipContent>
            </Tooltip>

            <LanguageSelector class="text-foreground" :expanded="false" />
          </div>
        </TooltipProvider>
      </header>
      <div class="grid min-h-[50vh] place-items-center px-page-gutter py-10">
        <div class="w-full min-w-0 max-w-[500px]">
          <Transition name="slide-fade">
            <form v-if="registerForm" id="register-form" name="register" method="post" @submit.prevent="registerUser">
              <Card class="hb-panel w-full">
                <CardHeader>
                  <CardTitle class="flex items-center gap-2 leading-snug">
                    <MdiAccount class="mr-1 size-7 shrink-0" />
                    {{ $t("index.register") }}
                  </CardTitle>
                </CardHeader>
                <CardContent class="flex flex-col gap-2">
                  <FormTextField
                    id="register-email"
                    v-model="email"
                    :label="$t('index.set_email')"
                    type="email"
                    name="email"
                    autocomplete="username"
                    :required="true"
                    data-testid="email-input"
                  />
                  <FormTextField
                    id="register-name"
                    v-model="username"
                    :label="$t('index.set_name')"
                    name="name"
                    autocomplete="name"
                    :required="true"
                    data-testid="name-input"
                  />
                  <div v-if="!(groupToken == '')" class="pb-1 pt-4 text-center">
                    <p>{{ $t("index.joining_group") }}</p>
                    <button type="button" class="text-xs underline" @click="groupToken = ''">
                      {{ $t("index.dont_join_group") }}
                    </button>
                  </div>
                  <FormPassword
                    id="register-password"
                    v-model="password"
                    :label="$t('index.set_password')"
                    name="new-password"
                    autocomplete="new-password"
                    :min-length="PASSWORD_MIN_LENGTH"
                    :passwordrules="PASSWORD_RULES"
                    :required="true"
                    data-testid="password-input"
                  />
                  <PasswordScore v-model:valid="canRegister" :password="password" />
                </CardContent>
                <CardFooter>
                  <Button
                    data-testid="confirm-register-button"
                    class="h-auto min-h-9 w-full whitespace-normal break-words"
                    type="submit"
                    :class="loading ? 'loading' : ''"
                    :disabled="loading || !canRegister"
                  >
                    {{ $t("index.register") }}
                  </Button>
                </CardFooter>
              </Card>
            </form>
            <form v-else id="login-form" name="login" method="post" @submit.prevent="login">
              <Card class="hb-panel w-full">
                <CardHeader>
                  <CardTitle class="flex items-center gap-2 leading-snug">
                    <MdiAccount class="mr-1 size-7 shrink-0" />
                    {{ $t("index.login") }}
                  </CardTitle>
                </CardHeader>
                <CardContent v-if="status?.oidc?.allowLocal !== false" class="flex flex-col gap-2">
                  <template v-if="status && status.demo">
                    <p class="text-center text-xs italic">
                      {{ $t("global.demo_instance") }}
                    </p>
                    <p class="text-center text-xs">
                      <b>{{ $t("global.email") }}</b> demo@example.com
                    </p>
                    <p class="text-center text-xs">
                      <b>{{ $t("global.password") }}</b> demodemo
                    </p>
                  </template>
                  <FormTextField
                    id="login-username"
                    v-model="email"
                    :label="$t('global.email')"
                    name="username"
                    autocomplete="username"
                    :required="true"
                  />
                  <FormPassword
                    id="login-password"
                    v-model="loginPassword"
                    :label="$t('global.password')"
                    name="password"
                    autocomplete="current-password"
                    :required="true"
                  />
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div class="min-w-0">
                      <FormCheckbox v-model="remember" :label="$t('index.remember_me')" />
                    </div>
                    <NuxtLink to="/forgot-password" class="hb-focus text-sm text-primary hover:underline">
                      {{ $t("index.forgot_password") }}
                    </NuxtLink>
                  </div>
                </CardContent>
                <CardFooter class="flex flex-col gap-2">
                  <Button
                    v-if="status?.oidc?.allowLocal !== false"
                    class="h-auto min-h-9 w-full whitespace-normal break-words"
                    type="submit"
                    :class="loading ? 'loading' : ''"
                    :disabled="loading"
                  >
                    {{ $t("index.login") }}
                  </Button>

                  <div
                    v-if="status?.oidc?.enabled && status?.oidc?.allowLocal !== false"
                    class="flex w-full items-center gap-2"
                  >
                    <hr class="flex-1" />
                    <span class="text-xs text-muted-foreground">{{ $t("index.or") }}</span>
                    <hr class="flex-1" />
                  </div>

                  <Button
                    v-if="status?.oidc?.enabled"
                    type="button"
                    variant="outline"
                    class="h-auto min-h-9 w-full whitespace-normal break-words"
                    @click="loginWithOIDC"
                  >
                    {{ status.oidc.buttonText || "Sign in with OIDC" }}
                  </Button>
                </CardFooter>
              </Card>
            </form>
          </Transition>
          <div class="mt-6 text-center">
            <Button
              v-if="status && status.allowRegistration && status?.oidc?.allowLocal !== false"
              class="group h-auto min-h-9 max-w-full whitespace-normal break-words"
              variant="link"
              data-testid="register-button"
              @click="() => toggleLogin()"
            >
              <div class="relative mx-2">
                <div
                  class="absolute inset-0 flex items-center justify-center transition-transform duration-300 group-hover:rotate-[360deg]"
                >
                  <MdiAccountPlus v-if="!registerForm" class="size-5 group-hover:hidden" />
                  <MdiLogin v-else class="size-5 group-hover:hidden" />
                  <MdiArrowRight class="hidden size-5 group-hover:block" />
                </div>
              </div>
              {{ registerForm ? $t("index.login") : $t("index.register") }}
            </Button>
            <p v-else class="inline-flex items-center gap-2 text-sm italic">
              <MdiLock class="inline-block size-4" />
              {{ $t("index.disabled_registration") }}
            </p>
          </div>
        </div>
      </div>
    </div>
    <footer v-if="status" class="mt-auto w-full break-words px-page-gutter pb-6 text-center text-muted-foreground">
      <p class="text-center text-sm">
        {{ $t("global.version", { version: status.build.version }) }} ~
        {{ $t("global.build", { build: status.build.commit }) }}
      </p>
    </footer>
  </div>
</template>

<style lang="css" scoped>
  .slide-fade-enter-active {
    transition: all 0.2s ease-out;
  }

  .slide-fade-enter-from,
  .slide-fade-leave-to {
    position: absolute;
    transform: translateX(20px);
    opacity: 0;
  }

  progress[value]::-webkit-progress-value {
    transition: width 0.5s;
  }
</style>
