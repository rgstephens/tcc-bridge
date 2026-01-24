export interface SystemStatus {
  tcc: {
    connected: boolean
    last_poll?: string
    error?: string
  }
  matter: {
    running: boolean
    commissioned: boolean
    fabric_id?: string
  }
  configured: boolean
}

export interface ThermostatState {
  device_id: number
  name: string
  current_temp: number
  heat_setpoint: number
  cool_setpoint: number
  system_mode: string
  humidity: number
  is_heating: boolean
  is_cooling: boolean
  updated_at: string
}

export interface ConfigStatus {
  has_credentials: boolean
  username?: string
}

export interface PairingInfo {
  qr_code: string
  manual_pair_code: string
  commissioned: boolean
}

export interface EventLog {
  id: number
  timestamp: string
  source: string
  event_type: string
  message: string
  details?: Record<string, unknown>
}

export interface VersionInfo {
  version: string
  build_date: string
}

class ApiClient {
  private baseUrl: string

  constructor(baseUrl: string = '/api') {
    this.baseUrl = baseUrl
  }

  private async request<T>(path: string, options?: RequestInit): Promise<T> {
    const response = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Request failed' }))
      throw new Error(error.error || `HTTP ${response.status}`)
    }

    return response.json()
  }

  async getStatus(): Promise<SystemStatus> {
    return this.request<SystemStatus>('/status')
  }

  async getThermostat(): Promise<ThermostatState[]> {
    return this.request<ThermostatState[]>('/thermostat')
  }

  async setSetpoint(deviceId: number, type: 'heat' | 'cool', value: number): Promise<void> {
    await this.request('/thermostat/setpoint', {
      method: 'POST',
      body: JSON.stringify({ device_id: deviceId, type, value }),
    })
  }

  async setMode(deviceId: number, mode: string): Promise<void> {
    await this.request('/thermostat/mode', {
      method: 'POST',
      body: JSON.stringify({ device_id: deviceId, mode }),
    })
  }

  async getConfig(): Promise<ConfigStatus> {
    return this.request<ConfigStatus>('/config')
  }

  async saveCredentials(username: string, password: string): Promise<void> {
    await this.request('/config/credentials', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
  }

  async testCredentials(username: string, password: string): Promise<{ success: boolean; error?: string }> {
    return this.request('/config/credentials/test', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
  }

  async getPairing(): Promise<PairingInfo> {
    return this.request<PairingInfo>('/pairing')
  }

  async getLogs(params?: { limit?: number; offset?: number; source?: string }): Promise<EventLog[]> {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    if (params?.offset) searchParams.set('offset', params.offset.toString())
    if (params?.source) searchParams.set('source', params.source)

    const query = searchParams.toString()
    return this.request<EventLog[]>(`/logs${query ? `?${query}` : ''}`)
  }

  async getVersion(): Promise<VersionInfo> {
    return this.request<VersionInfo>('/version')
  }
}

export const api = new ApiClient()

// WebSocket connection for live updates
export class WebSocketClient {
  private ws: WebSocket | null = null
  private url: string
  private reconnectTimeout: number = 5000
  private handlers: Map<string, ((data: unknown) => void)[]> = new Map()

  constructor(url: string = `ws://${window.location.host}/api/ws`) {
    this.url = url
  }

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) return

    this.ws = new WebSocket(this.url)

    this.ws.onopen = () => {
      console.log('WebSocket connected')
      this.emit('connected', null)
    }

    this.ws.onclose = () => {
      console.log('WebSocket disconnected')
      this.emit('disconnected', null)
      // Reconnect
      setTimeout(() => this.connect(), this.reconnectTimeout)
    }

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        this.emit(data.type || 'message', data)
      } catch {
        console.error('Failed to parse WebSocket message')
      }
    }
  }

  disconnect(): void {
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
  }

  on(event: string, handler: (data: unknown) => void): void {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, [])
    }
    this.handlers.get(event)!.push(handler)
  }

  off(event: string, handler: (data: unknown) => void): void {
    const handlers = this.handlers.get(event)
    if (handlers) {
      const index = handlers.indexOf(handler)
      if (index > -1) handlers.splice(index, 1)
    }
  }

  private emit(event: string, data: unknown): void {
    const handlers = this.handlers.get(event) || []
    handlers.forEach((handler) => handler(data))
  }
}

export const wsClient = new WebSocketClient()
