<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { rankingsApi } from '../api'

const { t } = useI18n()

const rankings = ref<any[]>([])
const loading = ref(true)
const selectedCategory = ref('global')

const categories = ['global', 'coding', 'creative', 'reasoning', 'math', 'general']

async function fetchRankings() {
  loading.value = true
  try {
    const response = selectedCategory.value === 'global'
      ? await rankingsApi.global()
      : await rankingsApi.byCategory(selectedCategory.value)

    // Handle null responses - ensure we always have an array
    const data = selectedCategory.value === 'global'
      ? response.data
      : response.data?.rankings
    rankings.value = Array.isArray(data) ? data : []
  } catch (e) {
    console.error('Failed to fetch rankings', e)
    rankings.value = []
  } finally {
    loading.value = false
  }
}

function selectCategory(cat: string) {
  selectedCategory.value = cat
  fetchRankings()
}

function getTrendClass(trend: number) {
  if (trend > 0) return 'text-success'
  if (trend < 0) return 'text-error'
  return 'text-text-muted'
}

function getTrendSymbol(trend: number) {
  if (trend > 0) return `+${trend}`
  return trend.toString()
}

onMounted(fetchRankings)
</script>

<template>
  <div class="max-w-4xl mx-auto p-4 space-y-6">
    <h1 class="text-2xl font-bold">{{ t('rankings.title') }}</h1>

    <!-- Category Tabs -->
    <div class="flex gap-2 overflow-x-auto pb-2">
      <button
        v-for="cat in categories"
        :key="cat"
        @click="selectCategory(cat)"
        :class="[
          'chip whitespace-nowrap',
          selectedCategory === cat ? 'chip-selected' : 'chip-unselected'
        ]"
      >
        {{ cat === 'global' ? t('rankings.global') : t(`categories.${cat}`) }}
      </button>
    </div>

    <!-- Rankings Table -->
    <div class="card overflow-hidden">
      <table class="w-full">
        <thead class="bg-surface">
          <tr class="text-left text-text-secondary text-sm">
            <th class="p-4 w-16">#</th>
            <th class="p-4">Model</th>
            <th class="p-4 text-right">{{ t('models.elo_rating') }}</th>
            <th class="p-4 text-right">{{ t('models.win_rate') }}</th>
            <th class="p-4 text-right">{{ t('models.games_played') }}</th>
            <th class="p-4 text-right">{{ t('models.trend') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="entry in rankings"
            :key="entry.model_id"
            class="border-t border-zinc-800 hover:bg-surface-hover transition-colors"
          >
            <td class="p-4 text-text-muted">{{ entry.rank }}</td>
            <td class="p-4">
              <div class="font-medium">{{ entry.display_name || entry.model_id }}</div>
              <div class="text-sm text-text-muted">{{ entry.provider }}</div>
            </td>
            <td class="p-4 text-right font-mono">{{ entry.rating }}</td>
            <td class="p-4 text-right">
              {{ (entry.win_rate * 100).toFixed(1) }}%
            </td>
            <td class="p-4 text-right text-text-secondary">
              {{ entry.games_played }}
            </td>
            <td class="p-4 text-right font-mono" :class="getTrendClass(entry.trend)">
              {{ getTrendSymbol(entry.trend || 0) }}
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="loading" class="p-8 text-center text-text-muted">
        {{ t('common.loading') }}
      </div>

      <div v-if="!loading && rankings.length === 0" class="p-8 text-center text-text-muted">
        No rankings data yet
      </div>
    </div>
  </div>
</template>
