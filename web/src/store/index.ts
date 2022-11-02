import { App } from "vue";
import { createPinia } from "pinia";

import { useAuthStore } from "@/store/modules/auth";
import { useSystemStore } from "@/store/modules/system";
import { useErrorStore } from "@/store/modules/error";

export default {
  install: (app: App) => {
    app.use(createPinia());

    const authStore = useAuthStore();
    const systemStore = useSystemStore();
    const errorStore = useErrorStore();

    app.config.globalProperties.$authStore = authStore;
    app.config.globalProperties.$systemStore = systemStore;
    app.config.globalProperties.$errorStore = errorStore;

    if (authStore.loggedIn) {
      authStore.getProfile();
      systemStore.getVersion();
    }
  },
};

export { useAuthStore, useSystemStore, useErrorStore };
