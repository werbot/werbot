<template>
  <div class="artboard">
    <header>
      <h1>Servers</h1>
      <div class="breadcrumbs">
        <BServerName
          :memberId="proxy.$authStore.hasUserID"
          :serverId="props.serverId"
          :projectId="props.projectId"
        />
        <span>Firewall</span>
      </div>
    </header>

    <div class="desc">
      In addition to creating your own, some add-ons come with their own.
    </div>

  </div>
</template>

<script setup lang="ts">
  import { ref, onMounted, getCurrentInstance } from "vue";
  import { useRoute } from "vue-router";
  import { BServerName } from "@/components";
  
  const { proxy } = getCurrentInstance();
  const route = useRoute();
  const data: any = ref({});
  const props = defineProps({
    projectId: String,
    serverId: String,
  });
  </script>