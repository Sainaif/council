<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()

const sessions = ref<any[]>([])
const loading = ref(true)
const selectedSession = ref<any>(null)

async function fetchHistory() {
  loading.value = true
  try {
    // Note: This endpoint would need to be added to backend
    const response = await api.get('/api/council/history')
    sessions.value = response.data
  } catch (e) {
    sessions.value = []
  } finally {
    loading.value = false
  }
}

function viewSession(session: any) {
  selectedSession.value = session
}

function closeDetails() {
  selectedSession.value = null
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString()
}

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

      <div class="card p-4">
        <h2 class="text-lg font-medium mb-2">Question</h2>
        <p>{{ selectedSession.question }}</p>
      </div>

      <div v-if="selectedSession.responses" class="space-y-4">
        <h2 class="text-lg font-medium">Responses</h2>
        <div
          v-for="response in selectedSession.responses"
          :key="response.id"
          class="card p-4"
        >
          <div class="flex justify-between items-center mb-2">
            <span class="text-primary font-medium">{{ response.anonymous_label }}</span>
            <span class="text-sm text-text-muted">{{ response.response_time_ms }}ms</span>
          </div>
          <p class="whitespace-pre-wrap">{{ response.content }}</p>
        </div>
      </div>

      <div v-if="selectedSession.synthesis" class="card p-4 border-primary">
        <h2 class="text-lg font-medium mb-2 text-primary">{{ t('arena.synthesis') }}</h2>
        <p class="whitespace-pre-wrap">{{ selectedSession.synthesis }}</p>
      </div>
    </div>
  </div>
</template>
