import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../api'

export interface Model {
  id: string
  display_name: string
  provider: string
  rating: number
  wins: number
  losses: number
  draws: number
  win_rate: number
  games_played: number
  capabilities?: string[]
}

export const useModelsStore = defineStore('models', () => {
  const models = ref<Model[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  const sortedByRating = computed(() => {
    return [...models.value].sort((a, b) => b.rating - a.rating)
  })

  const byProvider = computed(() => {
    const grouped: Record<string, Model[]> = {}
    for (const model of models.value) {
      const provider = model.provider
      if (!grouped[provider]) {
        grouped[provider] = []
      }
      grouped[provider]!.push(model)
    }
    return grouped
  })

  async function fetchModels() {
    loading.value = true
    error.value = null
    try {
      const response = await api.get('/api/models')
      models.value = response.data
    } catch (e: any) {
      error.value = e.response?.data?.message || 'Failed to fetch models'
    } finally {
      loading.value = false
    }
  }

  async function fetchModel(id: string) {
    try {
      const response = await api.get(`/api/models/${id}`)
      return response.data
    } catch (e: any) {
      error.value = e.response?.data?.message || 'Failed to fetch model'
      return null
    }
  }

  function getModel(id: string): Model | undefined {
    return models.value.find(m => m.id === id)
  }

  return {
    models,
    loading,
    error,
    sortedByRating,
    byProvider,
    fetchModels,
    fetchModel,
    getModel
  }
})
