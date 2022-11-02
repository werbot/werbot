import axios, { AxiosRequestConfig, AxiosResponse } from "axios";
import createAuthRefreshInterceptor from "axios-auth-refresh";
import { useRouter } from "vue-router";
import { useErrorStore } from "@/store";
// @ts-ignore
import * as NProgress from "nprogress";

import { getStorage } from "@/utils/storage";
import { useAuthStore } from "@/store";

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

axios.defaults.baseURL = import.meta.env.VITE_API_URL;
axios.defaults.timeout = 5000;

//if (import.meta.env.MODE === "production") {
//  axios.defaults.withCredentials = false;
//  axios.defaults.headers.post["Access-Control-Allow-Origin-Type"] = "*";
//} else {
axios.defaults.withCredentials = true;
//}

axios.interceptors.request.use(
  (config: AxiosRequestConfig) => {
    NProgress.start();

    const access_token = getStorage("access_token");
    if (access_token) {
      config.headers.Authorization = `Bearer ${access_token}`;
    }

    return config;
  },
  (error: any) => {
    NProgress.done();
    return Promise.reject(error);
  }
);

axios.interceptors.response.use(
  (response: AxiosResponse) => {
    NProgress.done();
    return response;
  },
  (error: any) => {
    NProgress.done();

    if (error.response.data.message === "Your token has been revoked") {
      useAuthStore().resetStore();
      window.location.reload();
    }

    if (error.response.status === 400 || error.response.status === 500) {
      useErrorStore().$state.message = error.response.data.message;
      if (error.response.data.result) {
        useErrorStore().$state.errors = error.response.data.result;
      }
    }

    return Promise.reject(error);
  }
);

const refreshAuthLogic = (failedRequest) =>
  useAuthStore()
    .refreshToken()
    .then((res) => {
      failedRequest.response.config.headers["Authorization"] =
        "Bearer " + getStorage("access_token");
    });

createAuthRefreshInterceptor(axios, refreshAuthLogic);

export function http<T = any>(method: Method, route: string, options = {}): Promise<T> {
  return axios.request<T, T>({
    method: method,
    url: route,
    ...options,
  });
}
