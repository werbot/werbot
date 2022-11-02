<template>
  <div>
    <h1>404 - Page Not Found</h1>
    <p>This page no longer exists or was moved to another location.</p>
  </div>
</template>

<script setup lang="ts">
document.title = "404 - Page Not Found";
</script>

<route lang="yaml">
name: "error404"
meta:
  layout: 404
  requiresAuth: false
</route>
