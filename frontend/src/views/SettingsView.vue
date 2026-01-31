<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import { useModelsStore } from '../stores/models'
import { settingsApi } from '../api'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const modelsStore = useModelsStore()

const settings = ref({
  default_models: [] as string[],
  language: 'en',
  ui_density: 'comfortable' as 'compact' | 'comfortable',
  auto_save_sessions: true,
  user_feedback_weight: 0.5
})

const loading = ref(true)
const saving = ref(false)
const saved = ref(false)

async function fetchSettings() {
  loading.value = true
  try {
    const response = await settingsApi.get()
    settings.value = { ...settings.value, ...response.data }
  } catch (e) {
    console.error('Failed to fetch settings', e)
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  saved.value = false
  try {
    await settingsApi.update(settings.value)
    saved.value = true
    setTimeout(() => saved.value = false, 2000)
  } catch (e) {
    console.error('Failed to save settings', e)
  } finally {
    saving.value = false
  }
}

function toggleDefaultModel(modelId: string) {
  const index = settings.value.default_models.indexOf(modelId)
  if (index >= 0) {
    settings.value.default_models.splice(index, 1)
  } else {
    settings.value.default_models.push(modelId)
  }
}

watch(() => settings.value.language, (newLang) => {
  locale.value = newLang
  localStorage.setItem('language', newLang)
})

onMounted(fetchSettings)
</script>

<template>
  <div class="max-w-2xl mx-auto p-4 space-y-6">
    <h1 class="text-2xl font-bold">{{ t('settings.title') }}</h1>

    <div v-if="loading" class="text-center text-text-muted py-8">
      {{ t('common.loading') }}
    </div>

    <div v-else class="space-y-6">
      <!-- Account -->
      <div class="card p-4">
        <h2 class="text-lg font-medium mb-4">{{ t('settings.account') }}</h2>
        <div class="flex items-center gap-4">
          <img
            v-if="authStore.user?.avatar_url"
            :src="authStore.user.avatar_url"
            :alt="authStore.user.username"
            class="w-16 h-16 rounded-full"
          />
          <div>
            <div class="font-medium text-lg">{{ authStore.user?.username }}</div>
            <div class="text-text-secondary">Connected via GitHub</div>
          </div>
        </div>
      </div>

      <!-- Preferences -->
      <div class="card p-4 space-y-6">
        <h2 class="text-lg font-medium">{{ t('settings.preferences') }}</h2>

        <!-- Language -->
        <div>
          <label class="block text-sm text-text-secondary mb-2">{{ t('settings.language') }}</label>
          <select v-model="settings.language" class="input">
            <option value="en">English</option>
            <option value="pl">Polski</option>
          </select>
        </div>

        <!-- UI Density -->
        <div>
          <label class="block text-sm text-text-secondary mb-2">{{ t('settings.ui_density') }}</label>
          <div class="flex gap-4">
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                v-model="settings.ui_density"
                value="compact"
                class="text-primary"
              />
              <span>{{ t('settings.compact') }}</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                v-model="settings.ui_density"
                value="comfortable"
                class="text-primary"
              />
              <span>{{ t('settings.comfortable') }}</span>
            </label>
          </div>
        </div>

        <!-- Default Models -->
        <div>
          <label class="block text-sm text-text-secondary mb-2">{{ t('settings.default_models') }}</label>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="model in modelsStore.models"
              :key="model.id"
              @click="toggleDefaultModel(model.id)"
              :class="[
                'chip',
                settings.default_models.includes(model.id) ? 'chip-selected' : 'chip-unselected'
              ]"
            >
              {{ model.display_name }}
            </button>
          </div>
        </div>

        <!-- Auto-save -->
        <div>
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              v-model="settings.auto_save_sessions"
              class="rounded text-primary"
            />
            <span>{{ t('settings.auto_save') }}</span>
          </label>
        </div>

        <!-- Feedback Weight -->
        <div>
          <label class="block text-sm text-text-secondary mb-2">
            {{ t('settings.feedback_weight') }}: {{ settings.user_feedback_weight.toFixed(1) }}
          </label>
          <input
            type="range"
            v-model.number="settings.user_feedback_weight"
            min="0"
            max="1"
            step="0.1"
            class="w-full"
          />
          <div class="flex justify-between text-xs text-text-muted mt-1">
            <span>Low impact</span>
            <span>High impact</span>
          </div>
        </div>
      </div>

      <!-- Save Button -->
      <button
        @click="saveSettings"
        :disabled="saving"
        class="btn btn-primary w-full"
      >
        {{ saving ? t('common.loading') : saved ? t('common.success') : t('common.save') }}
      </button>
    </div>
  </div>
</template>
