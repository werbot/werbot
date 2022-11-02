<template>
  <NotificationGroup group="alerts">
    <div class="alert">
      <div class="w-full max-w-sm">
        <Notification
          v-slot="{ notifications }"
          enter="transform ease-out duration-300 transition"
          enter-from="translate-y-2 opacity-0 sm:translate-y-0 sm:translate-x-4"
          enter-to="translate-y-0 opacity-100 sm:translate-x-0"
          leave="transition ease-in duration-500"
          leave-from="opacity-100"
          leave-to="opacity-0"
          move="transition duration-500"
          move-delay="delay-300"
        >
          <div v-for="notification in notifications" :key="notification.id">
            <div v-if="notification.type === 'error'" class="notification">
              <div class="ico bg-red-500">
                <SvgIcon name="error" class="text-white" />
              </div>

              <div class="message">
                <div class="mx-3">
                  <span class="font-semibold text-red-500">Error</span>
                  <p>{{ notification.text }}</p>
                </div>
              </div>
            </div>

            <div v-if="notification.type === 'info'" class="notification">
              <div class="ico bg-blue-500">
                <SvgIcon name="info" class="text-white" />
              </div>

              <div class="message">
                <div class="mx-3">
                  <span class="font-semibold text-blue-500">Info</span>
                  <p>{{ notification.text }}</p>
                </div>
              </div>
            </div>

            <div v-if="notification.type === 'success'" class="notification">
              <div class="ico bg-green-500">
                <SvgIcon name="success" class="text-white" />
              </div>

              <div class="message">
                <div class="mx-3">
                  <span class="font-semibold text-green-500">Success</span>
                  <p>{{ notification.text }}</p>
                </div>
              </div>
            </div>

            <div v-if="notification.type === 'warning'" class="notification">
              <div class="ico bg-yellow-500">
                <SvgIcon name="warning" class="text-white" />
              </div>

              <div class="message">
                <div class="mx-3">
                  <span class="font-semibold text-yellow-500">Warning</span>
                  <p>{{ notification.text }}</p>
                </div>
              </div>
            </div>
          </div>
        </Notification>
      </div>
    </div>
  </NotificationGroup>
</template>

<script setup lang="ts">
import { notify } from "notiwind";
import { SvgIcon } from "@/components";

addEventListener("connextError", (e) => {
  notify(
    {
      group: "alerts",
      type: "error",
      text: (<any>e).detail,
    },
    4000
  );
});
addEventListener("connextSuccess", (e) => {
  notify(
    {
      group: "alerts",
      type: "success",
      text: (<any>e).detail,
    },
    4000
  );
});
addEventListener("connextWarning", (e) => {
  notify(
    {
      group: "alerts",
      type: "warning",
      text: (<any>e).detail,
    },
    4000
  );
});
addEventListener("connextInfo", (e) => {
  notify(
    {
      group: "alerts",
      type: "info",
      text: (<any>e).detail,
    },
    4000
  );
});
</script>

<style lang="scss">
.alert {
  @apply pointer-events-none fixed bottom-0 right-0 z-50 flex items-start justify-end p-6 px-4 py-6;

  & .notification {
    @apply mx-auto mt-4 flex w-full max-w-sm overflow-hidden rounded-lg bg-white shadow-md;

    & .ico {
      @apply flex w-12 items-center justify-center;
    }

    & .message {
      @apply -mx-3 px-4 py-2;

      p {
        @apply text-sm text-gray-600;
      }
    }
  }
}
</style>
