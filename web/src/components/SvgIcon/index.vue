<template>
  <svg aria-hidden="true" :class="class">
    <use :xlink:href="symbolId" />
  </svg>
</template>

<script lang="ts" setup>
import { computed } from "vue";

const props = defineProps({
  prefix: {
    type: String,
    default: "icon",
  },
  name: {
    type: String,
    required: true,
  },
  class: {
    type: String,
  },
});

const symbolId = computed(() => `#${props.prefix}-${props.name}`);
</script>

<style lang="scss">
svg {
  @apply h-6 w-6 align-middle;
}
</style>
