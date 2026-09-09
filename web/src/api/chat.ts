import { http, unwrap } from './client'
import type { ChatRequest, ChatResponse, HealthData } from '@/types'

export const chatApi = {
  send(payload: ChatRequest) {
    return unwrap<ChatResponse>(http.post('/chat', payload))
  },
  health() {
    return unwrap<HealthData>(http.get('/health'))
  },
}
