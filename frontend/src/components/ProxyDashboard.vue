<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from "vue";
import { Events } from "@wailsio/runtime";
import { useProxyStore } from "../stores/proxy";
import type { ModelAdapter, Provider, View } from "../types";

import TopBar from "./TopBar.vue";
import TabNav from "./TabNav.vue";
import OverviewTab from "./OverviewTab.vue";
import ModelsTab from "./ModelsTab.vue";
import ModelEditor from "./ModelEditor.vue";
import StatsTab from "./StatsTab.vue";
import CloseDialog from "./CloseDialog.vue";

const store = useProxyStore();

/* ---- local UI state ---- */
const currentView = ref<View>("overview");
const editorIndex = ref<number>(-1);
const editorModel = ref<ModelAdapter | null>(null);
const editorProvider = ref<Provider>("openai");
const showApiKey = ref(false);
const editorTestRunning = ref(false);
const showCloseDialog = ref(false);

/* ---- lifecycle & events ---- */
let offStateEvent: (() => void) | null = null;
let offCloseEvent: (() => void) | null = null;

onMounted(() => {
  store.refresh();
  offStateEvent = Events.On("proxyState", () => {
    store.refresh();
  });
  offCloseEvent = Events.On("closeRequested", () => {
    showCloseDialog.value = true;
  });
});

onBeforeUnmount(() => {
  if (offStateEvent) offStateEvent();
  if (offCloseEvent) offCloseEvent();
});

watch(currentView, (v) => {
  if (v === "stats") store.loadStats();
});

/* ---- editor helpers ---- */
function openEditor(idx: number) {
  if (idx === -1) {
    editorIndex.value = -1;
    editorProvider.value = store.providerTab;
    editorModel.value = {
      displayName: "",
      type: store.providerTab,
      baseURL: "",
      apiKey: "",
      modelID: "",
      contextWindow: "",
      reasoningEffort: "medium",
      serviceTier: "",
      maxOutputTokens: 0,
      thinkingBudget: 0,
      retryCount: 0,
      retryInterval: 0,
      timeout: 0,
      notes: "",
    };
  } else {
    editorIndex.value = idx;
    const src = store.cfg.modelAdapters[idx];
    editorProvider.value = src.type === "anthropic" ? "anthropic" : "openai";
    editorModel.value = JSON.parse(JSON.stringify(src));
  }
  showApiKey.value = false;
  currentView.value = "editor";
}

function cancelEditor() {
  currentView.value = "models";
  editorIndex.value = -1;
  editorModel.value = null;
}

async function saveEditor(runTest = false) {
  if (!editorModel.value) return;
  editorModel.value.type = editorProvider.value;
  if (editorIndex.value === -1) {
    store.cfg.modelAdapters.push(editorModel.value);
    editorIndex.value = store.cfg.modelAdapters.length - 1;
  } else {
    store.cfg.modelAdapters[editorIndex.value] = editorModel.value;
  }
  await store.persistConfig();
  if (runTest) {
    editorTestRunning.value = true;
    try {
      await store.testAdapter(editorIndex.value);
      editorModel.value = { ...store.cfg.modelAdapters[editorIndex.value] };
    } finally {
      editorTestRunning.value = false;
    }
    currentView.value = "models";
  } else {
    currentView.value = "models";
  }
}

function onEditorProviderChange(p: Provider) {
  editorProvider.value = p;
}

function onEditorModelUpdate(m: ModelAdapter) {
  editorModel.value = m;
}

function onRefresh() {
  if (currentView.value === "stats") {
    store.loadStats();
  } else {
    store.refresh();
  }
}
</script>

<template>
  <div class="shell">
    <!-- ============ STICKY TOP BAR ============ -->
    <TopBar />

    <!-- ============ TABS ============ -->
    <TabNav
      v-if="currentView !== 'editor'"
      :currentView="currentView"
      :modelCount="store.cfg.modelAdapters.length"
      @update:currentView="currentView = $event"
      @refresh="onRefresh"
    />

    <!-- ============ OVERVIEW ============ -->
    <OverviewTab
      v-if="currentView === 'overview'"
      @openEditor="openEditor"
    />

    <!-- ============ MODELS ============ -->
    <ModelsTab
      v-else-if="currentView === 'models'"
      @openEditor="openEditor"
    />

    <!-- ============ STATS ============ -->
    <StatsTab v-else-if="currentView === 'stats'" />

    <!-- ============ EDITOR ============ -->
    <ModelEditor
      v-else-if="currentView === 'editor' && editorModel"
      :editorIndex="editorIndex"
      :editorModel="editorModel"
      :editorProvider="editorProvider"
      :showApiKey="showApiKey"
      :editorTestRunning="editorTestRunning"
      @cancel="cancelEditor"
      @save="saveEditor"
      @update:editorProvider="onEditorProviderChange"
      @update:showApiKey="showApiKey = $event"
      @update:editorModel="onEditorModelUpdate"
    />

    <!-- ============ CLOSE DIALOG ============ -->
    <CloseDialog :show="showCloseDialog" />
  </div>
</template>

<style scoped>
.shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  color: #e4e4e7;
}
</style>