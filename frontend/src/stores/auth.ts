import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../api'

export interface User {
  user_id: string
  username: string
  avatar_url: string
  language: string
  ui_density: 'compact' | 'comfortable'
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  const isAuthenticated = computed(() => !!user.value)

  async function fetchUser() {
    loading.value = true
    error.value = null
    try {
      const response = await api.get('/auth/me')
      user.value = response.data
    } catch (e) {
      user.value = null
    } finally {
      loading.value = false
    }
  }

  function login() {
    window.location.href = '/auth/github'
  }

  async function logout() {
    try {
      await api.get('/auth/logout')
      user.value = null
      window.location.href = '/login'
    } catch (e) {
      error.value = 'Failed to logout'
    }
  }

  return {
    user,
    loading,
    error,
    isAuthenticated,
    fetchUser,
    login,
    logout
  }
})
