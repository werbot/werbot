<template>
  <div role="status">Please wait...</div>
</template>

<script setup lang="ts">
import { onMounted, getCurrentInstance } from "vue";
import { useRouter } from "vue-router";

const { proxy } = getCurrentInstance();
const router = useRouter();

document.title = `Please wait...`;

onMounted(async () => {
  try {
    await proxy.$authStore.logout();
  } finally {
    router.push({ name: "auth-signin" });
  }
});
</script>

<route lang="yaml">
meta:
  layout: auth
  requiresAuth: true
</route>