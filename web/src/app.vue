<template>
  <RouterView />
  <Alert />
</template>

<script setup lang="ts">
import { Alert } from "@/components";
document.title = "Werbot";
</script>