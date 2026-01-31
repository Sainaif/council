<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCouncilStore, type CouncilMode, type StartSessionRequest } from '../stores/council'
import { useModelsStore } from '../stores/models'

const { t } = useI18n()
const councilStore = useCouncilStore()
const modelsStore = useModelsStore()

const question = ref('')
const selectedModels = ref<string[]>([])
const mode = ref<CouncilMode>('standard')
const debateRounds = ref(3)
const enableDevil = ref(false)
const enableMystery = ref(false)

// Fetch models if not already loaded
onMounted(() => {
  if (modelsStore.models.length === 0 && !modelsStore.loading) {
    modelsStore.fetchModels()
  }
})

const canStart = computed(() => {
  return question.value.trim().length > 0 && selectedModels.value.length >= 2
})

const isActive = computed(() => {
  return ['pending', 'responding', 'voting', 'synthesizing'].includes(councilStore.status)
})

// Initialize userVote when responses are available and voting starts
const userVote = ref<string[]>([])
watch(
  () => ({ status: councilStore.status, responses: councilStore.anonymizedResponses }),
  ({ status, responses }) => {
    if ((status === 'voting' || status === 'completed') && userVote.value.length === 0 && responses.length > 0) {
      userVote.value = responses.map(r => r.label)
    }
  },
  { immediate: true }
)

function toggleModel(modelId: string) {
  const index = selectedModels.value.indexOf(modelId)
  if (index >= 0) {
    selectedModels.value.splice(index, 1)
  } else if (selectedModels.value.length < 8) {
    selectedModels.value.push(modelId)
  }
}

async function startCouncil() {
  const request: StartSessionRequest = {
    question: question.value,
    models: selectedModels.value,
    mode: mode.value,
    enable_devil_advocate: enableDevil.value,
    enable_mystery_judge: enableMystery.value
  }
  if (mode.value === 'debate') {
    request.debate_rounds = debateRounds.value
  }
  await councilStore.startSession(request)
}

function newCouncil() {
  councilStore.reset()
  question.value = ''
  selectedModels.value = []
  userVote.value = []
}

function moveUp(index: number) {
  if (index > 0 && index < userVote.value.length) {
    const arr = [...userVote.value]
    const temp = arr[index - 1]!
    arr[index - 1] = arr[index]!
    arr[index] = temp
    userVote.value = arr
  }
}

function moveDown(index: number) {
  if (index >= 0 && index < userVote.value.length - 1) {
    const arr = [...userVote.value]
    const temp = arr[index + 1]!
    arr[index + 1] = arr[index]!
    arr[index] = temp
    userVote.value = arr
  }
}

async function submitVote() {
  await councilStore.submitVote(userVote.value)
}
</script>

