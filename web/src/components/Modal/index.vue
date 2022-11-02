<template>
  <transition name="animation">
    <div class="modal" v-show="showModal">
      <div class="modal-box">
        <h1>{{ title }}</h1>
        <div class="py-4">
          <slot />
        </div>
        <slot name="footer">
          <div class="form-control">
            <button class="btn" @click="closeModal">Close</button>
          </div>
        </slot>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
const props = defineProps<{
  showModal: boolean;
  title: string;
}>();

const emit = defineEmits(["close"]);

const closeModal = () => {
  emit("close");
};
</script>

<style lang="scss" scoped>
.animation-enter-active,
.animation-leave-active {
  @apply transition;
}
.animation-enter-from,
.animation-leave-to {
  @apply opacity-0;
}

.modal {
  @apply fixed inset-0 flex justify-center;
  @apply pointer-events-auto bg-slate-900 bg-opacity-70 backdrop-blur-sm;
  @apply z-[999];
}
:where(.modal) {
  @apply items-center;
}

.modal-box {
  max-height: calc(100vh - 5em);
  @apply w-11/12 max-w-xl overflow-y-auto overscroll-contain rounded bg-white p-6 shadow-2xl;
}
</style>
