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
        <span>Add server #3</span>
      </h1>
    </header>

    <div class="desc">step 3: description</div>

    <div class="artboard-content">content</div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps({
  projectId: String,
});

document.title = "step 3";
</script>
