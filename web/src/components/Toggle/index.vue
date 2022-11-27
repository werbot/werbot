<template>
  <label :for="`toggle_` + id" class="toggle">
    <input
      type="checkbox"
      :id="`toggle_` + id"
      class="peer sr-only"
      :checked="Boolean(value)"
      :disabled="disabled"
    />
    <div
      class="peer h-6 w-11 rounded-full bg-gray-200 after:absolute after:top-[2px] after:left-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-green-500 peer-checked:after:translate-x-full peer-checked:after:border-white"
    ></div>
    <span v-if="name" class="ml-3 text-sm font-medium">{{ name }}</span>
  </label>
</template>

<script lang="ts" setup>
import { computed } from "vue";

const props = defineProps({
  name: {
    type: String,
  },
  modelValue: {
    required: false,
  },
  disabled: {
    type: Boolean,
    default: false,
  },
  id: {},
});

const emits = defineEmits(["update:modelValue"]);
const value = computed({
  get: () => {
    return props.modelValue;
  },
  set: (val) => {
    emits("update:modelValue", val);
  },
});
</script>

<style lang="scss">
.toggle {
  @apply relative inline-flex cursor-pointer select-none items-center;
}
</style>
