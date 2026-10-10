<template>
  <div class="min-w-0 border-t">
    <dl class="divide-y divide-border">
      <div
        v-for="detail in details"
        :key="detail.name"
        class="group grid min-w-0 gap-2 p-4 sm:grid-cols-3 sm:gap-4 sm:px-6"
      >
        <dt class="min-w-0 text-sm font-medium text-muted-foreground">
          {{ $t(detail.name) }}
        </dt>
        <dd class="min-w-0 text-start text-sm leading-relaxed sm:col-span-2">
          <slot :name="detail.slot || detail.name" v-bind="{ detail }">
            <DateTime
              v-if="detail.type == 'date'"
              :date="detail.text"
              :datetime-type="detail.date ? 'date' : 'datetime'"
            />
            <Currency v-else-if="detail.type == 'currency'" :amount="detail.text" />
            <template v-else-if="detail.type === 'link'">
              <TooltipProvider :delay-duration="0">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <a
                      :href="detail.href"
                      target="_blank"
                      rel="noopener noreferrer"
                      :class="badgeVariants()"
                      class="hb-focus max-w-full gap-1 whitespace-normal text-start"
                    >
                      <MdiOpenInNew />
                      {{ detail.text }}
                    </a>
                  </TooltipTrigger>
                  <TooltipContent>
                    {{ detail.href }}
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </template>
            <template v-else-if="detail.type === 'markdown'">
              <ClientOnly>
                <!-- eslint-disable-next-line tailwindcss/no-custom-classname -->
                <div class="markdown-container min-w-0 break-words">
                  <Markdown :source="detail.text" />
                </div>
              </ClientOnly>
            </template>
            <template v-else>
              <!-- Fixed version with improved overflow handling -->
              <span class="flex w-full items-center break-words">
                <a
                  v-if="maybeUrl(detail.text.toString()).isUrl"
                  :href="maybeUrl(detail.text.toString()).url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="hb-focus min-w-0 break-words text-primary underline hover:text-primary/80"
                  >{{ detail.text }}</a
                >
                <span v-else class="min-w-0 break-words">{{ detail.text }}</span>
                <span v-if="detail.copyable" class="my-0 ml-4 shrink-0">
                  <CopyText v-if="detail.text.toString()" :text="detail.text.toString()" :icon-size="16" />
                </span>
              </span>
            </template>
          </slot>
        </dd>
      </div>
    </dl>
  </div>
</template>

<script setup lang="ts">
  import type { AnyDetail, Detail } from "./types";
  import MdiOpenInNew from "~icons/mdi/open-in-new";
  import { badgeVariants } from "~/components/ui/badge";
  import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
  import DateTime from "@/components/global/DateTime.vue";
  import Currency from "@/components/global/Currency.vue";
  import Markdown from "@/components/global/Markdown.vue";
  import CopyText from "@/components/global/CopyText.vue";

  defineProps({
    details: {
      type: Object as () => (Detail | AnyDetail)[],
      required: true,
    },
  });
</script>

<style scoped>
  /* Use :deep to target elements inside the markdown container */
  :deep(.markdown-container) {
    /* General container styles */
    width: 100%;
    overflow-wrap: break-word;
    word-wrap: break-word;

    /* Handle tables */
    table {
      table-layout: fixed;
      width: 100%;
    }

    td,
    th {
      word-break: break-word;
      overflow-wrap: break-word;
      max-width: 100%;
    }

    /* Handle code blocks */
    pre,
    code {
      white-space: pre-wrap;
      word-break: break-all;
      overflow-x: auto;
    }

    /* Handle images */
    img {
      max-width: 100%;
      height: auto;
    }

    /* Handle headings */
    h1,
    h2,
    h3,
    h4,
    h5,
    h6 {
      word-break: break-word;
      overflow-wrap: break-word;
    }

    /* Handle blockquotes */
    blockquote {
      overflow-wrap: break-word;
      word-break: break-word;
    }

    /* Handle inline elements */
    a,
    strong,
    em {
      word-break: break-all;
      overflow-wrap: break-word;
    }
  }
</style>
