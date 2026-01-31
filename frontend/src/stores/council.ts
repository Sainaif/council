import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../api'

export type SessionStatus = 'idle' | 'pending' | 'responding' | 'voting' | 'synthesizing' | 'completed' | 'failed' | 'cancelled'
export type CouncilMode = 'standard' | 'debate' | 'tournament'

export interface Response {
  id: number
  session_id: string
  model_id: string
  round: number
  content: string
  anonymous_label: string
  response_time_ms: number
  token_count: number
  created_at: string
  isStreaming?: boolean
}

export interface Vote {
  id: number
  session_id: string
  voter_type: 'model' | 'user'
  voter_id: string
  ranked_responses: string[]
  weight: number
}

export interface Session {
  id: string
  user_id: string
  question: string
  mode: CouncilMode
  status: SessionStatus
  category_id?: number
  chairperson_id?: string
  devil_advocate_id?: string
  mystery_judge_id?: string
  synthesis: string
  minority_report?: string
  responses: Response[]
  votes: Vote[]
  config: {
    debate_rounds: number
    response_timeout: number
    enable_devil_advocate: boolean
    enable_mystery_judge: boolean
  }
  created_at: string
  completed_at?: string
}

export interface StartSessionRequest {
  question: string
  models: string[]
  mode: CouncilMode
  category_id?: number
  chairperson_id?: string
  debate_rounds?: number
  enable_devil_advocate?: boolean
  enable_mystery_judge?: boolean
  response_timeout?: number
}

export const useCouncilStore = defineStore('council', () => {
  const currentSession = ref<Session | null>(null)
  const responses = ref<Map<string, Response>>(new Map())
  const status = ref<SessionStatus>('idle')
  const ws = ref<WebSocket | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  const sortedResponses = computed(() => {
    return Array.from(responses.value.values()).sort((a, b) => {
      if (a.round !== b.round) return a.round - b.round
      return a.anonymous_label.localeCompare(b.anonymous_label)
    })
  })

  const anonymizedResponses = computed(() => {
    return sortedResponses.value.map(r => ({
      label: r.anonymous_label,
      content: r.content,
      isStreaming: r.isStreaming
    }))
  })

  async function startSession(request: StartSessionRequest) {
    loading.value = true
    error.value = null
    responses.value.clear()

    try {
      const response = await api.post('/api/council/start', request)
      const { session_id } = response.data

      // Connect to WebSocket
      connectWebSocket(session_id)

      status.value = 'pending'
      return session_id
    } catch (e: any) {
      error.value = e.response?.data?.message || 'Failed to start session'
      throw e
    } finally {
      loading.value = false
    }
  }

  function connectWebSocket(sessionId: string) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/council/${sessionId}`

    ws.value = new WebSocket(wsUrl)

    ws.value.onmessage = (event) => {
      const message = JSON.parse(event.data)
      handleWebSocketMessage(message)
    }

    ws.value.onclose = () => {
      ws.value = null
    }

    ws.value.onerror = (e) => {
      console.error('WebSocket error:', e)
      error.value = 'Connection error'
    }
  }

  function handleWebSocketMessage(message: { event: string; data: any }) {
    switch (message.event) {
      case 'council.started':
        status.value = 'responding'
        break

      case 'model.responding':
        const { model_id, label } = message.data
        responses.value.set(label, {
          id: 0,
          session_id: currentSession.value?.id || '',
          model_id,
          round: 1,
          content: '',
          anonymous_label: label,
          response_time_ms: 0,
          token_count: 0,
          created_at: new Date().toISOString(),
          isStreaming: true
        })
        break

      case 'model.response_chunk':
        const chunkLabel = message.data.label
        const existing = responses.value.get(chunkLabel)
        if (existing) {
          existing.content += message.data.content
          existing.isStreaming = !message.data.done
          responses.value.set(chunkLabel, { ...existing })
        }
        break

      case 'model.complete':
        const completeLabel = message.data.label
        const resp = responses.value.get(completeLabel)
        if (resp) {
          resp.isStreaming = false
          resp.response_time_ms = message.data.response_time
          responses.value.set(completeLabel, { ...resp })
        }
        break

      case 'voting.started':
        status.value = 'voting'
        break

      case 'voting.received':
        // A model has voted
        break

      case 'synthesis.started':
        status.value = 'synthesizing'
        break

      case 'synthesis.complete':
        if (currentSession.value) {
          currentSession.value.synthesis = message.data.synthesis
          currentSession.value.minority_report = message.data.minority_report
        }
        break

      case 'council.completed':
        status.value = 'completed'
        fetchSession(currentSession.value?.id || '')
        break

      case 'council.failed':
        status.value = 'failed'
        error.value = message.data.reason
        break

      case 'council.cancelled':
        status.value = 'cancelled'
        break
    }
  }

  async function fetchSession(sessionId: string) {
    try {
      const response = await api.get(`/api/council/${sessionId}`)
      currentSession.value = response.data

      // Populate responses
      responses.value.clear()
      for (const r of currentSession.value!.responses || []) {
        responses.value.set(r.anonymous_label, r)
      }

      status.value = currentSession.value!.status as SessionStatus
    } catch (e: any) {
      error.value = e.response?.data?.message || 'Failed to fetch session'
    }
  }

  async function submitVote(rankedResponses: string[]) {
    if (!currentSession.value) return

    try {
      await api.post(`/api/council/${currentSession.value.id}/vote`, {
        ranked_responses: rankedResponses
      })
    } catch (e: any) {
      error.value = e.response?.data?.message || 'Failed to submit vote'
    }
  }

  function disconnect() {
    if (ws.value) {
      ws.value.close()
      ws.value = null
    }
  }

  function reset() {
    currentSession.value = null
    responses.value.clear()
    status.value = 'idle'
    error.value = null
    disconnect()
  }

  return {
    currentSession,
    responses,
    sortedResponses,
    anonymizedResponses,
    status,
    loading,
    error,
    startSession,
    fetchSession,
    submitVote,
    disconnect,
    reset
  }
})
