<template>
  <div class="px-6">
    <nav class="header">
      <div class="logo">
        <router-link :to="{ name: 'index' }">
          <img src="/img/logo_name.svg" alt="Werbot" />
        </router-link>
      </div>
    </nav>
    <hr />

    <div class="content">
      <div id="alert"></div>
      <RouterView />
    </div>
  </div>
</template>

<script setup lang="ts"></script>
