<template>
  <div class="-mt-3 pl-6 pb-6">
    <div class="flex flex-row">
      <div class="flex w-20">Project ID:</div>
      <Badge :name="props.projectId" />
      <router-link :to="{ name: 'projects-projectId-servers' }">
        <Badge name="Servers" color="blue" />
      </router-link>
    </div>

    <div class="flex flex-row pt-1">
      <div class="flex w-20">Server ID:</div>
      <Badge :name="props.serverId" />
      {{props.sessionId}}
      <router-link :to="{ name: 'projects-projectId-servers-serverId-session' }">
        <Badge name="All sessions" color="blue" />
      </router-link>
    </div>

    <div class="flex flex-row pt-1" v-if="props.memberId">
      <div class="flex w-20">Member ID:</div>
      <Badge :name="props.memberId" />
      <router-link :to="{ name: 'projects-projectId-servers-serverId-members' }">
        <Badge name="Members" color="blue" />
      </router-link>

      <router-link :to="{ name: 'projects-projectId-servers-serverId-members' }" v-if="props.sessionId">
        <Badge name="Sessions" color="blue" />
      </router-link>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { Badge } from "@/components";

const props = defineProps({
  projectId: {
    type: String,
    required: true,
  },
  serverId: {
    type: String,
    required: true,
  },
  memberId: {
    type: String,
  },
  sessionId: {
    type: String,
  },
});
</script>
