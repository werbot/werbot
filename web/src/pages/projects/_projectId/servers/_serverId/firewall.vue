<template>
  <div class="artboard">
    <header>
      <h1>Firewall</h1>
    </header>
    <BServers :projectId="props.projectId" :serverId="props.serverId" />

    <div class="desc">
      In addition to creating your own, some add-ons come with their own.
    </div>

  </div>
</template>

<script setup lang="ts">
  import { ref, onMounted, getCurrentInstance } from "vue";
  import { useRoute } from "vue-router";
  import { BServers } from "@/components";
  
  const { proxy } = getCurrentInstance();
  const route = useRoute();
  const data: any = ref({});
  const props = defineProps({
    projectId: String,
    serverId: String,
  });
  </script>