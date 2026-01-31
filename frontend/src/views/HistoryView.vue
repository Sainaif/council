<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()

const sessions = ref<any[]>([])
const loading = ref(true)
const selectedSession = ref<any>(null)
const loadingDetails = ref(false)
const expandedRounds = ref<Set<number>>(new Set())

async function fetchHistory() {
  loading.value = true
  try {
    const response = await api.get('/api/council/history')
    sessions.value = Array.isArray(response.data) ? response.data : []
  } catch (e) {
    console.error('Failed to fetch history', e)
    sessions.value = []
  } finally {
    loading.value = false
  }
}

async function viewSession(session: any) {
  loadingDetails.value = true
  expandedRounds.value = new Set()
  try {
    // Fetch full session details including responses
    const response = await api.get(`/api/council/${session.id}`)
    selectedSession.value = response.data
    // Expand all rounds by default
    if (selectedSession.value?.responses) {
      const rounds = new Set(selectedSession.value.responses.map((r: any) => r.round as number))
      expandedRounds.value = rounds
    }
  } catch (e) {
    console.error('Failed to fetch session details', e)
    selectedSession.value = session
  } finally {
    loadingDetails.value = false
  }
}

function closeDetails() {
  selectedSession.value = null
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString()
}

function toggleRound(round: number) {
  if (expandedRounds.value.has(round)) {
    expandedRounds.value.delete(round)
  } else {
    expandedRounds.value.add(round)
  }
  expandedRounds.value = new Set(expandedRounds.value)
}

// Group responses by round
const responsesByRound = computed(() => {
  if (!selectedSession.value?.responses) return {}
  const grouped: Record<number, any[]> = {}
  for (const response of selectedSession.value.responses) {
    const round = response.round || 1
    if (!grouped[round]) grouped[round] = []
    grouped[round].push(response)
  }
  return grouped
})

const rounds = computed(() => {
  return Object.keys(responsesByRound.value).map(Number).sort((a, b) => a - b)
})

// Get vote counts per response label
const voteResults = computed(() => {
  if (!selectedSession.value?.votes) return {}
  const counts: Record<string, { points: number, firstPlace: number }> = {}
  
  for (const vote of selectedSession.value.votes) {
    if (!vote.ranked_responses) continue
    const rankings = vote.ranked_responses
    rankings.forEach((label: string, index: number) => {
      if (!counts[label]) counts[label] = { points: 0, firstPlace: 0 }
      // More points for higher rank (inverse of position)
      counts[label].points += (rankings.length - index) * vote.weight
      if (index === 0) counts[label].firstPlace++
    })
  }
  return counts
})

const sortedVoteResults = computed(() => {
  return Object.entries(voteResults.value)
    .sort(([, a], [, b]) => b.points - a.points)
    .map(([label, data]) => ({ label, ...data }))
})

onMounted(fetchHistory)
</script>

