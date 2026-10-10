<script setup lang="ts">
  import { Button } from "@/components/ui/button";
  import BaseContainer from "@/components/Base/Container.vue";
  import BaseCard from "@/components/Base/Card.vue";
  import { useDialog } from "@/components/ui/dialog-provider";
  import { DialogID } from "@/components/ui/dialog-provider/utils";

  definePageMeta({
    middleware: ["auth"],
  });

  const { openDialog } = useDialog();
</script>

<template>
  <BaseContainer class="flex justify-center py-10">
    <div class="w-full min-w-0 max-w-2xl">
      <BaseCard>
        <template #title>
          <h2 class="hb-display break-words text-center text-3xl sm:text-4xl">
            {{ $t("collection.no_collections.title") }}
          </h2>
        </template>

        <div class="mx-4 mb-4 flex flex-col gap-4 text-sm text-muted-foreground">
          <p class="text-center text-base text-foreground">
            {{ $t("collection.no_collections.message") }}
          </p>

          <div class="flex flex-col gap-2 sm:flex-row sm:justify-center">
            <Button
              class="h-auto min-h-9 w-full whitespace-normal break-words sm:w-auto"
              @click="
                openDialog(DialogID.CreateCollection, {
                  params: {
                    redirectTo: '/home',
                  },
                })
              "
            >
              {{ $t("collection.no_collections.create") }}
            </Button>
            <Button
              class="h-auto min-h-9 w-full whitespace-normal break-words sm:w-auto"
              variant="secondary"
              @click="
                openDialog(DialogID.JoinCollection, {
                  params: {
                    redirectTo: '/home',
                  },
                })
              "
            >
              {{ $t("collection.no_collections.join") }}
            </Button>
          </div>
        </div>
      </BaseCard>
    </div>
  </BaseContainer>
</template>
