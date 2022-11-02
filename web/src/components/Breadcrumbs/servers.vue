<template>
  <div class="-mt-3 pl-6 pb-6">
    <div class="flex flex-row">
      <div class="flex w-20">Project ID:</div>
      <div>
        <router-link :to="{ name: 'projects-projectId-servers' }">
          <Badge :name="props.projectId" color="blue" />
        </router-link>
      </div>
    </div>
    <div class="flex flex-row pt-1">
      <div class="flex w-20">Server ID:</div>
      <div v-if="props.sessionId">
        <router-link :to="{ name: 'projects-projectId-servers-serverId-session' }">
          <Badge :name="props.serverId" color="blue" />
        </router-link>
      </div>
      <div v-else><Badge :name="props.serverId" /></div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { SvgIcon, Badge } from "@/components";

const props = defineProps({
  projectId: {
    type: String,
    required: true,
  },
  serverId: {
    type: String,
    required: true,
  },
  sessionId: {
    type: String,
  },
});
</script>