<template>
  <div class="max-w-4xl mx-auto p-4 space-y-6">
    <h1 class="text-2xl font-bold">{{ t('arena.title') }}</h1>

    <!-- Question Input -->
    <div v-if="!isActive && councilStore.status !== 'completed'" class="space-y-6">
      <div class="card p-4">
        <textarea
          v-model="question"
          :placeholder="t('arena.question_placeholder')"
          class="input min-h-32 resize-y"
          :disabled="isActive"
        />
      </div>

      <!-- Model Selection -->
      <div class="card p-4">
        <h2 class="text-lg font-medium mb-4">{{ t('arena.select_models') }} ({{ selectedModels.length }}/8)</h2>
        
        <!-- Loading state -->
        <div v-if="modelsStore.loading" class="text-text-secondary py-4">
          {{ t('common.loading') }}
        </div>
        
        <!-- Error state -->
        <div v-else-if="modelsStore.error" class="text-error py-4">
          {{ modelsStore.error }}
          <button @click="modelsStore.fetchModels()" class="btn btn-ghost ml-2">Retry</button>
        </div>
        
        <!-- Empty state -->
        <div v-else-if="modelsStore.models.length === 0" class="text-text-secondary py-4">
          No models available. Make sure your GitHub account has Copilot access.
        </div>
        
        <!-- Models list -->
        <div v-else class="flex flex-wrap gap-2">
          <button
            v-for="model in modelsStore.models"
            :key="model.id"
            @click="toggleModel(model.id)"
            :class="[
              'chip',
              selectedModels.includes(model.id) ? 'chip-selected' : 'chip-unselected'
            ]"
          >
            {{ model.display_name }}
            <span class="ml-1 text-xs opacity-75">({{ model.rating }})</span>
          </button>
        </div>
      </div>

      <!-- Mode Selection -->
      <div class="card p-4">
        <h2 class="text-lg font-medium mb-4">{{ t('arena.select_mode') }}</h2>
        <div class="grid grid-cols-3 gap-4">
          <button
            v-for="m in ['standard', 'debate', 'tournament'] as CouncilMode[]"
            :key="m"
            @click="mode = m"
            :class="[
              'p-4 rounded-lg border text-left transition-colors',
              mode === m
                ? 'border-primary bg-primary/10'
                : 'border-zinc-700 hover:border-zinc-600'
            ]"
          >
            <div class="font-medium">{{ t(`modes.${m}`) }}</div>
            <div class="text-sm text-text-secondary mt-1">{{ t(`modes.${m}_desc`) }}</div>
          </button>
        </div>

        <!-- Debate rounds -->
        <div v-if="mode === 'debate'" class="mt-4">
          <label class="text-sm text-text-secondary">Debate Rounds: {{ debateRounds }}</label>
          <input
            type="range"
            v-model.number="debateRounds"
            min="1"
            max="10"
            class="w-full mt-2"
          />
        </div>

        <!-- Special mechanics -->
        <div class="mt-4 flex gap-4">
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" v-model="enableDevil" class="rounded" />
            <span class="text-sm">Devil's Advocate</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" v-model="enableMystery" class="rounded" />
            <span class="text-sm">Mystery Judge</span>
          </label>
        </div>
      </div>

      <!-- Start Button -->
      <button
        @click="startCouncil"
        :disabled="!canStart || councilStore.loading"
        class="btn btn-primary w-full py-3 text-lg"
      >
        {{ councilStore.loading ? t('common.loading') : t('arena.start_council') }}
      </button>
    </div>

    <!-- Active Session -->
    <div v-if="isActive || councilStore.status === 'completed'" class="space-y-6">
      <!-- Status -->
      <div class="card p-4">
        <div class="flex items-center gap-2">
          <div v-if="isActive" class="w-3 h-3 bg-primary rounded-full animate-pulse" />
          <div v-else class="w-3 h-3 bg-success rounded-full" />
          <span class="font-medium">
            {{ councilStore.status === 'responding' ? t('arena.stage_responding') : '' }}
            {{ councilStore.status === 'voting' ? t('arena.stage_voting') : '' }}
            {{ councilStore.status === 'synthesizing' ? t('arena.stage_synthesizing') : '' }}
            {{ councilStore.status === 'completed' ? t('common.success') : '' }}
          </span>
        </div>
      </div>

      <!-- Responses -->
      <div class="space-y-4">
        <div
          v-for="response in councilStore.anonymizedResponses"
          :key="response.label"
          class="card p-4"
        >
          <div class="flex items-center justify-between mb-2">
            <span class="font-medium text-primary">{{ response.label }}</span>
            <span v-if="response.isStreaming" class="text-xs text-text-muted animate-pulse">
              typing...
            </span>
          </div>
          <div class="prose prose-invert max-w-none">
            <p class="whitespace-pre-wrap">{{ response.content }}</p>
          </div>
        </div>
      </div>

      <!-- User Voting (after responses complete) -->
      <div v-if="councilStore.status === 'voting' || councilStore.status === 'completed'" class="card p-4">
        <h2 class="text-lg font-medium mb-4">{{ t('arena.your_vote') }}</h2>
        <p class="text-sm text-text-secondary mb-4">{{ t('arena.vote_instructions') }}</p>

        <div class="space-y-2">
          <div
            v-for="(label, index) in userVote"
            :key="label"
            class="flex items-center gap-2 p-2 bg-surface rounded-lg"
          >
            <span class="text-primary font-medium w-6">{{ index + 1 }}.</span>
            <span class="flex-1">{{ label }}</span>
            <button @click="moveUp(index)" :disabled="index === 0" class="btn btn-ghost p-1">↑</button>
            <button @click="moveDown(index)" :disabled="index === userVote.length - 1" class="btn btn-ghost p-1">↓</button>
          </div>
        </div>

        <button
          @click="submitVote"
          :disabled="userVote.length === 0"
          class="btn btn-primary w-full mt-4"
        >
          {{ t('arena.submit_vote') }}
        </button>
      </div>

      <!-- Synthesis -->
      <div v-if="councilStore.currentSession?.synthesis" class="card p-4 border-primary">
        <h2 class="text-lg font-medium mb-4 text-primary">{{ t('arena.synthesis') }}</h2>
        <div class="prose prose-invert max-w-none">
          <p class="whitespace-pre-wrap">{{ councilStore.currentSession.synthesis }}</p>
        </div>
      </div>

      <!-- New Council Button -->
      <button
        v-if="councilStore.status === 'completed'"
        @click="newCouncil"
        class="btn btn-secondary w-full"
      >
        {{ t('arena.new_council') }}
      </button>
    </div>

    <!-- Error -->
    <div v-if="councilStore.error" class="card p-4 border-error bg-error/10">
      <p class="text-error">{{ councilStore.error }}</p>
    </div>
  </div>
</template>
