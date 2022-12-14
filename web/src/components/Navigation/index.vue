<template>
  <!-- Profile block -->
  <nav class="sidebar" v-if="route.name.startsWith('profile')">
    <ul>
      <li>
        <router-link active-class="current" :to="{ name: 'profile-setting' }">
          <SvgIcon name="profile" />
          <span>Profile settings</span>
        </router-link>
      </li>
      <li>
        <router-link
          :to="{ name: 'profile-keys' }"
          :class="{ current: route.name.startsWith('profile-keys') }"
        >
          <SvgIcon name="key" />
          <span>SSH keys</span>
        </router-link>
      </li>
      <li>
        <router-link active-class="current" :to="{ name: 'profile-logs' }">
          <SvgIcon name="logs" />
          <span>Profile logs</span>
        </router-link>
      </li>
    </ul>
  </nav>

  <!-- Profile block -->
  <nav class="sidebar" v-if="route.name.startsWith('projects')">
    <ul>
      <li>
        <router-link
          :to="{
            name: 'projects-projectId-servers',
            params: { projectId: route.params.projectId },
          }"
          :class="{ current: route.name.startsWith('projects-projectId-servers') }"
        >
          <SvgIcon name="server" />
          <span>Servers</span>
        </router-link>
      </li>
      <li>
        <router-link
          :to="{
            name: 'projects-projectId-databases',
            params: { projectId: route.params.projectId },
          }"
          :class="{ current: route.name.startsWith('projects-projectId-databases') }"
        >
          <SvgIcon name="database" />
          <span>Databases</span>
        </router-link>
      </li>
      <li>
        <router-link
          :to="{
            name: 'projects-projectId-applications',
            params: { projectId: route.params.projectId },
          }"
          :class="{ current: route.name.startsWith('projects-projectId-applications') }"
        >
          <SvgIcon name="application" />
          <span>Applications</span>
        </router-link>
      </li>
      <li>
        <router-link
          :to="{
            name: 'projects-projectId-desktops',
            params: { projectId: route.params.projectId },
          }"
          :class="{ current: route.name.startsWith('projects-projectId-desktops') }"
        >
          <SvgIcon name="desktop" />
          <span>Desktops</span>
        </router-link>
      </li>
      <li>
        <router-link
          :to="{
            name: 'projects-projectId-containers',
            params: { projectId: route.params.projectId },
          }"
          :class="{ current: route.name.startsWith('projects-projectId-containers') }"
        >
          <SvgIcon name="container" />
          <span>Containers</span>
        </router-link>
      </li>
      <li>
        <router-link
          :to="{
            name: 'projects-projectId-clouds',
            params: { projectId: route.params.projectId },
          }"
          :class="{ current: route.name.startsWith('projects-projectId-clouds') }"
        >
          <SvgIcon name="cloud" />
          <span>Clouds</span>
        </router-link>
      </li>
      <li>
        <hr />
      </li>
      <li>
        <router-link
          :to="{
            name: 'projects-projectId-members',
            params: { projectId: route.params.projectId },
          }"
          :class="{ current: route.name.startsWith('projects-projectId-members') }"
        >
          <SvgIcon name="users" />
          <span>Members</span>
        </router-link>
      </li>
      <li>
        <router-link
          :to="{
            name: 'projects-projectId-teams',
            params: { projectId: route.params.projectId },
          }"
          :class="{ current: route.name.startsWith('projects-projectId-teams') }"
        >
          <SvgIcon name="teams" />
          <span>Teams</span>
        </router-link>
      </li>
      <li>
        <hr />
      </li>
      <li>
        <router-link
          :to="{ name: 'projects-projectId-logs', params: { projectId: route.params.projectId } }"
          :class="{ current: route.name.startsWith('projects-projectId-logs') }"
        >
          <SvgIcon name="logs" />
          <span>Logs activities</span>
        </router-link>
      </li>
      <li>
        <hr />
      </li>
      <li>
        <router-link
          :to="{
            name: 'projects-projectId-setting',
            params: { projectId: route.params.projectId },
          }"
          :class="{ current: route.name.startsWith('projects-projectId-setting') }"
        >
          <SvgIcon name="setting" />
          <span>Settings</span>
        </router-link>
      </li>
    </ul>
  </nav>

  <!-- Admin block -->
  <nav class="sidebar" v-if="route.name.startsWith('admin') && route.path !== '/projects/new'">
    <ul>
      <li>
        <router-link active-class="current" :to="{ name: 'admin-users' }">
          <SvgIcon name="users" />
          <span>Users</span>
        </router-link>
      </li>
      <li>
        <router-link active-class="current" :to="{ name: 'admin-projects' }">
          <SvgIcon name="project" />
          <span>Projects</span>
        </router-link>
      </li>
      <li>
        <hr />
      </li>
      <li>
        <router-link active-class="current" :to="{ name: 'admin-subscriptions' }">
          <SvgIcon name="subscriptions" />
          <span>Subscriptions</span>
        </router-link>
      </li>
      <li>
        <router-link active-class="current" :to="{ name: 'admin-license' }">
          <SvgIcon name="license" />
          <span>License</span>
        </router-link>
      </li>
      <li>
        <hr />
      </li>
      <li>
        <router-link active-class="current" :to="{ name: 'admin-settings' }">
          <SvgIcon name="setting" />
          <span>Settings</span>
        </router-link>
      </li>
    </ul>
  </nav>
</template>

<script setup lang="ts">
import { useRoute } from "vue-router";
import { SvgIcon } from "@/components";

const route = useRoute();
</script>

<style lang="scss">
.sidebar {
  @apply mb-6;

  li {
    @apply mb-4 w-3 sm:w-36;

    a {
      @apply leading-5 text-gray-400;

      &:hover {
        @apply -m-2 rounded bg-gray-100 p-2;
      }

      span {
        @apply hidden sm:inline-block;
      }

      svg {
        @apply inline-block w-5 fill-gray-400 pr-0 align-middle sm:mr-2.5;
      }

      &.current {
        @apply -m-2 rounded bg-gray-200 p-2 text-gray-700;

        svg {
          @apply fill-gray-700;
        }
      }
    }
  }
}
</style>
