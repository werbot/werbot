<template>
  <div class="artboard">
    <header>
      <h1>Users</h1>
    </header>

    <div class="desc">Description</div>
    <div class="artboard-content p-5">section under development</div>
  </div>
</template>

<script setup lang="ts">
document.title = "Users";
</script>

<route lang="yaml">
meta:
  requiresAuth: true
</route>