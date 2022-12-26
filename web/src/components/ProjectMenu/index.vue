<template>
  <div class="relative" v-click-outside="closeDropdown">
    <button class="dropdown" type="button" @click="toggleDropdown">
      <SvgIcon name="project" />
      <span class="hidden md:block">Projects</span>
      <SvgIcon name="row_down" />
    </button>

    <ul v-show="isDropdownOpen" class="dropdown-menu" @click="closeDropdown">
      <li v-for="(item, index) in data.projects" :key="index">
        <router-link
          active-class="current"
          :to="{ name: 'projects-projectId', params: { projectId: item.project_id } }"
        >
          <SvgIcon name="project" />
          <span>{{ item.title }}</span>
        </router-link>
      </li>
      <div v-if="data.total > 5">
        <li>
          <hr />
        </li>
        <li>
          <router-link :to="{ name: 'projects' }">
            <SvgIcon name="project" />
            <span>Show all projects</span>
          </router-link>
        </li>
      </div>
      <li>
        <hr />
      </li>
      <li>
        <router-link active-class="current" :to="{ name: 'projects-add' }">
          <SvgIcon name="plus" />
          <span>Add new project</span>
        </router-link>
      </li>
    </ul>
  </div>
</template>

<script lang="ts" setup>
import { ref, watch, onMounted, getCurrentInstance } from "vue";
import { SvgIcon } from "@/components";
import { getProjects } from "@/api/project";
import { ListProjects_Request } from "@proto/project";
// @ts-ignore
import { directive as vClickOutside } from "click-outside-vue3";

const { proxy } = getCurrentInstance();
const data: any = ref({});
const isDropdownOpen = ref(false);
const props = defineProps<{
  isLoading?: boolean;
}>();

const openDropdown = () => {
  if (props.isLoading) return false;
  isDropdownOpen.value = true;
};
const closeDropdown = () => {
  isDropdownOpen.value = false;
};
const toggleDropdown = () => {
  isDropdownOpen.value ? closeDropdown() : openDropdown();
};

watch(
  () => props.isLoading,
  (isLoading) => {
    if (isLoading) {
      closeDropdown();
    }
  }
);

onMounted(async () => {
  await getProjects(
    <ListProjects_Request>{
      limit: 5,
    }
    //proxy.$authStore.hasUserID
  ).then((res) => {
    data.value = res.data.result;
  });
});
</script>
