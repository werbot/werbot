<template>
  <img class="mb-8 w-32" src="/img/logo_mini.svg" alt="Werbot" />
  <div class="card w-[22rem]">
    <span class="title">Sign In</span>
    <form @submit.prevent>
      <FormInput
        name="Email"
        v-model.trim="data.email"
        :error="$errorStore.errors['email']"
        autocomplete="username"
        :disabled="loading"
      />
      <FormInput
        name="Password"
        v-model.trim="data.password"
        :error="$errorStore.errors['password']"
        type="password"
        autocomplete="current-password"
        :disabled="loading"
      />

      <!--
      <div class="mt-2 flex items-center justify-between">
        <label class="label cursor-pointer">
          <input type="checkbox" class="checkbox" :disabled="loading" />
          <span>Remember me</span>
        </label>
        <router-link :to="{ name: 'auth-password_reset' }" class="mt-2">
          Forgot password?
        </router-link>
      </div>
-->
      <div class="form-control mt-6">
        <button type="submit" class="btn" @click="onSubmit" :disabled="loading">
          <div v-if="loading">
            <span>Loading...</span>
          </div>
          <span v-else>Login</span>
        </button>
      </div>
    </form>
  </div>

  <div class="mt-10">
    <router-link :to="{ name: 'auth-password_reset' }" class="mt-2"> Forgot password? </router-link>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, onBeforeUnmount, getCurrentInstance } from "vue";
import { useRouter } from "vue-router";
import { SignIn_Request } from "@proto/user";
import { FormInput } from "@/components";

const { proxy } = getCurrentInstance();

const data: any = ref({});
const loading = ref(false);
const router = useRouter();

const onSubmit = async () => {
  loading.value = !loading.value;

  proxy.$authStore
    .login(<SignIn_Request>{
      email: data.value.email,
      password: data.value.password,
    })
    .then(() => {
      if (proxy.$systemStore.invites.project) {
        const invite = proxy.$systemStore.invites.project;
        router.push({ name: "invite-project-invite", params: { invite } });
        return;
      }
      router.push({ name: "index" });
    })
    .catch(() => (loading.value = !loading.value));
};

onMounted(async () => {
  //proxy.$authStore.user.user_id
  if (proxy.$authStore.loggedIn) {
    router.push({ name: "index" });
  }
});

onBeforeUnmount(() => proxy.$errorStore.$reset());

document.title = "Sign In";
</script>

<route lang="yaml">
meta:
  layout: auth
  requiresAuth: false
</route>
