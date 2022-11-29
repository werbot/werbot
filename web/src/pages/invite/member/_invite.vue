<template>
  <div v-if="data.message == 'wrong user'" class="artboard-red">
    <header>
      <h1>Invitation to join the project</h1>
    </header>
    <div class="desc">
      This invite is not intended for the current account. To continue adding - log in with the
      required account.
    </div>
  </div>

  <div v-if="data.message == 'invite is invalid'" class="artboard-yellow">
    <header>
      <h1>Invitation to join the project</h1>
    </header>
    <div class="desc">
      This prompt is not valid because it has already been used to add the current account to the
      project.
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, getCurrentInstance } from "vue";
import { useRouter } from "vue-router";
import { getProjectMembersInviteActivate } from "@/api/member/project";

const { proxy } = getCurrentInstance();
const data: any = ref({});
const router = useRouter();
const props = defineProps({
  invite: String,
});

onMounted(async () => {
  await getProjectMembersInviteActivate(props.invite)
    .then((res) => {
      data.value = res.data.result;
      if (data.value.project_id) {
        router.push({ name: "index" });
      }
    })
    .catch((err) => {
      data.value = err.response.data;
    });
});

onBeforeUnmount(() => proxy.$errorStore.$reset());
</script>

<route lang="yaml">
meta:
  layout: private_blank
  requiresAuth: true
</route>
