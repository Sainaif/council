<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../../stores/auth'
import { useRoute } from 'vue-router'

const { t } = useI18n()
const authStore = useAuthStore()
const route = useRoute()

const navItems = [
  { name: 'arena', path: '/arena', icon: '⚔️' },
  { name: 'rankings', path: '/rankings', icon: '🏆' },
  { name: 'history', path: '/history', icon: '📜' },
  { name: 'analytics', path: '/analytics', icon: '📊' },
  { name: 'settings', path: '/settings', icon: '⚙️' }
]

function isActive(path: string): boolean {
  return route.path === path
}
</script>

<template>
  <nav class="fixed top-0 left-0 right-0 z-50 bg-surface border-b border-zinc-800 h-16">
    <div class="max-w-7xl mx-auto px-4 h-full flex items-center justify-between">
      <!-- Logo -->
      <router-link to="/arena" class="flex items-center gap-2">
        <span class="text-xl font-bold text-gradient">Council Arena</span>
      </router-link>

      <!-- Navigation -->
      <div class="flex items-center gap-1">
        <router-link
          v-for="item in navItems"
          :key="item.name"
          :to="item.path"
          :class="[
            'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
            isActive(item.path)
              ? 'bg-primary text-white'
              : 'text-text-secondary hover:text-text-primary hover:bg-surface-elevated'
          ]"
        >
          {{ t(`nav.${item.name}`) }}
        </router-link>
      </div>

      <!-- User menu -->
      <div class="flex items-center gap-4">
        <div class="flex items-center gap-2">
          <img
            v-if="authStore.user?.avatar_url"
            :src="authStore.user.avatar_url"
            :alt="authStore.user.username"
            class="w-8 h-8 rounded-full"
          />
          <span class="text-sm text-text-secondary">{{ authStore.user?.username }}</span>
        </div>
        <button
          @click="authStore.logout"
          class="btn btn-ghost text-sm"
        >
          {{ t('nav.logout') }}
        </button>
      </div>
    </div>
  </nav>
</template>
