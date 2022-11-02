<template>
  <ul
    class="flex flex-wrap bg-gray-100 pl-3 text-center text-gray-700"
    id="mainTab"
    data-tabs-toggle="#tabContent"
    role="tablist"
  >
    <li v-for="(item, index) in props.tabs" :key="index">
      <router-link
        :id="'tab-' + index"
        :data-tabs-target="'#' + index"
        type="button"
        role="tab"
        :aria-controls="index"
        :aria-selected="(active = item.link.name === route.name)"
        class="inline-block p-4 focus:outline-none text-gray-700"
        :class="[active ? 'bg-gray-200' : 'hover:bg-gray-800 hover:text-gray-200']"
        :to="item.link"
      >
        {{ item.name }}
      </router-link>
    </li>
  </ul>
</template>

<script lang="ts" setup>
import { useRoute } from "vue-router";

const route = useRoute();
const props = defineProps({
  tabs: {
    type: Object,
    required: true,
  },
});
</script>
