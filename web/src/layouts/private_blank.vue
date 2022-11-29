<template>
  <div class="px-6">
    <Header />
    <div class="content">
      <div id="alert"></div>
      <RouterView />
    </div>
  </div>
</template>

<script setup lang="ts">
import { Header } from "@/components";
</script>
