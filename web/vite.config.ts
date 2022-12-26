import { defineConfig } from "vite";
import path from "path";
import Vue from "@vitejs/plugin-vue";
import Pages from "vite-plugin-pages";
import Layouts from "vite-plugin-vue-layouts";
import { createSvgIconsPlugin } from "vite-plugin-svg-icons";

export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@proto": path.resolve(__dirname, "./src/proto"),
      "@protobuf-ts": path.resolve(__dirname, "./node_modules/@protobuf-ts"),
    },
    extensions: [".js", ".ts", ".json", ".vue"],
  },

  plugins: [
    Vue(),

    Pages({
      routeStyle: "nuxt",
      dirs: [
        { dir: "src/pages", baseRoute: "" },
      ],
      exclude: ["**/_menu_/index.vue"],
      //syncIndex: false,
      //importMode: "async",
    }),

    Layouts({
      defaultLayout: "private",
    }),

    createSvgIconsPlugin({
      iconDirs: [path.resolve(process.cwd(), "src/assets/icons")],
      symbolId: "icon-[dir]-[name]",
    }),
  ],

  css: {
    preprocessorOptions: {
      less: {
        javascriptEnabled: true,
      },
    },
  },
});
