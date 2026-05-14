<script setup lang="ts">
import { useProxyStore } from "../stores/proxy";
import { t } from "../i18n";

const store = useProxyStore();
</script>

<template>
  <div class="page">
    <div class="stats-header">
      <div>
        <h3>{{ t('stats.title') }}</h3>
        <p class="stats-desc">{{ t('stats.desc') }}</p>
      </div>
    </div>

    <div class="stats-cards">
      <div class="stat-card">
        <div class="stat-label">{{ t('stats.totalTokens') }}</div>
        <div class="stat-value">
          {{ store.usageSummary.total.toLocaleString() }}
        </div>
        <div class="stat-sub">
          {{ store.usageSummary.prompt.toLocaleString() }}
          {{ t('stats.input') }} ·
          {{ store.usageSummary.completion.toLocaleString() }}
          {{ t('stats.output') }}
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-label">{{ t('stats.conversations') }}</div>
        <div class="stat-value">
          {{ store.usageSummary.conversations.toLocaleString() }}
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-label">{{ t('stats.turns') }}</div>
        <div class="stat-value">
          {{ store.usageSummary.turns.toLocaleString() }}
        </div>
      </div>
    </div>

    <div class="card" style="margin-top: 16px">
      <div class="row">
        <div class="row-text">
          <div class="row-title">{{ t('stats.perModel') }}</div>
          <div class="row-desc">{{ t('stats.perModelDesc') }}</div>
        </div>
      </div>
      <div class="hr" />
      <div v-if="!store.usageByModel.length" class="empty-mini">
        {{ t('stats.noTurns') }}
      </div>
      <table v-else class="data-table">
        <thead>
          <tr>
            <th>{{ t('stats.colModel') }}</th>
            <th>{{ t('stats.colProvider') }}</th>
            <th class="num">{{ t('stats.colPrompt') }}</th>
            <th class="num">{{ t('stats.colCompletion') }}</th>
            <th class="num">{{ t('stats.colTotal') }}</th>
            <th class="num">{{ t('stats.colTurns') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in store.usageByModel" :key="row.model">
            <td class="mono">{{ row.model }}</td>
            <td>{{ row.provider }}</td>
            <td class="num">{{ row.prompt.toLocaleString() }}</td>
            <td class="num">{{ row.completion.toLocaleString() }}</td>
            <td class="num">{{ row.total.toLocaleString() }}</td>
            <td class="num">{{ row.turns.toLocaleString() }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.page {
  flex: 1;
  padding: 20px 24px 40px;
  max-width: 1040px;
  width: 100%;
  margin: 0 auto;
  box-sizing: border-box;
}

.stats-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.stats-header h3 {
  font-size: 14px;
  font-weight: 600;
  color: #e4e4e7;
  margin: 0;
}
.stats-desc {
  font-size: 12px;
  color: #71717a;
  margin: 4px 0 0;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
.stat-card {
  background: #101012;
  border: 1px solid #1f1f22;
  border-radius: 10px;
  padding: 14px 16px;
}
.stat-label {
  font-size: 11px;
  color: #71717a;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}
.stat-value {
  font-size: 22px;
  font-weight: 600;
  color: #fafafa;
  margin: 4px 0 2px;
}
.stat-sub {
  font-size: 11px;
  color: #52525b;
}
</style>