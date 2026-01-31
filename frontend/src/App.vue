<script setup lang="ts">
import { RouterView } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { useModelsStore } from './stores/models'
import { watch } from 'vue'
import NavBar from './components/base/NavBar.vue'

const authStore = useAuthStore()
const modelsStore = useModelsStore()

// Watch for authentication state changes and load models when authenticated
watch(() => authStore.isAuthenticated, async (isAuthenticated) => {
  if (isAuthenticated && modelsStore.models.length === 0) {
    await modelsStore.fetchModels()
  }
}, { immediate: true })
</script>

<template>
  <div class="min-h-screen bg-background">
    <NavBar v-if="authStore.isAuthenticated" />
    <main :class="authStore.isAuthenticated ? 'pt-16' : ''">
      <RouterView />
    </main>
  </div>
</template>
