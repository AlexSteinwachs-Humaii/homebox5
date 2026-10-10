<template>
  <div v-if="!inline" data-entity-datepicker class="flex w-full min-w-0 flex-col gap-1.5">
    <Label :for="`dp-input-${id}`" class="cursor-pointer"> {{ label }} </Label>
    <VueDatePicker
      v-model="selected"
      class="min-w-0 sm:col-span-3"
      :uid="id"
      :aria-labels="{ input: label }"
      :enable-time-picker="false"
      clearable
      :dark="isDark"
      :format="formatDate"
    />
  </div>
  <div v-else data-entity-datepicker class="min-w-0 sm:grid sm:grid-cols-4 sm:items-start sm:gap-4">
    <Label :for="`dp-input-${id}`" class="flex w-full cursor-pointer px-1 py-2"> {{ label }} </Label>
    <VueDatePicker
      v-model="selected"
      class="min-w-0 sm:col-span-3"
      :uid="id"
      :aria-labels="{ input: label }"
      :enable-time-picker="false"
      clearable
      :dark="isDark"
      :format="formatDate"
    />
  </div>
</template>

<script setup lang="ts">
  import VueDatePicker from "@vuepic/vue-datepicker";
  import "@vuepic/vue-datepicker/dist/main.css";
  import { isDateOnlyString, parseDateOnly, toDateOnlyString } from "~/lib/datelib/dateOnly";
  import { Label } from "@/components/ui/label";
  import { darkThemes } from "~/lib/data/themes";

  const emit = defineEmits(["update:modelValue", "update:text"]);

  const props = defineProps({
    modelValue: {
      type: [Date, String] as unknown as () => Date | string | null,
      required: false,
      default: null,
    },
    inline: {
      type: Boolean,
      default: false,
    },
    label: {
      type: String,
      default: "Date",
    },
  });

  const id = useId();

  const isDark = useIsThemeInList(darkThemes);

  const formatDate = (date: Date | string | number) => fmtDate(date, "human", "date");

  const selected = computed<Date | null>({
    get() {
      // String
      if (typeof props.modelValue === "string") {
        // Empty string
        if (props.modelValue === "") {
          return null;
        }

        // Invalid Date string
        if (props.modelValue === "Invalid Date") {
          return null;
        }

        // YYYY-MM-DD is read through local components. `new Date("2026-04-18")`
        // would be UTC midnight, which the calendar then highlights as the
        // 17th for anyone west of Greenwich.
        if (isDateOnlyString(props.modelValue)) {
          // parseDateOnly also rejects impossible days. Those must not reach
          // the constructor below: `new Date("2026-02-30")` rolls over to
          // March 2 rather than failing, and the picker would then re-emit
          // that invented date on the next save.
          return parseDateOnly(props.modelValue);
        }

        // Timestamps from older records still fall back to the plain
        // constructor.
        const parsed = new Date(props.modelValue);
        return isNaN(parsed.getTime()) ? null : parsed;
      }

      // Date
      if (props.modelValue instanceof Date) {
        if (props.modelValue.getFullYear() < 1000) {
          return null;
        }

        if (isNaN(props.modelValue.getTime())) {
          return null;
        }

        // Valid Date
        return props.modelValue;
      }

      return null;
    },
    set(value: Date | null) {
      // Always a YYYY-MM-DD string built from local components. Emitting a
      // Date instead would let JSON.stringify serialize it as a UTC instant,
      // which lands on the previous day for every user west of Greenwich and
      // shifts again on each subsequent save. Every field this picker drives
      // is a calendar date (types.Date on the backend), so there is no case
      // where a Date object is the right thing to emit — a timestamp field
      // should use VueDatePicker directly.
      emit("update:modelValue", value ? toDateOnlyString(value) : "");
    },
  });
</script>

<style scoped>
  [data-entity-datepicker] :deep(.dp__main) {
    --dp-background-color: hsl(var(--background));
    --dp-text-color: hsl(var(--foreground));
    --dp-border-color: hsl(var(--input));
    --dp-border-color-hover: hsl(var(--ring));
    --dp-border-color-focus: hsl(var(--ring));
    --dp-primary-color: hsl(var(--primary));
    --dp-primary-text-color: hsl(var(--primary-foreground));
    --dp-secondary-color: hsl(var(--muted-foreground));
    --dp-icon-color: hsl(var(--muted-foreground));
    --dp-hover-color: hsl(var(--accent));
    --dp-hover-text-color: hsl(var(--accent-foreground));
    --dp-border-radius: var(--radius);
    --dp-font-family: inherit;
  }

  [data-entity-datepicker] :deep(.dp__input:focus-visible) {
    outline: 2px solid hsl(var(--ring));
    outline-offset: 2px;
  }
</style>
