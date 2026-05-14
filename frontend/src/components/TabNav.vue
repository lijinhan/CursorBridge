<script setup lang="ts">
import type { View } from "../types";
import { t } from "../i18n";

const props = defineProps<{
  currentView: View;
  modelCount: number;
}>();

const emit = defineEmits<{
  (e: "update:currentView", view: View): void;
  (e: "refresh"): void;
}>();
</script>

<template>
  <nav class="tabs">
    <button
      :class="['tab', currentView === 'overview' ? 'tab-active' : '']"
      @click="emit('update:currentView', 'overview')"
    >
      {{ t('tab.overview') }}
    </button>
    <button
      :class="['tab', currentView === 'models' ? 'tab-active' : '']"
      @click="emit('update:currentView', 'models')"
    >
      {{ t('tab.models') }} <span class="tab-count">{{ modelCount }}</span>
    </button>
    <button
      :class="['tab', currentView === 'stats' ? 'tab-active' : '']"
      @click="emit('update:currentView', 'stats')"
    >
      {{ t('tab.stats') }}
    </button>
    <span class="tab-spacer"></span>
    <button
      class="link-btn"
      @click="emit('refresh')"
      :title="t('footer.refreshState')"
    >
      <span class="icn">↻</span> {{ t('footer.refreshState') }}
    </button>
  </nav>
</template>

<style scoped>
.tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 10px 24px 0;
  border-bottom: 1px solid #1f1f22;
}
.tab {
  position: relative;
  background: transparent;
  border: 0;
  color: #a1a1aa;
  font-family: inherit;
  font-size: 13px;
  font-weight: 500;
  padding: 10px 14px;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}
.tab:hover {
  color: #fafafa;
}
.tab-active {
  color: #fafafa;
  border-bottom-color: #22c55e;
}
.tab-count {
  background: #27272a;
  color: #a1a1aa;
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 999px;
  margin-left: 6px;
}
</style>