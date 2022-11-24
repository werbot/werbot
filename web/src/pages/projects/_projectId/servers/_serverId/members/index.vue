<template>
  <div class="artboard">
    <header>
      <h1>Member list</h1>
      <router-link :to="{ name: 'projects-projectId-servers-serverId-members-add' }">
        <label class="plus">
          <SvgIcon name="plus_square" />
          add new
        </label>
      </router-link>
    </header>
    <BServers :projectId="props.projectId" :serverId="props.serverId" />

    <table>
      <thead>
        <tr>
          <th class="w-12"></th>
          <th>Member</th>
          <th>Last activity</th>
          <th class="w-20">Activity</th>
          <th class="w-8"></th>
          <th class="w-8"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(item, index) in data.members" :key="index">
          <td>
            <div class="flex items-center">
              <span class="dot bg-green-500"></span>
            </div>
          </td>
          <td>{{ item.user_name }}</td>
          <td>{{ item }}</td>
          <td>
            <div class="flex items-center">
              <Toggle
                v-model="item.active"
                :id="index"
                @change="changeMemberActive(index, item.active)"
              />
            </div>
          </td>
          <td>
            <SvgIcon name="logs" class="text-gray-700" />
          </td>
          <td>
            <SvgIcon name="setting" class="text-gray-700" />
          </td>
        </tr>
      </tbody>
    </table>

    <div class="artboard-content">
      <Pagination :total="data.total" @selectPage="onSelectPage" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, getCurrentInstance } from "vue";
import { useRoute } from "vue-router";
import { SvgIcon, Toggle, BServers } from "@/components";
import { showMessage } from "@/utils/message";

import { getServerMembers, updateServerMemberStatus } from "@/api/member/server";
import { UpdateServerMemberStatus_Request } from "@proto/member/member";
import { RoleUser } from "@proto/user/user";

const { proxy } = getCurrentInstance();
const route = useRoute();
const data: any = ref({});
const props = defineProps({
  projectId: String,
  serverId: String,
});

const getData = async (routeQuery: any) => {
  if (proxy.$authStore.hasUserRole === 3) {
    routeQuery.member_id = proxy.$authStore.hasUserID;
  }
  routeQuery.project_id = props.projectId;
  routeQuery.server_id = props.serverId;
  await getServerMembers(
    routeQuery.member_id,
    routeQuery.project_id,
    routeQuery.server_id,
    routeQuery
  ).then((res) => {
    data.value = res.data.result;
  });
};

const onSelectPage = (e) => {
  getData(e);
};

onMounted(() => {
  getData(route.query);
});

const changeMemberActive = async (index: number, online: boolean) => {
  const status = !online;
  data.value.members[Number(index)].active = status;

  await updateServerMemberStatus(<UpdateServerMemberStatus_Request>{
    owner_id: proxy.$authStore.hasUserID,
    project_id: props.projectId,
    member_id: data.value.members[Number(index)].member_id,
    server_id: props.serverId,
    status: status,
  })
    .then((res) => {
      if (!status) {
        showMessage(res.data.message, "connextWarning");
      } else {
        showMessage(res.data.message);
      }

      proxy.$errorStore.$reset();
    })
    .catch((err) => {
      showMessage(err.response.data.message, "connextError");
    });
};
</script>
