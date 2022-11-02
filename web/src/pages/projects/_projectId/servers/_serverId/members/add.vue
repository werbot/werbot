<template>
  <div class="artboard">
    <header>
      <h1>
        <router-link
          :to="{
            name: 'projects-projectId-servers-serverId-members',
            params: {
              projectId: props.projectId,
              serverId: props.serverId,
            },
          }"
        >
          Member list
        </router-link>
        <span>Add new</span>
      </h1>
    </header>
    <BServers :projectId="props.projectId" :serverId="props.serverId" />

    <div class="desc">
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

    <div class="artboard-content">content</div>
  </div>
</template>

<script setup lang="ts">
import { ref, onBeforeUnmount, getCurrentInstance } from "vue";
import { useRoute } from "vue-router";
import { FormInput, FormTextarea, BServers } from "@/components";

const { proxy } = getCurrentInstance();
const data: any = ref({});
const loading = ref(false);
const route = useRoute();

const props = defineProps({
  projectId: String,
  serverId: String,
});
</script>
