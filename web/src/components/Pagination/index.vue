<template>
  <div v-if="isLength(pagination) > 1" class="pagination">
    <router-link
      v-for="(item, index) in pagination"
      class="pagination btn"
      :to="{
        name: route.name,
        query: isQuery(index),
      }"
      :class="item == 'active' ? 'btn-active' : ''"
      @click="onSelectPage(isQuery(index))"
    >
      {{ index }}
    </router-link>
  </div>
</template>

<script lang="ts" setup>
import { computed } from "vue";
import { useRoute } from "vue-router";

const props = defineProps({
  total: {
    type: Number,
    default: 0,
  },
});

const emits = defineEmits(["selectPage"]);

const route = useRoute();
const isLimit = (): number => {
  let limit = 10;
  if (route.query.limit) {
    limit = Number(route.query.limit);
  }
  return limit;
};

const isOffset = (): number => {
  let offset = 0;
  if (route.query.offset) {
    offset = Number(route.query.offset);
  }
  return offset;
};

const isQuery = (index: Number) => {
  const offset: number = (Number(index) - 1) * isLimit();
  return {
    limit: isLimit(),
    offset: offset,
  };
};

const isLength = (obj: {}): Number => {
  return Object.keys(obj).length;
};

const pagination = computed((): {} => {
  const res: any = {};
  const totalPage = Math.ceil(props.total / isLimit());
  const selectPage = isOffset() / isLimit() + 1;

  if (totalPage > 0) {
    var count = 1;
    for (let i = 0; i < totalPage; i++) {
      if (selectPage === count) {
        res[count] = "active";
      } else {
        res[count] = "";
      }
      count++;
    }
  }
  return res;
});

const onSelectPage = (query: any) => {
  emits("selectPage", query);
};
</script>

<style lang="scss" scoped>
.pagination {
  @apply my-3 flex justify-center;

  &.btn {
    @apply mr-2 inline-block rounded bg-gray-200 py-1.5 px-3 text-gray-700;

    &-active {
      @apply bg-gray-700 text-gray-200;
    }
  }
}
</style>
