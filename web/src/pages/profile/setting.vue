<template>
  <div class="artboard">
    <header>
      <h1>Profile setting</h1>
    </header>
    <div class="divider"></div>
    <div class="artboard-content">
      <form @submit.prevent>
        <div class="mb-5 flex flex-row">
          <FormInput
            name="Email"
            v-model.trim="data.email"
            :error="proxy.$errorStore.errors['email']"
            :disabled="loading"
            class="mr-5 flex-grow"
          />
          <FormInput
            name="Full name"
            v-model.trim="data.fio"
            :error="proxy.$errorStore.errors['fio']"
            :disabled="loading"
            class="flex-grow"
          />
        </div>

        <div class="my-6">
          <button type="submit" class="btn" @click="onUpdateProfile" :disabled="loading">
            <div v-if="loading">
              <span>Loading...</span>
            </div>
            <span v-else>Update profile</span>
          </button>
        </div>
      </form>
    </div>

    <header class="mt-3">
      <h1>Password update</h1>
    </header>
    <div class="divider"></div>
    <div class="artboard-content">
      <form @submit.prevent>
        <FormInput
          name="Current Password"
          v-model.trim="data.old_password"
          :error="proxy.$errorStore.errors['old_password']"
          :disabled="loading"
          class="w-80"
          type="password"
          autocomplete="current-password"
        />

        <div class="mb-5 flex flex-row">
          <FormInput
            name="New password"
            v-model.trim="data.new_password"
            :error="proxy.$errorStore.errors['new_password']"
            :disabled="loading"
            class="mr-5 flex-grow"
            type="password"
            autocomplete="new-password"
          />
          <FormInput
            name="Re-Type New Password"
            v-model="data.new_password2"
            :error="proxy.$errorStore.errors['new_password2']"
            :disabled="loading"
            class="flex-grow"
            type="password"
            autocomplete="new-password"
          />
        </div>

        <div class="my-6">
          <button type="submit" class="btn" @click="onUpdatePassword" :disabled="loading">
            <div v-if="loading">
              <span>Loading...</span>
            </div>
            <span v-else>Update password</span>
          </button>
        </div>
      </form>
    </div>
  </div>

  <div class="artboard-red">
    <header>
      <h1>Delete Your Account</h1>
    </header>
    <div class="desc">
      This operation will permanently delete your user account. It CAN NOT be undone.
    </div>
    <div class="artboard-content">
      <form @submit.prevent>
        <FormInput
          name="Password"
          v-model="data.password"
          :error="proxy.$errorStore.errors['password']"
          :disabled="loading"
          class="w-80"
          type="password"
          autocomplete="current-password"
        />

        <div class="my-6">
          <button type="submit" class="btn" @click="onDelete" :disabled="loading">
            <div v-if="loading">
              <span>Loading...</span>
            </div>
            <span v-else>Send me email for delete</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { ref, onMounted, onBeforeUnmount, getCurrentInstance } from "vue";
import { getUser, updateUser, updatePassword, deleteUserStep1 } from "@/api/user";
import {
  User_Request,
  UpdateUser_Request,
  UpdatePassword_Request,
  DeleteUser_Request,
} from "@proto/user";
import { FormInput } from "@/components";
import { showMessage } from "@/utils/message";

const { proxy } = getCurrentInstance();
const data: any = ref({});
const loading = ref(false);

const onUpdateProfile = async () => {
  await updateUser(<UpdateUser_Request>{
    user_id: proxy.$authStore.hasUserID,
    email: data.value.email,
    fio: data.value.fio,
  })
    .then((res) => {
      showMessage(res.data.message);
      proxy.$errorStore.$reset();
    })
    .catch((err) => {
      showMessage(err.response.data.message, "connextError");
    });
};

const onUpdatePassword = async () => {
  proxy.$errorStore.$reset();

  if (!data.value.old_password) {
    proxy.$errorStore.errors["old_password"] = "Weak password";
    return;
  }

  if (data.value.new_password.length < 8 || !data.value.new_password) {
    proxy.$errorStore.errors["new_password"] = proxy.$errorStore.errors["new_password2"] =
      "Weak password";
    return;
  }

  if (data.value.new_password != data.value.new_password2) {
    proxy.$errorStore.errors["new_password"] = proxy.$errorStore.errors["new_password2"] =
      "Passwords do not match";
    return;
  }

  if (data.value.old_password && data.value.new_password) {
    await updatePassword(<UpdatePassword_Request>{
      user_id: proxy.$authStore.hasUserID,
      old_password: data.value.old_password,
      new_password: data.value.new_password,
    })
      .then((res) => {
        showMessage(res.data.message);
        proxy.$authStore.refreshToken();

        data.value.old_password = "";
        data.value.new_password = "";
        data.value.new_password2 = "";
        proxy.$errorStore.$reset();
      })
      .catch((err) => {
        proxy.$errorStore.errors["old_password"] = err.response.data.message;
      });
  }
};

const onDelete = async () => {
  proxy.$errorStore.$reset();

  if (!data.value.password) {
    proxy.$errorStore.errors["password"] = "Password required";
    return;
  }

  if (data.value.password.length < 8) {
    proxy.$errorStore.errors["password"] = "Weak password";
    return;
  }

  await deleteUserStep1(<DeleteUser_Request>{
    user_id: proxy.$authStore.hasUserID,
    request: {
      password: data.value.password,
    },
  }).then((res) => {
    showMessage(res.data.message);
    proxy.$authStore.refreshToken();

    data.value.password = "";
    proxy.$errorStore.$reset();
  });
};

onMounted(async () => {
  await getUser(<User_Request>{
    user_id: proxy.$authStore.hasUserID,
  }).then((res) => {
    data.value = res.data.result;
  });
});

onBeforeUnmount(() => proxy.$errorStore.$reset());
</script>
