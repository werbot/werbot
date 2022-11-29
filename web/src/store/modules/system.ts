import { defineStore } from "pinia";
import { getVersion } from "@/api/info";
import pkg from "../../../package.json";

export const useSystemStore = defineStore("system", {
  state: () => ({
    versions: {
      ui: "1.0 (00000000)",
      api: "1.0 (00000000)",
    },
    invites: {
      project: null,
    }
  }),

  getters: {
    hasVersions: (state) => state.versions,
  },

  actions: {
    resetStore() {
      this.$reset();
    },

    async getVersion() {
      this.versions.ui = pkg.version + " (" + pkg.commit + ")";

      await getVersion().then((res) => {
        this.versions.api = res.data.result;
      });
    },
  },
});