<template>
  <div class="max-w-4xl mx-auto p-4 space-y-6">
    <h1 class="text-2xl font-bold">{{ t('history.title') }}</h1>

    <!-- Session List -->
    <div v-if="!selectedSession" class="space-y-4">
      <div v-if="loading" class="text-center text-text-muted py-8">
        {{ t('common.loading') }}
      </div>

      <div v-else-if="sessions.length === 0" class="text-center text-text-muted py-8">
        {{ t('history.no_sessions') }}
      </div>

      <div
        v-else
        v-for="session in sessions"
        :key="session.id"
        class="card p-4 cursor-pointer hover:border-primary transition-colors"
        @click="viewSession(session)"
      >
        <div class="flex justify-between items-start">
          <div class="flex-1">
            <p class="font-medium line-clamp-2">{{ session.question }}</p>
            <div class="flex gap-4 mt-2 text-sm text-text-secondary">
              <span>{{ t(`modes.${session.mode}`) }}</span>
              <span>{{ session.responses?.length || 0 }} responses</span>
            </div>
          </div>
          <div class="text-right text-sm">
            <div :class="session.status === 'completed' ? 'text-success' : 'text-warning'">
              {{ session.status }}
            </div>
            <div class="text-text-muted">{{ formatDate(session.created_at) }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Session Details -->
    <div v-if="selectedSession" class="space-y-6">
      <button @click="closeDetails" class="btn btn-ghost">
        ← {{ t('common.back') }}
      </button>

      <div v-if="loadingDetails" class="text-center py-8 text-text-muted">
        {{ t('common.loading') }}
      </div>

      <template v-else>
        <!-- Session Header -->
        <div class="card p-4">
          <div class="flex justify-between items-start mb-4">
            <div>
              <span class="text-sm text-text-secondary">{{ t(`modes.${selectedSession.mode}`) }}</span>
              <span class="mx-2 text-text-muted">•</span>
              <span class="text-sm text-text-secondary">{{ formatDate(selectedSession.created_at) }}</span>
            </div>
            <span :class="selectedSession.status === 'completed' ? 'text-success' : 'text-warning'" class="text-sm font-medium">
              {{ selectedSession.status }}
            </span>
          </div>
          <h2 class="text-lg font-medium">Question</h2>
          <p class="mt-2">{{ selectedSession.question }}</p>
        </div>

        <!-- Synthesis (Council's Conclusion) -->
        <div v-if="selectedSession.synthesis" class="card p-4 border-primary bg-primary/5">
          <h2 class="text-lg font-medium mb-3 text-primary">🎯 Council's Conclusion</h2>
          <p class="whitespace-pre-wrap">{{ selectedSession.synthesis }}</p>
        </div>

        <!-- Minority Report -->
        <div v-if="selectedSession.minority_report" class="card p-4 border-warning bg-warning/5">
          <h2 class="text-lg font-medium mb-3 text-warning">⚠️ Minority Report</h2>
          <p class="whitespace-pre-wrap">{{ selectedSession.minority_report }}</p>
        </div>

        <!-- Voting Results -->
        <div v-if="selectedSession.votes?.length" class="card p-4">
          <h2 class="text-lg font-medium mb-4">🗳️ Voting Results</h2>
          <p class="text-sm text-text-secondary mb-4">{{ selectedSession.votes.length }} models voted</p>
          
          <div class="space-y-2">
            <div
              v-for="(result, index) in sortedVoteResults"
              :key="result.label"
              class="flex items-center gap-3 p-2 rounded-lg"
              :class="index === 0 ? 'bg-primary/10 border border-primary/30' : 'bg-surface'"
            >
              <span class="text-lg font-bold w-8" :class="index === 0 ? 'text-primary' : 'text-text-muted'">
                #{{ index + 1 }}
              </span>
              <span class="font-medium flex-1">{{ result.label }}</span>
              <span class="text-sm text-text-secondary">{{ result.points.toFixed(1) }} pts</span>
              <span v-if="result.firstPlace > 0" class="text-xs bg-primary/20 text-primary px-2 py-0.5 rounded">
                🥇 {{ result.firstPlace }}
              </span>
            </div>
          </div>
        </div>

        <!-- Debate Rounds -->
        <div v-if="rounds.length > 0" class="space-y-4">
          <h2 class="text-lg font-medium">💬 Debate Rounds</h2>
          
          <div v-for="round in rounds" :key="round" class="card overflow-hidden">
            <!-- Round Header (clickable) -->
            <button
              @click="toggleRound(round)"
              class="w-full p-4 flex items-center justify-between hover:bg-surface transition-colors"
            >
              <span class="font-medium">
                Round {{ round }}
                <span class="text-text-secondary font-normal ml-2">
                  ({{ responsesByRound[round]?.length || 0 }} responses)
                </span>
              </span>
              <span class="text-text-muted">{{ expandedRounds.has(round) ? '▼' : '▶' }}</span>
            </button>

            <!-- Round Content -->
            <div v-if="expandedRounds.has(round)" class="border-t border-zinc-800">
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4 p-4">
                <div
                  v-for="response in responsesByRound[round]"
                  :key="response.id"
                  class="bg-surface rounded-lg p-4"
                >
                  <div class="flex justify-between items-center mb-2">
                    <span class="text-primary font-medium">{{ response.anonymous_label }}</span>
                    <span class="text-xs text-text-muted">{{ response.response_time_ms }}ms</span>
                  </div>
                  <p class="whitespace-pre-wrap text-sm max-h-48 overflow-auto">{{ response.content }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Model Reveal (after completion) -->
        <div v-if="selectedSession.status === 'completed' && selectedSession.responses?.length" class="card p-4">
          <h2 class="text-lg font-medium mb-4">🎭 Model Reveal</h2>
          <p class="text-sm text-text-secondary mb-4">See which model was behind each response</p>
          <div class="grid grid-cols-2 md:grid-cols-3 gap-2">
            <div
              v-for="response in ([...new Map(selectedSession.responses.map((r: any) => [r.anonymous_label, r])).values()] as any[])"
              :key="response.anonymous_label"
              class="flex items-center gap-2 p-2 bg-surface rounded"
            >
              <span class="text-primary font-medium">{{ response.anonymous_label }}</span>
              <span class="text-text-muted">=</span>
              <span class="text-sm truncate">{{ response.model_id }}</span>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
