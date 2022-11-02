<template>
  <div class="artboard">
    <header>
      <h1>
        <router-link
          :to="{
            name: 'projects-projectId-servers',
            params: {
              projectId: props.projectId,
            },
          }"
        >
          Servers
        </router-link>
        <span>Add server #1</span>
      </h1>
    </header>

    <div class="desc">step 1: description</div>

    <div class="artboard-content">
      <router-link
        :to="{
          name: 'projects-projectId-servers-add-step2',
          params: {
            projectId: props.projectId,
          },
        }"
      >
        step 2
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps({
  projectId: String,
});

document.title = "step 1";
</script>
