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
        <span>
          <router-link
            :to="{
              name: 'projects-projectId-servers-serverId-sessions',
              params: {
                projectId: props.projectId,
                serverId: props.serverId,
              },
            }">Sessions</router-link>
        </span>
        <span>Session</span>
      </div>
    </header>
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
  sessionId: String,
});
</script>
