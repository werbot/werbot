<template>
  <div class="artboard">
    <header>
      <h1>SSH keys</h1>
      <router-link :to="{ name: 'profile-keys-add' }">
        <label class="plus">
          <SvgIcon name="plus_square" />
          add new
        </label>
      </router-link>
    </header>

    <div class="desc">
      This is a list of SSH keys associated with your account. Remove any keys that you do not
      recognize.
    </div>

    <table v-if="data.total > 0">
      <tbody>
        <tr v-for="(item, index) in data.public_keys" :key="index">
          <td>
            <div class="font-bold">{{ item.title }}</div>
            <div>{{ item.fingerprint }}</div>
          </td>
          <td class="adaptive-md">
            <div><span class="font-bold">Added on:</span> {{ toDate(item.created, "lite") }}</div>
            <div v-if="item.last_used.seconds > 0">
              <span class="font-bold">Last used:</span> {{ toDate(item.last_used, "lite") }}
            </div>
            <div v-else class="text-gray-400">
              <span class="font-bold">Last used:</span> no used
            </div>
          </td>
          <td class="w-6">
            <SvgIcon name="delete" class="cursor-pointer text-red-500" @click="openModal(index)" />
          </td>
        </tr>
      </tbody>
    </table>
    <div v-else class="artboard-content p-5">Empty</div>

    <div class="artboard-content">
      <Pagination :total="data.total" @selectPage="onSelectPage" />
    </div>
  </div>

  <div class="m-6">
    Check out our guide to <a href="#">generating SSH keys</a> or troubleshoot
    <a href="#">common SSH problems</a>.
  </div>

  <Modal
    :showModal="modalActive"
    @close="closeModal"
    title="Are you sure you want to delete this SSH key?"
  >
    <p>
      This action CANNOT be undone. This will permanently delete the SSH key and if you’d like to
      use it in the future, you will need to upload it again. <br />
    </p>
    <template v-slot:footer>
      <div class="flex flex-row justify-end">
        <button class="btn btn-red" @click="removeKey(key.id)">Delete SSH key</button>
        <button class="btn ml-5" @click="closeModal">Close</button>
      </div>
    </template>
  </Modal>
</template>

<script setup lang="ts">
import { onMounted, ref, getCurrentInstance } from "vue";
import { useRoute } from "vue-router";
import { toDate } from "@/utils/time";
import { SvgIcon, Modal, Pagination } from "@/components";
import { showMessage } from "@/utils/message";

import { getKeys, deleteKey } from "@/api/key";
import { DeletePublicKey_Request } from "@proto/key";

const { proxy } = getCurrentInstance();
const data: any = ref({});
const key: any = ref({});
const route = useRoute();
const modalActive = ref(false);

const openModal = async (id: number) => {
  modalActive.value = true;
  key.value.id = id;
};

const closeModal = () => {
  modalActive.value = false;
};

const removeKey = async (id: number) => {
  await deleteKey(<DeletePublicKey_Request>{
    user_id: proxy.$authStore.hasUserID,
    key_id: data.value.public_keys[id].key_id,
  }).then((res) => {
    if (res.data.success) {
      closeModal();
      data.value.public_keys.splice(id, 1);
      data.value.total = data.value.total - 1;

      showMessage(res.data.message);
      proxy.$errorStore.$reset();
    }
  });
};

const getData = async (routeQuery: any) => {
  if (proxy.$authStore.hasUserRole === 3) {
    routeQuery.user_id = proxy.$authStore.hasUserID;
  }

  await getKeys(routeQuery).then((res) => {
    data.value = res.data.result;
  });
};

const onSelectPage = (e) => {
  getData(e);
};

onMounted(() => {
  getData(route.query);
});
</script>
