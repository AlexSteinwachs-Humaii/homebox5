<script setup lang="ts">
  import { useI18n } from "vue-i18n";

  const { t } = useI18n();
  // Instance-specific paint IDs keep this original artwork self-contained.
  const id = useId();
</script>

<template>
  <header class="relative grid gap-4 overflow-hidden border-b border-border py-6 lg:grid-cols-2 lg:py-10">
    <div class="relative z-10 self-center">
      <p class="hb-eyebrow mb-3 text-primary">{{ t("home.hero.eyebrow") }}</p>
      <h1 class="text-4xl font-normal leading-tight tracking-tight sm:text-6xl">
        <i18n-t keypath="home.hero.title" tag="span" scope="global">
          <template #everything>
            <em class="font-editorial font-normal">{{ t("home.hero.everything") }}</em>
          </template>
        </i18n-t>
      </h1>
      <p class="mt-4 max-w-lg text-sm text-muted-foreground">{{ t("home.hero.description") }}</p>
    </div>

    <!-- Original decorative camera illustration; never an inventory photo or a record. -->
    <svg
      class="pointer-events-none hidden h-56 w-full self-center lg:block"
      viewBox="0 0 520 300"
      aria-hidden="true"
      focusable="false"
    >
      <defs>
        <linearGradient :id="`${id}-body`" x1="0" y1="0" x2="0.6" y2="1">
          <stop stop-color="#59616c" />
          <stop offset="0.25" stop-color="#222a33" />
          <stop offset="0.75" stop-color="#090d12" />
          <stop offset="1" stop-color="#353f4a" />
        </linearGradient>
        <linearGradient :id="`${id}-rim`" x1="0" y1="0" x2="1" y2="1">
          <stop stop-color="#edf4fa" />
          <stop offset="0.4" stop-color="#9eaab8" />
          <stop offset="0.7" stop-color="#252e39" />
          <stop offset="1" stop-color="#737f8c" />
        </linearGradient>
        <radialGradient :id="`${id}-glass`" cx="0.3" cy="0.25">
          <stop stop-color="#617d9c" />
          <stop offset="0.4" stop-color="#26394f" />
          <stop offset="0.7" stop-color="#0b1420" />
          <stop offset="1" stop-color="#03080d" />
        </radialGradient>
        <radialGradient :id="`${id}-glow`">
          <stop stop-color="#657b8c" stop-opacity="0.2" />
          <stop offset="1" stop-color="#657b8c" stop-opacity="0" />
        </radialGradient>
        <pattern :id="`${id}-texture`" width="4" height="4" patternUnits="userSpaceOnUse">
          <circle cx="1" cy="1" r="0.5" fill="#89939e" opacity="0.2" />
        </pattern>
      </defs>
      <ellipse cx="290" cy="185" rx="230" ry="110" :fill="`url(#${id}-glow)`" />
      <g transform="translate(100 48) rotate(-13 160 110)">
        <path d="M115 30 L132 3 L193 3 L218 30" :fill="`url(#${id}-body)`" :stroke="`url(#${id}-rim)`" />
        <path d="M20 47 L43 24 H299 L320 47 V208 L296 229 H20 Z" fill="#151c24" stroke="#73808e" />
        <rect
          x="12"
          y="42"
          width="290"
          height="190"
          rx="20"
          :fill="`url(#${id}-body)`"
          :stroke="`url(#${id}-rim)`"
          stroke-width="2"
        />
        <path d="M25 99 H90 V219 H25 Z" :fill="`url(#${id}-texture)`" />
        <path d="M258 55 H284 V218 H258 Z" :fill="`url(#${id}-texture)`" />
        <path d="M23 56 H288 M22 71 V202" fill="none" stroke="#e5edf5" stroke-opacity="0.7" />
        <rect x="42" y="26" width="33" height="12" rx="3" fill="#424e5b" stroke="#a9b5c1" />
        <path d="M48 27 V37 M55 27 V37 M62 27 V37 M69 27 V37" stroke="#98a5b2" />
        <rect x="236" y="64" width="29" height="17" rx="2" fill="#879bab" />
        <rect x="240" y="67" width="21" height="10" fill="#dbe4eb" />
        <path d="M39 82 H80" stroke="#c5ced7" stroke-width="2" />
        <circle cx="166" cy="147" r="79" fill="#070c12" :stroke="`url(#${id}-rim)`" stroke-width="4" />
        <circle cx="175" cy="150" r="69" fill="#19232f" stroke="#a6b4c3" stroke-width="2" />
        <circle cx="175" cy="150" r="61" fill="#070d16" stroke="#586b7e" stroke-width="3" />
        <circle cx="175" cy="150" r="53" :fill="`url(#${id}-glass)`" stroke="#8295a7" />
        <path d="M175 119 L201 133 L206 160 L184 179 L158 172 L147 146 Z" fill="#03080e" stroke="#344454" />
        <ellipse cx="154" cy="126" rx="22" ry="11" transform="rotate(-35 154 126)" fill="#9ab7dc" opacity="0.22" />
        <path d="M121 124 A58 58 0 0 1 184 93" fill="none" stroke="#e0e9f5" stroke-opacity="0.75" />
        <path d="M148 209 A61 61 0 0 0 231 173" fill="none" stroke="#71899f" />
        <circle cx="79" cy="209" r="3" fill="#b2e7c3" />
      </g>
    </svg>
  </header>
</template>
