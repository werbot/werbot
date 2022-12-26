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
            }">Members</router-link
          >
        </span>
        <span>Add new</span>
      </div>
    </header>

    <table v-if="data.total > 0">
      <thead>
        <tr>
          <th>Member</th>
          <th class="w-20">Status</th>
          <th class="w-20">Add</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(item, index) in data.members" :key="index">
          <td>{{ item.user_name }}</td>
          <td>
            <Badge v-if="item.active" name="online" color="green" />
            <Badge v-else name="offline" color="red" />
          </td>
          <td>
            <div class="flex items-center">
              <SvgIcon name="plus_square" @click="addingMember(index)" class="cursor-pointer" />
            </div>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-else class="desc">Empty</div>

    <div class="artboard-content">
      <Pagination :total="data.total" @selectPage="onSelectPage" />
    </div>
  </div>

  <div class="m-6">
    In order to add a new member, he must first be invited to the general list of
    <router-link
      :to="{
        name: 'projects-projectId-members',
        params: {
          projectId: props.projectId,
        },
      }"
    >
      project members </router-link
    >.
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, getCurrentInstance } from "vue";
import { useRoute } from "vue-router";
import { SvgIcon, Toggle, BServerName, Badge, Pagination } from "@/components";
import { showMessage } from "@/utils/message";

import { getMembersWithoutServer, postServerMember } from "@/api/member/server";
import { MembersWithoutServer_Request, AddServerMember_Request } from "@proto/member";

const { proxy } = getCurrentInstance();
const data: any = ref({});
const loading = ref(false);
const route = useRoute();

const props = defineProps({
  projectId: String,
  serverId: String,
});

const getData = async (routeQuery: any) => {
  if (proxy.$authStore.hasUserRole === 3) {
    routeQuery.owner_id = proxy.$authStore.hasUserID;
  }
  routeQuery.project_id = props.projectId;
  routeQuery.server_id = props.serverId;
  await getMembersWithoutServer(<MembersWithoutServer_Request>{
    limit: routeQuery.limit,
    offset: routeQuery.offset,
    owner_id: routeQuery.owner_id,
    project_id: routeQuery.project_id,
    server_id: routeQuery.server_id,
    name: "",
  }).then((res) => {
    data.value = res.data.result;
  });
};

const onSelectPage = (e) => {
  getData(e);
};

onMounted(() => {
  getData(route.query);
});

const addingMember = async (index: number) => {
  var active = data.value.members[Number(index)].active;
  if (!active) {
    active = false;
  }

  await postServerMember(<AddServerMember_Request>{
    owner_id: proxy.$authStore.hasUserID,
    project_id: props.projectId,
    server_id: props.serverId,
    member_id: data.value.members[Number(index)].member_id,
    active: active,
  })
    .then((res) => {
      if (res.data.success) {
        data.value.members.splice(index, 1);
        data.value.total = data.value.total - 1;

        showMessage(res.data.message);
        proxy.$errorStore.$reset();
      }
    })
    .catch((err) => {
      showMessage(err.response.data.message, "connextError");
      proxy.$errorStore.$reset();
    });
};
</script>
