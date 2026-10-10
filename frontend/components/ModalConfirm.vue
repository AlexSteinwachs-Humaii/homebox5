<template>
  <AlertDialog :open="isRevealed">
    <AlertDialogContent @escape-key-down="cancel(false)" @close-auto-focus="restoreFocus">
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t("global.confirm") }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ text || $t("global.delete_confirm") }}
        </AlertDialogDescription>
        <div v-if="href && href !== ''">
          <a :href="href" target="_blank" rel="noopener noreferrer" class="break-all text-sm text-primary underline">
            {{ href }}
          </a>
        </div>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel @click="cancel(false)">
          {{ $t("global.cancel") }}
        </AlertDialogCancel>
        <AlertDialogAction @click="confirm(true)">
          {{ $t("global.confirm") }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>

<script setup lang="ts">
  import { useDialog } from "./ui/dialog-provider";
  import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
  } from "@/components/ui/alert-dialog";

  const { text, href, isRevealed, confirm, cancel } = useConfirm();
  const { addAlert, removeAlert } = useDialog();
  let opener: HTMLElement | null = null;

  function restoreFocus(event: Event) {
    if (opener?.isConnected) {
      event.preventDefault();
      opener.focus();
    }
    opener = null;
  }

  watch(
    isRevealed,
    val => {
      if (val) {
        opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
        addAlert("confirm-modal");
      } else {
        removeAlert("confirm-modal");
      }
    },
    { immediate: true }
  );
</script>
