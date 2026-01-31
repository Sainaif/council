<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { analyticsApi } from '../api'

const { t } = useI18n()

const overview = ref<any>(null)
const modelTrends = ref<any[]>([])
const userBias = ref<any>(null)
const costs = ref<any>(null)
const loading = ref(true)
const activeTab = ref('overview')

async function fetchAnalytics() {
  loading.value = true
  try {
    const [overviewRes, biasRes, costsRes] = await Promise.all([
      analyticsApi.overview(),
      analyticsApi.userBias(),
      analyticsApi.costs()
    ])
    overview.value = overviewRes.data.overview
    modelTrends.value = overviewRes.data.model_trends || []
    userBias.value = biasRes.data
    costs.value = costsRes.data
  } catch (e) {
    console.error('Failed to fetch analytics', e)
  } finally {
    loading.value = false
  }
}

onMounted(fetchAnalytics)
</script>

<template>
  <div class="max-w-4xl mx-auto p-4 space-y-6">
    <h1 class="text-2xl font-bold">{{ t('analytics.title') }}</h1>

    <!-- Tabs -->
    <div class="flex gap-2">
      <button
        v-for="tab in ['overview', 'user_bias', 'costs']"
        :key="tab"
        @click="activeTab = tab"
        :class="[
          'chip',
          activeTab === tab ? 'chip-selected' : 'chip-unselected'
        ]"
      >
        {{ t(`analytics.${tab}`) }}
      </button>
    </div>

    <div v-if="loading" class="text-center text-text-muted py-8">
      {{ t('common.loading') }}
    </div>

    <!-- Overview Tab -->
    <div v-else-if="activeTab === 'overview'" class="space-y-6">
      <!-- Stats Grid -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div class="card p-4">
          <div class="text-text-secondary text-sm">{{ t('analytics.total_sessions') }}</div>
          <div class="text-2xl font-bold">{{ overview?.total_sessions || 0 }}</div>
        </div>
        <div class="card p-4">
          <div class="text-text-secondary text-sm">{{ t('analytics.completed') }}</div>
          <div class="text-2xl font-bold text-success">{{ overview?.completed_count || 0 }}</div>
        </div>
        <div class="card p-4">
          <div class="text-text-secondary text-sm">{{ t('analytics.most_used_model') }}</div>
          <div class="text-lg font-medium truncate">{{ overview?.most_used_model || '-' }}</div>
        </div>
        <div class="card p-4">
          <div class="text-text-secondary text-sm">{{ t('analytics.top_performer') }}</div>
          <div class="text-lg font-medium truncate">{{ overview?.top_performer || '-' }}</div>
        </div>
      </div>

      <!-- Model Trends -->
      <div class="card p-4">
        <h2 class="text-lg font-medium mb-4">Model Performance</h2>
        <div class="space-y-3">
          <div
            v-for="model in modelTrends"
            :key="model.model_id"
            class="flex items-center justify-between"
          >
            <div>
              <span class="font-medium">{{ model.display_name }}</span>
              <span class="text-text-secondary ml-2">({{ model.rating }})</span>
            </div>
            <div class="flex items-center gap-4">
              <span class="text-sm text-text-secondary">
                {{ (model.win_rate * 100).toFixed(1) }}% win rate
              </span>
              <span
                :class="model.trend_7d > 0 ? 'text-success' : model.trend_7d < 0 ? 'text-error' : 'text-text-muted'"
                class="font-mono"
              >
                {{ model.trend_7d > 0 ? '+' : '' }}{{ model.trend_7d }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- User Bias Tab -->
    <div v-else-if="activeTab === 'user_bias'" class="space-y-6">
      <div v-if="userBias?.bias_warning" class="card p-4 border-warning bg-warning/10">
        <p class="text-warning">{{ userBias.bias_warning }}</p>
      </div>

      <div class="card p-4">
        <h2 class="text-lg font-medium mb-4">Your Model Preferences</h2>
        <div class="space-y-3">
          <div
            v-for="pref in userBias?.preferences"
            :key="pref.model_id"
            class="flex items-center justify-between"
          >
            <span>{{ pref.display_name }}</span>
            <div class="flex items-center gap-4">
              <span class="text-sm text-text-secondary">
                {{ pref.times_voted_for }} / {{ pref.total_votes }} votes
              </span>
              <span class="font-mono">
                {{ (pref.preference * 100).toFixed(1) }}%
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Costs Tab -->
    <div v-else-if="activeTab === 'costs'" class="space-y-6">
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div class="card p-4">
          <div class="text-text-secondary text-sm">{{ t('analytics.tokens_used') }}</div>
          <div class="text-2xl font-bold">{{ costs?.summary?.total_tokens?.toLocaleString() || 0 }}</div>
        </div>
        <div class="card p-4">
          <div class="text-text-secondary text-sm">Today</div>
          <div class="text-2xl font-bold">{{ costs?.summary?.tokens_today?.toLocaleString() || 0 }}</div>
        </div>
        <div class="card p-4">
          <div class="text-text-secondary text-sm">This Week</div>
          <div class="text-2xl font-bold">{{ costs?.summary?.tokens_this_week?.toLocaleString() || 0 }}</div>
        </div>
        <div class="card p-4">
          <div class="text-text-secondary text-sm">This Month</div>
          <div class="text-2xl font-bold">{{ costs?.summary?.tokens_this_month?.toLocaleString() || 0 }}</div>
        </div>
      </div>

      <div class="card p-4">
        <h2 class="text-lg font-medium mb-4">Usage by Model</h2>
        <div class="space-y-3">
          <div
            v-for="model in costs?.by_model"
            :key="model.model_id"
            class="flex items-center justify-between"
          >
            <span>{{ model.display_name }}</span>
            <div class="text-right">
              <div>{{ model.token_count?.toLocaleString() }} tokens</div>
              <div class="text-sm text-text-muted">{{ model.requests }} requests</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
