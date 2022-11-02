<template></template>

<script setup lang="ts">
const props = defineProps<{
  showDrawer: boolean;
  title: string;
}>();

const emit = defineEmits(["close"]);

const closeDrawer = () => {
  emit("close");
};
</script>

<style lang="scss" scoped></style>
