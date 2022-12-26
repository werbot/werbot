<template>
  <div class="artboard">
    <header>
      <h1>Project information</h1>
    </header>

    <div class="artboard-content pb-5">
      {{ data }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, getCurrentInstance } from "vue";
import { getProject } from "@/api/project";
import { Project_Request } from "@proto/project";
import { SvgIcon } from "@/components";

const { proxy } = getCurrentInstance();
const data: any = ref({});
const props = defineProps({
  projectId: String,
});

const getData = async (routeQuery: any) => {
  await getProject(routeQuery).then((res) => {
    data.value = res.data.result;
  });
};

watch(
  () => props.projectId,
  () => {
    getData(<Project_Request>{
      project_id: props.projectId,
    });
  }
);

onMounted(() => {
  getData(<Project_Request>{
    project_id: props.projectId,
  });
});
</script>
