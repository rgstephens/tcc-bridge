import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api, wsClient, type SystemStatus, type ThermostatState, type ConfigStatus, type PairingInfo, type EventLog } from '../api/client'

export const useThermostatStore = defineStore('thermostat', () => {
  // State
  const status = ref<SystemStatus | null>(null)
  const thermostats = ref<ThermostatState[]>([])
  const config = ref<ConfigStatus | null>(null)
  const pairing = ref<PairingInfo | null>(null)
  const logs = ref<EventLog[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const wsConnected = ref(false)

  // Computed
  const primaryThermostat = computed(() => thermostats.value[0] || null)
  const isTCCConnected = computed(() => status.value?.tcc.connected ?? false)
  const isMatterRunning = computed(() => status.value?.matter.running ?? false)
  const isCommissioned = computed(() => status.value?.matter.commissioned ?? false)
  const isConfigured = computed(() => status.value?.configured ?? false)

  // Actions
  async function fetchStatus() {
    try {
      status.value = await api.getStatus()
    } catch (e) {
      console.error('Failed to fetch status:', e)
    }
  }

  async function fetchThermostats() {
    try {
      loading.value = true
      thermostats.value = (await api.getThermostat()) ?? []
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch thermostats'
    } finally {
      loading.value = false
    }
  }

  async function setSetpoint(deviceId: number, type: 'heat' | 'cool', value: number) {
    try {
      loading.value = true
      await api.setSetpoint(deviceId, type, value)
      await fetchThermostats()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to set setpoint'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function setMode(deviceId: number, mode: string) {
    try {
      loading.value = true
      await api.setMode(deviceId, mode)
      await fetchThermostats()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to set mode'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchConfig() {
    try {
      config.value = await api.getConfig()
    } catch (e) {
      console.error('Failed to fetch config:', e)
    }
  }

  async function saveCredentials(username: string, password: string) {
    try {
      loading.value = true
      await api.saveCredentials(username, password)
      await fetchConfig()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to save credentials'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function testCredentials(username: string, password: string) {
    try {
      loading.value = true
      const result = await api.testCredentials(username, password)
      return result
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to test credentials'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchPairing() {
    try {
      pairing.value = await api.getPairing()
    } catch (e) {
      console.error('Failed to fetch pairing:', e)
    }
  }

  async function decommission() {
    try {
      loading.value = true
      await api.decommission()
      await Promise.all([
        fetchStatus(),
        fetchPairing(),
      ])
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to decommission device'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchLogs(params?: { limit?: number; offset?: number; source?: string }) {
    try {
      loading.value = true
      logs.value = await api.getLogs(params)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch logs'
    } finally {
      loading.value = false
    }
  }

  function connectWebSocket() {
    wsClient.on('connected', () => {
      wsConnected.value = true
    })

    wsClient.on('disconnected', () => {
      wsConnected.value = false
    })

    wsClient.on('thermostat_update', (data) => {
      const message = data as { type: string; data: ThermostatState }
      const update = message.data
      const index = thermostats.value.findIndex((t) => t.device_id === update.device_id)
      if (index > -1) {
        thermostats.value[index] = update
      } else {
        thermostats.value.push(update)
      }
    })

    wsClient.on('status_update', (data) => {
      const message = data as { type: string; data: SystemStatus }
      status.value = message.data
    })

    wsClient.on('matter_decommissioned', () => {
      fetchStatus()
      fetchPairing()
    })

    wsClient.connect()
  }

  function disconnectWebSocket() {
    wsClient.disconnect()
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    status,
    thermostats,
    config,
    pairing,
    logs,
    loading,
    error,
    wsConnected,

    // Computed
    primaryThermostat,
    isTCCConnected,
    isMatterRunning,
    isCommissioned,
    isConfigured,

    // Actions
    fetchStatus,
    fetchThermostats,
    setSetpoint,
    setMode,
    fetchConfig,
    saveCredentials,
    testCredentials,
    fetchPairing,
    decommission,
    fetchLogs,
    connectWebSocket,
    disconnectWebSocket,
    clearError,
  }
})
