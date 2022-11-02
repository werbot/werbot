<template>
  <div class="form-control" :class="class">
    <label class="label">
      <span v-if="name" class="text">{{ name }}{{ required ? "*" : "" }}</span>
      <span v-if="error" class="error">
        {{ error }}
      </span>
    </label>

    <textarea
      v-model="value"
      :class="error ? 'error' : ''"
      :disabled="disabled"
      :placeholder="placeholder"
      :rows="rows"
    ></textarea>
  </div>
</template>

<script lang="ts" setup>
import { computed } from "vue";

const props = defineProps({
  name: {
    type: String,
  },
  modelValue: {
    required: true,
  },
  rows: {
    type: String,
    default: 6,
  },
  error: {
    type: String,
  },
  class: {
    type: String,
  },
  disabled: {
    type: Boolean,
  },
  required: {
    type: Boolean,
  },
  placeholder: {
    type: String,
  },
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
