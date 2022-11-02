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
        <span>Add server #2</span>
      </h1>
    </header>

    <div class="desc">step 2: description</div>

    <div class="artboard-content">
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
