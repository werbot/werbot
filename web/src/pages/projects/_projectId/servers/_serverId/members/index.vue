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
          <th></th>
          <th>Member</th>
          <th>Last activity</th>
          <th>Activity</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>
            <div class="flex items-center">
              <span class="dot bg-green-500"></span>
            </div>
          </td>
          <td>user name</td>
          <td>login_success</td>
          <td></td>
          <td></td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, getCurrentInstance } from "vue";
import { useRoute } from "vue-router";
import { SvgIcon, BServers } from "@/components";

const { proxy } = getCurrentInstance();
const route = useRoute();
const data: any = ref({});
const props = defineProps({
  projectId: String,
  serverId: String,
});
</script>
