<template>
  <div class="artboard">
    <header>
      <h1>Servers</h1>
      <div class="breadcrumbs">Step 3</div>
    </header>

    <div class="desc">step 3: description</div>

    <div class="artboard-content p-5">
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
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps({
  projectId: String,
});

document.title = "step 3";
</script>
