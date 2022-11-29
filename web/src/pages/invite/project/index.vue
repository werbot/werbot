<template></template>

<script setup lang="ts">
import { useRouter } from "vue-router";
const router = useRouter();
router.push({ name: "index" });
</script>

<route lang="yaml">
meta:
  layout: blank
  requiresAuth: false
</route>
