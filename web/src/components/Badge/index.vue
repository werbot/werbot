<template>
  <span class="badge" :class="classBadge">
    {{ name }}
  </span>
</template>

<script lang="ts" setup>
const props = defineProps({
  name: {
    type: String,
    required: true,
  },
  color: {
    type: String,
    default: "gray",
  },
  class: {
    type: String,
  },
});

const classBadge = "badge-" + props.color + " " + props.class;
</script>

<style lang="scss">
.badge {
  @apply mr-2 rounded px-2.5 py-0.5 text-sm font-medium;

  &-gray {
    @apply bg-gray-200 text-gray-700;
  }

  &-blue {
    @apply bg-blue-200 text-blue-700;
  }

  &-red {
    @apply bg-red-200 text-red-700;
  }

  &-green {
    @apply bg-green-200 text-green-700;
  }

  &-yellow {
    @apply bg-yellow-200 text-yellow-700;
  }

  &-indigo {
    @apply bg-indigo-200 text-indigo-700;
  }

  &-purple {
    @apply bg-purple-200 text-purple-700;
  }

  &-pink {
    @apply bg-pink-200 text-pink-700;
  }
}
</style>
