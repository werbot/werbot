<template>
  <img class="mb-8 w-32" src="/img/logo_mini.svg" alt="Werbot" />
  <div class="card w-[22rem]">
    <span class="title">Reset password</span>
    <form @submit.prevent v-if="!data.message">
      <FormInput
        name="Email"
        v-model.trim="data.email"
        :error="proxy.$errorStore.errors['email']"
        :disabled="loading"
      />

      <div class="form-control mt-6">
        <button type="submit" class="btn" @click="onSubmit" :disabled="loading">
          <div v-if="loading">
            <span>Loading...</span>
          </div>
          <span v-else>Send me message</span>
        </button>
      </div>
    </form>

    <div v-if="data.message === 'Verification email has been sent'">
      <span class="message"
        >Bad An email has been sent. It contains a link you must click to reset your password.</span
      >
      <span class="message">Note: You can only request a new password once within 24 hours.</span>
      <span class="message">If you don't get an email check your spam folder or try again.</span>
    </div>

    <div v-if="data.message === 'Resend only after 24 hours'">
      <span class="message"
        >In the last 24 hours, you have already been sent a password reset email</span
      >
      <span class="message">If you don't get an email check your spam folder or try again.</span>
    </div>
  </div>

  <div class="mt-10">
    <p>
      Already have an account?
      <router-link :to="{ name: 'auth-signin' }"> Sign in </router-link>
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, onBeforeUnmount, getCurrentInstance } from "vue";
import { FormInput } from "@/components";
import { postSendEmail } from "@/api/auth";

const { proxy } = getCurrentInstance();
const data: any = ref({});
const loading = ref(false);

const onSubmit = async () => {
  loading.value = !loading.value;

  // @ts-ignore
  await postSendEmail(data.value.email)
    .then((res) => {
      data.value = res.data.result;
    })
    .catch(() => (loading.value = !loading.value));
};

onBeforeUnmount(() => proxy.$errorStore.$reset());

document.title = "Reset password";
</script>

<route lang="yaml">
meta:
  layout: auth
  requiresAuth: false
</route>
