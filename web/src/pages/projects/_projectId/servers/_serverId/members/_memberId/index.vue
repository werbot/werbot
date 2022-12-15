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
              name: 'projects-projectId-servers-serverId-members',
              params: {
                projectId: props.projectId,
                serverId: props.serverId,
              },
            }">Members</router-link>
        </span>
        <span>Logs</span>
      </div>
    </header>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, getCurrentInstance } from "vue";
import { useRoute } from "vue-router";
import { SvgIcon, Modal, Toggle, BServerName, Pagination } from "@/components";

const { proxy } = getCurrentInstance();
const route = useRoute();
const data: any = ref({});
const props = defineProps({
  projectId: String,
  serverId: String,
  memberId: String,
});
</script>
