<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useThermostatStore } from '../stores/thermostat'
import ThermostatCard from '../components/ThermostatCard.vue'
import ConnectionStatus from '../components/ConnectionStatus.vue'

const store = useThermostatStore()

let pollInterval: number | undefined

onMounted(async () => {
  await Promise.all([
    store.fetchStatus(),
    store.fetchThermostats(),
  ])
  store.connectWebSocket()

  // Poll for updates
  pollInterval = window.setInterval(async () => {
    await store.fetchStatus()
    await store.fetchThermostats()
  }, 30000)
})

onUnmounted(() => {
  if (pollInterval) {
    clearInterval(pollInterval)
  }
})
</script>

<template>
  <div class="status-view">
    <h1 class="title">System Status</h1>

    <!-- Not Configured Warning -->
    <div v-if="!store.isConfigured" class="notification is-warning mb-5">
      <p class="is-size-5"><strong>Setup Required</strong></p>
      <p class="mt-2">
        Please configure your TCC credentials to connect to your thermostat.
      </p>
      <router-link to="/config" class="button is-primary mt-3">
        Configure Credentials
      </router-link>
    </div>

    <!-- Connection Status Cards -->
    <div class="columns">
      <div class="column is-4">
        <ConnectionStatus
          title="TCC Connection"
          :connected="store.isTCCConnected"
          :details="store.isConfigured ? (store.isTCCConnected ? 'Connected' : 'Disconnected') : 'Not configured'"
        />
      </div>
      <div class="column is-4">
        <ConnectionStatus
          title="Matter Bridge"
          :connected="store.isMatterRunning"
          :details="store.isMatterRunning ? 'Running' : 'Not running'"
        />
      </div>
      <div class="column is-4">
        <ConnectionStatus
          title="HomeKit"
          :connected="store.isCommissioned"
          :details="store.isCommissioned ? 'Paired' : 'Not paired'"
        />
      </div>
    </div>

    <!-- Thermostat Cards -->
    <h2 class="title is-4 mt-5">Thermostats</h2>

    <div v-if="store.loading" class="has-text-centered py-5">
      <span class="loader"></span>
      <p>Loading...</p>
    </div>

    <div v-else-if="!store.isConfigured" class="notification is-light">
      <p class="has-text-grey">Configure your TCC credentials to view thermostat data.</p>
    </div>

    <div v-else-if="store.thermostats.length === 0" class="notification is-info is-light">
      <p v-if="store.isTCCConnected">No thermostats found in your TCC account.</p>
      <p v-else>Waiting for TCC connection to retrieve thermostat data...</p>
      <p class="is-size-7 has-text-grey mt-2">
        Thermostat data is polled every 10 minutes to avoid rate limiting.
      </p>
    </div>

    <div v-else class="columns is-multiline">
      <div
        v-for="thermostat in store.thermostats"
        :key="thermostat.device_id"
        class="column is-6"
      >
        <ThermostatCard :thermostat="thermostat" />
      </div>
    </div>

    <!-- Error Display -->
    <div v-if="store.error" class="notification is-danger mt-4">
      <button class="delete" @click="store.clearError"></button>
      {{ store.error }}
    </div>
  </div>
</template>
