<template>
  <div
    ref="el"
    class="grid min-h-24 w-full place-content-center rounded-md border-2 border-dashed border-primary p-3 text-center [overflow-wrap:anywhere]"
    :class="isOverDropZone ? 'bg-primary/10' : ''"
  >
    <slot />
  </div>
</template>

<script setup lang="ts">
  defineProps({
    modelValue: {
      type: Boolean,
      required: false,
    },
  });

  const emit = defineEmits(["update:modelValue", "drop"]);

  const el = ref<HTMLDivElement>();
  const { isOverDropZone } = useDropZone(el, files => {
    emit("drop", files);
  });

  watch(isOverDropZone, () => {
    emit("update:modelValue", isOverDropZone.value);
  });
</script>

<style scoped></style>
