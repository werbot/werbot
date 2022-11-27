<template>
  <div class="artboard">
    <header>
      <h1>Member logs</h1>
    </header>
    <BServers :projectId="props.projectId" :serverId="props.serverId" :memberId="props.memberId" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, getCurrentInstance } from "vue";
import { useRoute } from "vue-router";
import { SvgIcon, Modal, Toggle, BServers, Pagination } from "@/components";

const { proxy } = getCurrentInstance();
const route = useRoute();
const data: any = ref({});
const props = defineProps({
  projectId: String,
  serverId: String,
  memberId: String,
});
</script>
