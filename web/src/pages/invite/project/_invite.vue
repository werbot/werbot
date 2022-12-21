<template>
  <div v-if="data.message == 'Invite is invalid'" class="artboard-red">
    <header>
      <h1>Invitation to join the project</h1>
    </header>
    <div class="desc">
      The link to join the project is invalid.
    </div>
  </div>

  <div v-if="data.message == 'Wrong user'" class="artboard-red">
    <header>
      <h1>Invitation to join the project</h1>
    </header>
    <div class="desc">
      <b>Possible reasons for this warning:</b><br />
      1. You are not authorized. To continue -
      <router-link :to="{ name: 'index' }">login</router-link>.<br />
      2. You are authorized, but the invite is not intended for the current account. You must be
      logged in with the correct account (the email address you received the invitation to is the
      correct account)
    </div>
  </div>

  <div v-if="data.message == 'Invite is activated'" class="artboard-yellow">
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
  proxy.$systemStore.invites.project = props.invite;

  await getProjectMembersInviteActivate(props.invite)
    .then((res) => {
      data.value = res.data.result;
      if (data.value.project_id) {
        proxy.$systemStore.invites.project = null;
        router.push({ name: "index" });
      }
    })
    .catch((err) => {
      data.value = err.response.data;
      if (data.value.message == "New user") {
        router.push({ name: "auth-signup" });
      }
    });
});

onBeforeUnmount(() => proxy.$errorStore.$reset());

document.title = "Invitation to join the project";
</script>

<route lang="yaml">
meta:
  layout: public_blank
  requiresAuth: false
</route>
