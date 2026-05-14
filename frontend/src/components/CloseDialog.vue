<script setup lang="ts">
import { ref } from "vue";
import { ProxyService } from "../../bindings/cursorbridge/internal/bridge";

const props = defineProps<{
  show: boolean;
}>();

const rememberChoice = ref(false);
const closeBusy = ref(false);

async function pickClose(action: "quit" | "tray") {
  if (closeBusy.value) return;
  closeBusy.value = true;
  try {
    // Persist the choice FIRST so the dispatch happens with the preference
    // already on disk -- otherwise a crash between the two calls would leave
    // the pref unset and the dialog would re-appear next close.
    if (rememberChoice.value) {
      try {
        await ProxyService.SetCloseAction(action);
      } catch (e: any) {
        // Saving shouldn't block the close; surface but continue.
        console.warn("SetCloseAction failed:", e);
      }
    }
    if (action === "quit") {
      await ProxyService.RequestQuit();
    } else {
      await ProxyService.RequestHide();
    }
  } finally {
    closeBusy.value = false;
  }
}

/** Reset checkbox when dialog opens */
function resetState() {
  rememberChoice.value = false;
}

defineExpose({ resetState });
</script>

<template>
  <div
    v-if="props.show"
    class="close-modal-backdrop"
    role="dialog"
    aria-modal="true"
    aria-labelledby="close-modal-title"
  >
    <div class="close-modal">
      <div class="close-modal-title" id="close-modal-title">
        Close cursor-byok?
      </div>
      <div class="close-modal-desc">
        The proxy keeps running in the background when minimized to the system
        tray. Quitting stops the proxy and reverts Cursor to its
        pre-cursor-byok settings.
      </div>
      <label class="close-modal-remember">
        <input type="checkbox" v-model="rememberChoice" />
        <span>Remember my choice</span>
      </label>
      <div class="close-modal-actions">
        <button
          class="btn btn-ghost"
          :disabled="closeBusy"
          @click="pickClose('quit')"
        >
          Quit
        </button>
        <button
          class="btn btn-primary"
          :disabled="closeBusy"
          @click="pickClose('tray')"
        >
          Minimize to tray
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.close-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(9, 9, 11, 0.72);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  animation: close-modal-fade 120ms ease-out;
}
@keyframes close-modal-fade {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}
.close-modal {
  width: 420px;
  max-width: calc(100vw - 48px);
  background: #0f0f11;
  border: 1px solid #27272a;
  border-radius: 12px;
  padding: 22px;
  box-shadow:
    0 12px 40px rgba(0, 0, 0, 0.6),
    0 0 0 1px rgba(255, 255, 255, 0.02);
}
.close-modal-title {
  font-size: 16px;
  font-weight: 600;
  color: #fafafa;
  margin-bottom: 8px;
}
.close-modal-desc {
  font-size: 12.5px;
  color: #a1a1aa;
  line-height: 1.55;
  margin-bottom: 18px;
}
.close-modal-remember {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
  color: #d4d4d8;
  cursor: pointer;
  user-select: none;
  margin-bottom: 18px;
}
.close-modal-remember input[type="checkbox"] {
  width: 14px;
  height: 14px;
  accent-color: #22c55e;
  cursor: pointer;
}
.close-modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
