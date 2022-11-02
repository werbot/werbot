import { defineStore } from "pinia";

export const useErrorStore = defineStore("error", {
  state: () => ({
    message: null,
    errors: {} ,
  }),

  getters: {},

  actions: {
    resetStore() {
      this.$reset();
    },
  },
});
