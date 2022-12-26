<template>
  <div role="status">Please wait...</div>
</template>

<script setup lang="ts">
import { onMounted, getCurrentInstance } from "vue";
import { useRouter } from "vue-router";
import { deleteUserStep2 } from "@/api/user";
import { DeleteUser_Request } from "@proto/user";
import { showMessage } from "@/utils/message";

const { proxy } = getCurrentInstance();
const router = useRouter();
const props = defineProps({
  token: String,
});

onMounted(async () => {
  await deleteUserStep2(<DeleteUser_Request>{
    user_id: proxy.$authStore.hasUserID,
    request: {
      token: props.token,
    }
  })
    .then((res) => {
      showMessage(res.data.message);
      proxy.$authStore.resetStore();
      router.push({ name: "auth-signin" });
    })
    .catch((err) => {
      showMessage(err.response.data.message, "connextError");
      router.push({ name: "index" });
    });
});
</script>
