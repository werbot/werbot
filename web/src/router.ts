import { createRouter, createWebHistory } from "vue-router";
// @ts-ignore
import * as NProgress from "nprogress";
import { useAuthStore } from "@/store";
import { setupLayouts } from "virtual:generated-layouts";
import generatedRoutes from "~pages";

const routes = setupLayouts(generatedRoutes);
const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach(async (to, from, next) => {
  NProgress.start();

  if (to.meta.requiresAuth && !useAuthStore().loggedIn) {
    next({ name: "auth-signin" });
  }
  next();
});

router.afterEach((to) => {
  NProgress.done();
});

export default router;
