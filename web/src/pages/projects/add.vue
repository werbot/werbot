<template>
  <div class="artboard">
    <header>
      <h1>New project</h1>
    </header>

    <div class="desc">Create a new project where you can add new servers and invite members.</div>

    <div class="artboard-content">
      <form @submit.prevent>
        <div class="mb-5 flex flex-row">
          <FormInput
            name="Title"
            v-model="data.title"
            :error="proxy.$errorStore.errors['title']"
            class="mr-5 flex-grow"
          />

          <FormInput
            name="Login"
            v-model.trim="data.login"
            :error="proxy.$errorStore.errors['login']"
            class="flex-grow"
          />
        </div>

        <div class="my-6">
          <button type="submit" class="btn" @click="onSubmit" :disabled="loading">
            <div v-if="loading">
              <span>Loading...</span>
            </div>
            <span v-else>Add project</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onBeforeUnmount, getCurrentInstance } from "vue";
import { useRouter } from "vue-router";
import { FormInput } from "@/components";
import { postProject } from "@/api/project";
import { AddProject_Request } from "@proto/project";
import { showMessage } from "@/utils/message";

const { proxy } = getCurrentInstance();
const data: any = ref({});
const loading = ref(false);
const router = useRouter();

const onSubmit = async () => {
  loading.value = !loading.value;

  await postProject(<AddProject_Request>{
    title: data.value.title,
    login: data.value.login,
  })
    .then((res) => {
      showMessage(res.data.message);
      proxy.$errorStore.$reset();
      const projectId = res.data.result.project_id;
      router.push({ name: "projects-projectId", params: { projectId } });
    })
    .catch((err) => {
      showMessage(err.response.data.message, "connextError");
      loading.value = !loading.value;
    });
};

onBeforeUnmount(() => proxy.$errorStore.$reset());

document.title = "new projects";
</script>

<route lang="yaml">
meta:
  layout: private_blank
  requiresAuth: true
</route>
