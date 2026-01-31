import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  {
    path: '/',
    redirect: '/arena'
  },
  {
    path: '/arena',
    name: 'arena',
    component: () => import('../views/ArenaView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/rankings',
    name: 'rankings',
    component: () => import('../views/RankingsView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/history',
    name: 'history',
    component: () => import('../views/HistoryView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/analytics',
    name: 'analytics',
    component: () => import('../views/AnalyticsView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('../views/SettingsView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/LoginView.vue')
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// Track if initial auth check is done
let initialAuthCheckDone = false

router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // Do initial auth check only once
  if (!initialAuthCheckDone) {
    initialAuthCheckDone = true
    await authStore.fetchUser()
  }

  // Now decide routing based on auth state
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    // Not authenticated, redirect to login with return URL
    next({ name: 'login', query: { redirect: to.fullPath } })
  } else if (to.name === 'login' && authStore.isAuthenticated) {
    // Already authenticated, redirect away from login
    const redirect = to.query.redirect as string
    next(redirect || { name: 'arena' })
  } else {
    next()
  }
})

export default router
