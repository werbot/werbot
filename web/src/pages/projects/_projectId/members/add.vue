<template>
  <div class="artboard">
    <header>
      <h1>
        <router-link
          :to="{
            name: 'projects-projectId-members-invites',
            params: {
              projectId: props.projectId,
            },
          }"
        >
          Invites
        </router-link>
        <span>New member</span>
      </h1>
    </header>

    <div class="desc">Description</div>
    <div class="artboard-content">
      <form @submit.prevent>
        <div class="flex flex-row">
          <FormInput
            name="Name"
            v-model.trim="data.name"
            :error="proxy.$errorStore.errors['name']"
            :disabled="loading"
            class="mr-5 flex-grow"
          />
          <FormInput
            name="Surname"
            v-model.trim="data.surname"
            :error="proxy.$errorStore.errors['surname']"
            :disabled="loading"
            class="flex-grow"
          />
        </div>

        <FormInput
          name="Email"
          v-model.trim="data.email"
          :error="proxy.$errorStore.errors['email']"
          :disabled="loading"
          class="flex-grow"
        />

        <div class="my-6">
          <button type="submit" class="btn" @click="onSendInvite" :disabled="loading">
            <div v-if="loading">
              <span>Loading...</span>
            </div>
            <span v-else>Send invite</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, getCurrentInstance, onBeforeUnmount } from "vue";
import { useRouter } from "vue-router";
import { FormInput } from "@/components";
import { showMessage } from "@/utils/message";

import { postProjectMemberInvite } from "@/api/member/project";
import { AddProjectMemberInvite_Request } from "@proto/member";

const { proxy } = getCurrentInstance();
const data: any = ref({});
const loading = ref(false);
const router = useRouter();

const props = defineProps({
  projectId: String,
});

const onSendInvite = async () => {
  console.log(data.value);

  await postProjectMemberInvite(<AddProjectMemberInvite_Request>{
    owner_id: proxy.$authStore.hasUserID,
    project_id: props.projectId,
    user_name: data.value.name,
    user_surname: data.value.surname,
    email: data.value.email,
  })
    .then((res) => {
      showMessage(res.data.message);
      proxy.$errorStore.$reset();
      router.push({
        name: "projects-projectId-members-invites",
        params: {
          projectId: props.projectId,
        },
      });
    })
    .catch((err) => {
      showMessage(err.response.data.message, "connextError");
    });
};

onBeforeUnmount(() => proxy.$errorStore.$reset());
</script>
