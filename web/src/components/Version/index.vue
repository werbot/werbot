<template>
  <p class="versions">
  <div>ui: {{ $systemStore.versions.ui }}</div>
  <div>api: {{ $systemStore.versions.api }}</div>
  </p>
</template>

<style lang="scss">
.versions {
  @apply text-gray-200 text-xs hidden sm:inline-block;
}
</style>