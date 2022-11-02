<template>
  <nav class="header">
    <div class="logo">
      <router-link :to="{ name: 'index' }">
        <img src="/img/logo_name.svg" alt="Werbot" />
      </router-link>
    </div>
    <div class="flex-grow">
      <ProjectMenu />
    </div>
    <div>
      <router-link
        :to="{ name: 'index' }"
        class="shared"
        :class="{ current: route.name === 'index' }"
      >
        <SvgIcon name="share" class="mr-2 w-5 text-gray-700" />
        Servers<span class="hidden sm:block">&nbsp;shared with me</span>
      </router-link>
    </div>
    <div>
      <ProfileMenu />
    </div>
  </nav>
  <hr />
</template>

<script setup lang="ts">
import { useRoute } from "vue-router";
import { SvgIcon, ProjectMenu, ProfileMenu } from "@/components";

const route = useRoute();
</script>

<style lang="scss">
.header {
  @apply relative flex w-full flex-wrap items-center justify-between py-6;

  & .logo {
    @apply mr-5 w-32 flex-none cursor-pointer sm:mr-12 sm:w-36;
  }

  & .shared {
    @apply mr-4 flex items-center whitespace-nowrap text-gray-700;

    &.current {
      @apply rounded bg-gray-200 p-2 text-gray-700;

      svg {
        @apply fill-gray-700;
      }
    }
  }
}
</style>
