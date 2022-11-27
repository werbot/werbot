<template>
  <div class="artboard">
    <header>
      <h1>Invites</h1>
      <router-link :to="{ name: 'projects-projectId-members-add' }">
        <label class="plus">
          <SvgIcon name="plus_square" />
          add new
        </label>
      </router-link>
    </header>

    <Tabs :tabs="tabMenu" />

    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Email</th>
          <th class="w-40">Invited</th>
          <th class="w-24">Status</th>
          <th class="w-8"></th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>Test user</td>
          <td>test@email.com</td>
          <td>2019-01-18 18:41:18</td>
          <td><Badge name="activated" /></td>
          <td>
            <router-link
              active-class="current"
              :to="{
                name: 'projects-projectId-members',
                params: {
                  projectId: props.projectId,
                },
              }"
            >
              <SvgIcon name="delete" class="cursor-pointer text-red-500" />
            </router-link>
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
import { onMounted, ref, getCurrentInstance, onBeforeUnmount } from "vue";
import { useRoute } from "vue-router";
import { SvgIcon, Pagination, Tabs, Badge } from "@/components";

const { proxy } = getCurrentInstance();
const route = useRoute();
const data: any = ref({});
const props = defineProps({
  projectId: String,
});

// Tabs section
const tabMenu = [
  {
    name: "Members",
    link: { name: "projects-projectId-members" },
  },
  {
    name: "Invites",
    link: { name: "projects-projectId-members-invites" },
  },
];

const getData = async (routeQuery: any) => {
  if (proxy.$authStore.hasUserRole === 3) {
    routeQuery.member_id = proxy.$authStore.hasUserID;
  }
  routeQuery.project_id = props.projectId;
  //await getMembers(routeQuery.member_id, routeQuery.project_id, routeQuery).then((res) => {
  //  data.value = res.data.result;
  //});
};

const onSelectPage = (e) => {
  getData(e);
};

onMounted(() => {
  getData(route.query);
});

/*
const searchUser = async () => {
  if (proxy.$authStore.hasUserRole === 3) {
    routeQuery.member_id = proxy.$authStore.hasUserID;
  }

  data.value.project_id = props.projectId;
  data.value.name = data.value.member;

  await getUsersWithoutProject(data.value).then((res) => {
    users.value = res.data.result;
  });
};

const selectUser = async (id: Number) => {
  data.value.member = users.value.users[Number(id)].name;
  data.value.user_id = users.value.users[Number(id)].user_id;
};

const onAdd = async () => {
  data.value.owner_id = Number(props.userId);
  data.value.project_id = Number(props.projectId);
  delete data.value.name;
  delete data.value.member;

  await postMember(data.value).then((res) => {
    if (res.data.success) {
      const eventError = new CustomEvent("connextSuccess", {
        detail: res.data.message,
      });
      dispatchEvent(eventError);
    }
  });

  router.push({ name: "users-userId-projects-projectId-members" });
};
*/

onBeforeUnmount(() => proxy.$errorStore.$reset());

document.title = "add member";
</script>
