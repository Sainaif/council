import axios from 'axios'

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '',
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json'
  }
})

api.interceptors.response.use(
  response => response,
  error => {
    if (error.response?.status === 401) {
      // Redirect to login if unauthorized
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
    return Promise.reject(error)
  }
)

// API modules
export const authApi = {
  me: () => api.get('/auth/me'),
  logout: () => api.get('/auth/logout')
}

export const councilApi = {
  start: (data: any) => api.post('/api/council/start', data),
  get: (id: string) => api.get(`/api/council/${id}`),
  vote: (id: string, data: any) => api.post(`/api/council/${id}/vote`, data),
  appeal: (id: string) => api.post(`/api/council/${id}/appeal`),
  cancel: (id: string) => api.post(`/api/council/${id}/cancel`)
}

export const modelsApi = {
  list: () => api.get('/api/models'),
  get: (id: string) => api.get(`/api/models/${id}`),
  history: (id: string) => api.get(`/api/models/${id}/history`)
}

export const rankingsApi = {
  global: () => api.get('/api/rankings'),
  byCategory: (category: string) => api.get(`/api/rankings/${category}`),
  headToHead: (a: string, b: string) => api.get(`/api/matchups/${a}/${b}`)
}

export const analyticsApi = {
  overview: () => api.get('/api/analytics/overview'),
  userBias: () => api.get('/api/analytics/user-bias'),
  costs: () => api.get('/api/analytics/costs')
}

export const settingsApi = {
  get: () => api.get('/api/settings'),
  update: (data: any) => api.put('/api/settings', data)
}
