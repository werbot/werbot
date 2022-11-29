<template>
  <img class="mb-8 w-32" src="/img/logo_mini.svg" alt="Werbot" />
  <div class="card w-[22rem]">
    <span class="title">Sign Up</span>
    <form @submit.prevent>
      <FormInput
        name="Email"
        placeholder='user@werbot.net'
        disabled
      />
      <FormInput
        name="Password"
        v-model.trim="data.password"
        :error="$errorStore.errors['password']"
        type="password"
        autocomplete="current-password"
        :disabled="loading"
      />
      <FormInput
        name="Re-Type Password"
        v-model.trim="data.password2"
        :error="$errorStore.errors['password2']"
        type="password"
        autocomplete="current-password"
        :disabled="loading"
      />

      <div class="form-control mt-6">
        <button type="submit" class="btn" @click="onSubmit" :disabled="loading">
          <div v-if="loading">
            <span>Loading...</span>
          </div>
          <span v-else>Registration</span>
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, onBeforeUnmount, getCurrentInstance } from "vue";
import { useRouter } from "vue-router";
import { FormInput } from "@/components";

const { proxy } = getCurrentInstance();

const data: any = ref({});
const loading = ref(false);
const router = useRouter();

const onSubmit = async () => {
  //loading.value = !loading.value;
};

onMounted(async () => {
  if (proxy.$authStore.loggedIn) {
    router.push({ name: "index" });
  }
});

onBeforeUnmount(() => proxy.$errorStore.$reset());

document.title = "Sign Up";
</script>

<route lang="yaml">
meta:
  layout: auth
  requiresAuth: false
</route>
