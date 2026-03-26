import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useMetricsStore = defineStore('metrics', () => {
  const isConnected = ref(false)
  const payload = ref(null)

  let ws = null

  function connect() {
    if (ws) ws.close()
    
    // Default to port 8080 if running frontend via dev server
    const port = window.location.port === '5173' || window.location.port === '5174' ? '8080' : window.location.port
    const host = window.location.hostname
    
    const wsProto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${wsProto}//${host}:${port}/ws`
    
    ws = new WebSocket(wsUrl)
    
    ws.onopen = () => {
      isConnected.value = true
    }
    
    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        payload.value = data
      } catch (err) {
        console.error('WebSocket receive error:', err)
      }
    }
    
    ws.onclose = () => {
      isConnected.value = false
      setTimeout(connect, 3000)
    }
  }

  return { isConnected, isDark, payload, toggleDark, connect }
})
