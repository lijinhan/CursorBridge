<script setup lang="ts">
import { computed } from "vue";
import { useProxyStore } from "../stores/proxy";

const store = useProxyStore();

function formatNum(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return (n / 1000).toFixed(n < 10_000 ? 1 : 0) + "k";
  return (n / 1_000_000).toFixed(n < 10_000_000 ? 2 : 1) + "M";
}

function shortDate(d: string): string {
  // YYYY-MM-DD -> "4月12日"
  const parts = d.split("-");
  if (parts.length !== 3) return d;
  const m = [
    "1月",
    "2月",
    "3月",
    "4月",
    "5月",
    "6月",
    "7月",
    "8月",
    "9月",
    "10月",
    "11月",
    "12月",
  ];
  const mi = Number(parts[1]) - 1;
  return (m[mi] ?? parts[1]) + Number(parts[2]) + "日";
}
</script>

<template>
  <div class="page">
    <div class="overview-section-head">
      <h3>Token usage</h3>
      <span class="row-desc" style="margin: 0"
        >Aggregated from on-disk conversation history.</span
      >
    </div>

    <div class="stat-cards">
      <div class="stat-card">
        <div class="stat-label">Total tokens</div>
        <div class="stat-value">{{ formatNum(store.stats.totalTokens) }}</div>
        <div class="stat-sub">{{ store.stats.totalTokens.toLocaleString() }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Prompt</div>
        <div class="stat-value">{{ formatNum(store.stats.totalPromptTokens) }}</div>
        <div class="stat-sub">input</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Completion</div>
        <div class="stat-value">
          {{ formatNum(store.stats.totalCompletionTokens) }}
        </div>
        <div class="stat-sub">output</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Conversations</div>
        <div class="stat-value">{{ store.stats.conversationCount }}</div>
        <div class="stat-sub">{{ store.stats.turnCount }} turns</div>
      </div>
    </div>

    <section class="card form-section" style="margin-top: 16px">
      <div class="section-head">
        <h3>Last 7 days</h3>
        <p>Prompt vs completion tokens per day (local time).</p>
      </div>
      <div
        v-if="store.maxDailyTotal === 0"
        class="empty-mini"
        style="margin: 8px 0"
      >
        No usage recorded in the last week.
      </div>
      <div v-else class="chart7">
        <div v-for="d in store.stats.last7Days" :key="d.date" class="chart-col">
          <div class="chart-bar-wrap">
            <div
              class="chart-bar chart-bar-prompt"
              :style="{
                height:
                  (store.maxDailyTotal
                    ? (d.promptTokens / store.maxDailyTotal) * 100
                    : 0) + '%',
              }"
              :title="'Prompt ' + d.promptTokens.toLocaleString()"
            ></div>
            <div
              class="chart-bar chart-bar-completion"
              :style="{
                height:
                  (store.maxDailyTotal
                    ? (d.completionTokens / store.maxDailyTotal) * 100
                    : 0) + '%',
              }"
              :title="'Completion ' + d.completionTokens.toLocaleString()"
            ></div>
          </div>
          <div class="chart-label">{{ shortDate(d.date) }}</div>
          <div class="chart-sub">
            {{ formatNum(d.promptTokens + d.completionTokens) }}
          </div>
        </div>
      </div>
      <div class="chart-legend">
        <span class="leg-dot leg-prompt"></span> Prompt
        <span class="leg-dot leg-completion" style="margin-left: 14px"></span>
        Completion
      </div>
    </section>

    <section class="card form-section" style="margin-top: 16px">
      <div class="section-head">
        <h3>Per model</h3>
        <p>Sorted by total tokens. One row per model+provider pair.</p>
      </div>
      <div
        v-if="!store.stats.perModel.length"
        class="empty-mini"
        style="margin: 8px 0"
      >
        No turns recorded yet.
      </div>
      <table v-else class="stats-table">
        <thead>
          <tr>
            <th>Model</th>
            <th>Provider</th>
            <th class="num">Prompt</th>
            <th class="num">Completion</th>
            <th class="num">Total</th>
            <th class="num">Turns</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(m, i) in store.stats.perModel" :key="i">
            <td class="mono">{{ m.model || "—" }}</td>
            <td>{{ m.provider || "—" }}</td>
            <td class="num mono">{{ m.promptTokens.toLocaleString() }}</td>
            <td class="num mono">
              {{ m.completionTokens.toLocaleString() }}
            </td>
            <td class="num mono">
              <b>{{
                (m.promptTokens + m.completionTokens).toLocaleString()
              }}</b>
            </td>
            <td class="num mono">{{ m.turnCount }}</td>
          </tr>
        </tbody>
      </table>
    </section>
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

.overview-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.overview-section-head h3 {
  font-size: 14px;
  font-weight: 600;
  color: #e4e4e7;
  margin: 0;
}

.stat-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}
.stat-card {
  background: #0d0d0f;
  border: 1px solid #1f1f22;
  border-radius: 10px;
  padding: 14px 16px;
}
.stat-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #71717a;
  margin-bottom: 8px;
}
.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: #fafafa;
  line-height: 1;
}
.stat-sub {
  margin-top: 6px;
  font-size: 12px;
  color: #a1a1aa;
}

.form-section {
  padding: 18px 20px;
}

.chart7 {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 10px;
  align-items: end;
  height: 180px;
  margin-top: 4px;
}
.chart-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  height: 100%;
}
.chart-bar-wrap {
  flex: 1;
  width: 100%;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  gap: 4px;
  min-height: 2px;
}
.chart-bar {
  width: 14px;
  min-height: 2px;
  border-radius: 3px 3px 0 0;
  transition: height 0.2s;
}
.chart-bar-prompt {
  background: #22c55e;
}
.chart-bar-completion {
  background: #60a5fa;
}
.chart-label {
  margin-top: 8px;
  font-size: 11px;
  color: #a1a1aa;
}
.chart-sub {
  font-size: 10px;
  color: #52525b;
  margin-top: 2px;
}
.chart-legend {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #a1a1aa;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #1f1f22;
}
.leg-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 2px;
  margin-right: 4px;
}
.leg-prompt {
  background: #22c55e;
}
.leg-completion {
  background: #60a5fa;
}

.stats-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.stats-table th {
  text-align: left;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: #71717a;
  padding: 8px 12px;
  border-bottom: 1px solid #1f1f22;
  font-weight: 500;
}
.stats-table td {
  padding: 10px 12px;
  border-bottom: 1px solid #141416;
  color: #d4d4d8;
}
.stats-table tr:last-child td {
  border-bottom: 0;
}
.stats-table .num {
  text-align: right;
}
.stats-table th.num {
  text-align: right;
}
</style>
