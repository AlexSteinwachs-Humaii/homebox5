<template>
  <div class="flex min-w-0 flex-col gap-1">
    <Label :for="id" class="break-words px-1">
      {{ props.name ?? $t("global.tags") }}
    </Label>

    <div class="flex min-w-0 items-center gap-2">
      <Popover v-model:open="open">
        <PopoverTrigger as-child>
          <Button
            :id="id"
            variant="outline"
            role="combobox"
            :aria-expanded="open"
            class="min-w-0 flex-1 justify-between"
          >
            <span class="flex min-w-0 flex-auto items-center gap-2 truncate text-left">
              <span
                v-if="value?.color"
                class="shrink-0 rounded-full"
                :style="{
                  width: '1rem',
                  height: '1rem',
                  backgroundColor: value.color,
                }"
              />
              <span class="truncate">
                {{ value && value.name ? value.name : $t("components.tag.selector.select_tags") }}
              </span>
            </span>

            <span class="ml-2 flex items-center">
              <ChevronsUpDown class="ml-2 size-4 shrink-0 opacity-50" />
            </span>
          </Button>
        </PopoverTrigger>
        <PopoverContent class="w-[--reka-popper-anchor-width] p-0">
          <Command :ignore-filter="true">
            <div class="p-2" @keydown.stop>
              <RecordListExport
                :request="
                  selectorExport(
                    'tags',
                    filteredTags.map(r => r.id),
                    id => $t(`csv_export.fields.${id}`)
                  )
                "
              />
            </div>
            <CommandInput
              v-model="search"
              :placeholder="$t('components.tag.selector.select_tags')"
              :display-value="_ => ''"
            />
            <CommandEmpty>{{ $t("components.tag.selector.no_tags_found") }}</CommandEmpty>
            <CommandList>
              <CommandGroup>
                <CommandItem v-for="tag in filteredTags" :key="tag.id" :value="tag.id" @select="selectTag(tag)">
                  <Check :class="cn('mr-2 h-4 w-4 shrink-0', value?.id === tag.id ? 'opacity-100' : 'opacity-0')" />
                  <span
                    v-if="tag.color"
                    class="mr-2 size-4 shrink-0 rounded-full"
                    :style="{ backgroundColor: tag.color }"
                  />
                  <div class="min-w-0 break-words [overflow-wrap:anywhere]">
                    {{ tag.name }}
                  </div>
                </CommandItem>
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
      <button
        v-if="value"
        type="button"
        class="hb-focus flex size-8 shrink-0 items-center justify-center rounded-md hover:bg-accent"
        :aria-label="$t('components.tag.selector.clear')"
        @click.stop.prevent="clearSelection"
      >
        <X class="size-4" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
  import RecordListExport from "~/components/Export/RecordListExport.vue";
  import { selectorExport } from "~/lib/exports/auxiliary";
  import { Check, ChevronsUpDown, X } from "lucide-vue-next";
  import fuzzysort from "fuzzysort";
  import { Button } from "~/components/ui/button";
  import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "~/components/ui/command";
  import { Label } from "~/components/ui/label";
  import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
  import { cn } from "~/lib/utils";
  import type { TagOut } from "~~/lib/api/types/data-contracts";

  type Props = {
    modelValue?: TagOut | null;
    tags: TagOut[];
    name?: string;
  };

  const props = defineProps<Props>();
  const emit = defineEmits(["update:modelValue"]);

  const open = ref(false);
  const search = ref("");
  const id = useId();
  const value = useVModel(props, "modelValue", emit);

  function selectTag(tag: TagOut) {
    if (value.value?.id !== tag.id) {
      value.value = tag;
    } else {
      value.value = null;
    }
    open.value = false;
  }

  function clearSelection() {
    value.value = null;
    search.value = "";
    open.value = false;
  }

  const filteredTags = computed(() => {
    const filtered = fuzzysort.go(search.value, props.tags, { key: "name", all: true }).map(i => i.obj);

    return filtered;
  });

  // Reset search when value is cleared
  watch(
    () => value.value,
    () => {
      if (!value.value) {
        search.value = "";
      }
    }
  );
</script>
