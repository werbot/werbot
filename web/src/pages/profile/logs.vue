<template>
  <div class="artboard">
    <header>
      <h1>Profile logs</h1>
    </header>

    <table>
      <thead>
        <tr>
          <th>Date</th>
          <th>Event</th>
          <th>Ip</th>
          <th class="w-8"></th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>25/05/2022 15:31:48</td>
          <td>login_success</td>
          <td>162.158.90.222</td>
          <td>
            <SvgIcon name="info" @click="openDrawer(1)" />
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { SvgIcon, Drawer } from "@/components";

const drawerActive = ref(false);

const openDrawer = async (id: number) => {
  drawerActive.value = true;
  console.log(id);
};

const closeDrawer = () => {
  drawerActive.value = false;
};
</script>
