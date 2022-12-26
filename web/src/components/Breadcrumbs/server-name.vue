<template>
  <router-link :to="{ name: 'projects-projectId-servers' }"> {{ data.server_name }} </router-link>

</template>

<script lang="ts" setup>
import { ref, onMounted, getCurrentInstance } from "vue";
import { serverNameByID } from "@/api/server";
import { ServerNameByID_Request } from "@proto/server";

const { proxy } = getCurrentInstance();
const data: any = ref({});

const props = defineProps({
  memberId: {
    type: String,
    required: true,
  },
  serverId: {
    type: String,
    required: true,
  },
  projectId: {
    type: String,
    required: true,
  },
});

onMounted(() => {
  serverNameByID(<ServerNameByID_Request>{
    user_id: props.memberId,
    server_id: props.serverId,
    project_id: props.projectId,
  }).then((res) => {
    data.value = res.data.result;
  });
});
</script>
