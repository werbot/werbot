<template>
  <div class="relative" v-click-outside="closeDropdown">
    <button class="dropdown" type="button" @click="toggleDropdown">
      <SvgIcon name="user" />
      <span class="hidden md:block">{{ $authStore.hasUserName }}</span>
      <SvgIcon name="row_down" />
    </button>

    <ul v-show="isDropdownOpen" class="dropdown-menu right-0" @click="closeDropdown">
      <li>
        <router-link active-class="current" :to="{ name: 'profile-setting' }">
          <SvgIcon name="profile" />
          <span>Profile settings</span>
        </router-link>
      </li>
      <li>
        <router-link active-class="current" :to="{ name: 'profile-keys' }">
          <SvgIcon name="key" />
          <span>SSH Keys</span>
        </router-link>
      </li>
      <li>
        <router-link active-class="current" :to="{ name: 'profile-logs' }">
          <SvgIcon name="logs" />
          <span>Profile logs</span>
        </router-link>
      </li>
      <li v-if="$authStore.hasUserRole === 3">
        <hr />
        <router-link active-class="current" :to="{ name: 'admin' }">
          <SvgIcon name="admin" />
          <span>Admin</span>
        </router-link>
      </li>
      <li>
        <hr />
        <router-link active-class="current" :to="{ name: 'auth-logout' }">
          <SvgIcon name="logout" />
          <span>Logout</span>
        </router-link>
      </li>
    </ul>
  </div>
</template>

<script lang="ts" setup>
import { ref, watch } from "vue";
import { SvgIcon } from "@/components";
// @ts-ignore
import { directive as vClickOutside } from "click-outside-vue3";

const props = defineProps<{
  isLoading?: boolean;
}>();

const isDropdownOpen = ref(false);

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
</script>
