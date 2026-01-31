<script setup lang="ts">
import { RouterView } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { useModelsStore } from './stores/models'
import { onMounted } from 'vue'
import NavBar from './components/base/NavBar.vue'

const authStore = useAuthStore()
const modelsStore = useModelsStore()

onMounted(async () => {
  await authStore.fetchUser()
  if (authStore.isAuthenticated) {
    await modelsStore.fetchModels()
  }
})
</script>

<template>
  <div class="min-h-screen bg-background">
    <NavBar v-if="authStore.isAuthenticated" />
    <main :class="authStore.isAuthenticated ? 'pt-16' : ''">
      <RouterView />
    </main>
  </div>
</template>
