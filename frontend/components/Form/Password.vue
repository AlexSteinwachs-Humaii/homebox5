<template>
  <div class="relative" :class="wrapperClass()">
    <FormTextField
      v-bind="fieldAttrs()"
      v-model="value"
      :placeholder="localizedPlaceholder"
      :label="localizedLabel"
      :type="inputType"
    />
    <TooltipProvider :delay-duration="0">
      <Tooltip>
        <TooltipTrigger as-child>
          <button
            type="button"
            class="hb-focus absolute bottom-0 right-1 inline-flex size-10 items-center justify-center rounded-md"
            :aria-label="$t('components.form.password.toggle_show')"
            :aria-pressed="!hide"
            @click="toggle()"
          >
            <MdiEye name="mdi-eye" class="size-5" />
          </button>
        </TooltipTrigger>
        <TooltipContent>{{ $t("components.form.password.toggle_show") }}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  </div>
</template>

<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import MdiEye from "~icons/mdi/eye";
  import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
  import FormTextField from "@/components/Form/TextField.vue";

  defineOptions({
    inheritAttrs: false,
  });

  const attrs = useAttrs();

  const { t } = useI18n();
  type Props = {
    modelValue: string;
    placeholder?: string;
    label: string;
  };

  const props = defineProps<Props>();

  const [hide, toggle] = useToggle(true);

  const localizedPlaceholder = computed(() => props.placeholder ?? t("global.password"));
  const localizedLabel = computed(() => props.label ?? t("global.password"));

  const inputType = computed(() => {
    return hide.value ? "password" : "text";
  });

  function wrapperClass() {
    return attrs.class;
  }

  function fieldAttrs() {
    const { class: _class, ...rest } = attrs;
    return rest;
  }

  const value = useVModel(props, "modelValue");
</script>

<style scoped>
  :deep(input) {
    padding-right: 3rem;
  }
</style>
