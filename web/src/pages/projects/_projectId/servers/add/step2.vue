<template>
  <div class="artboard">
    <header>
      <h1>Servers</h1>
      <div class="breadcrumbs">Step 2</div>
    </header>

    <div class="desc">step 2: description</div>

    <div class="artboard-content p-5">
      <router-link
        :to="{
          name: 'projects-projectId-servers-add-step3',
          params: {
            projectId: props.projectId,
          },
        }"
      >
        step 3
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps({
  projectId: String,
});

document.title = "step 2";
</script>
