<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { DialogID, type NoParamDialogIDs } from "@/components/ui/dialog-provider/utils";
  import {
    CommandDialog,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
    CommandSeparator,
  } from "~/components/ui/command";
  import { Shortcut } from "~/components/ui/shortcut";
  import { useDialog, useDialogHotkey } from "~/components/ui/dialog-provider";

  export type QuickMenuAction =
    | { text: string; href: string; type: "navigate" }
    | {
        text: string;
        dialogId: DialogID;
        shortcut: string;
        type: "create";
        id?: number;
      };

  const props = defineProps({
    actions: {
      type: Array as PropType<QuickMenuAction[]>,
      required: false,
      default: () => [],
    },
  });

  const { t } = useI18n();
  const { closeDialog, openDialog } = useDialog();

  useDialogHotkey(DialogID.QuickMenu, { code: "Backquote", ctrl: true });

  function selectCreate(action: Extract<QuickMenuAction, { type: "create" }>) {
    if (action.dialogId === DialogID.CreateEntity) {
      openDialog(DialogID.CreateEntity, { params: { baseType: action.id === 1 ? "location" : "item" } });
    } else {
      openDialog(action.dialogId as NoParamDialogIDs);
    }
  }
</script>

<template>
  <CommandDialog :dialog-id="DialogID.QuickMenu" :title="t('components.quick_menu.shortcut_hint')">
    <CommandInput
      :placeholder="t('components.quick_menu.shortcut_hint')"
      :aria-label="t('global.search')"
      @keydown="
        (e: KeyboardEvent) => {
          const item = props.actions.filter(item => 'shortcut' in item).find(item => item.shortcut === e.key);
          if (item) {
            e.preventDefault();
            if (item.type === 'create') selectCreate(item);
          }
          // if esc is pressed, close the dialog
          if (e.key === 'Escape') {
            e.preventDefault();
            closeDialog(DialogID.QuickMenu);
          }
        }
      "
    />
    <CommandList>
      <CommandSeparator />
      <CommandEmpty>{{ t("components.quick_menu.no_results") }}</CommandEmpty>
      <CommandGroup :heading="t('global.create')">
        <CommandItem
          v-for="(create, i) in props.actions.filter(item => item.type === 'create')"
          :key="`$global.create_${i + 1}`"
          :value="create.text"
          @select="
            e => {
              e.preventDefault();
              selectCreate(create);
            }
          "
        >
          {{ create.text }}
          <Shortcut v-if="'shortcut' in create" class="ml-auto" size="sm" :keys="[create.shortcut]" />
        </CommandItem>
      </CommandGroup>
      <CommandSeparator />
      <CommandGroup :heading="t('global.navigate')">
        <CommandItem
          v-for="(navigate, i) in props.actions.filter(item => item.type === 'navigate')"
          :key="navigate.text"
          :value="`global.navigate_${i + 1}`"
          @select="
            () => {
              closeDialog(DialogID.QuickMenu);
              navigateTo(navigate.href);
            }
          "
        >
          {{ navigate.text }}
        </CommandItem>
        <CommandItem
          value="scanner"
          @select="
            () => {
              closeDialog(DialogID.QuickMenu);
              openDialog(DialogID.Scanner);
            }
          "
        >
          {{ t("menu.scanner") }}
        </CommandItem>
      </CommandGroup>
    </CommandList>
  </CommandDialog>
</template>
