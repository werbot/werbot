<template>
  <div class="artboard">
    <header>
      <h1>Setting</h1>
    </header>
    <BServers :projectId="props.projectId" :serverId="props.serverId" />

    <div class="desc">
      Several options for adding new members are available. Choose the right one and follow the
      instructions.
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, getCurrentInstance } from "vue";
import { useRoute } from "vue-router";
import { BServers } from "@/components";

const { proxy } = getCurrentInstance();
const route = useRoute();
const data: any = ref({});
const props = defineProps({
  projectId: String,
  serverId: String,
});
</script>
