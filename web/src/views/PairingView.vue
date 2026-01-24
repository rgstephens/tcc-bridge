<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useThermostatStore } from '../stores/thermostat'
import QRCodeVue from 'qrcode.vue'

const store = useThermostatStore()
const loading = ref(true)
let pollInterval: number | undefined

onMounted(async () => {
  loading.value = true
  await Promise.all([
    store.fetchPairing(),
    store.fetchStatus(),
  ])
  loading.value = false

  // Poll for pairing info until available (in case of startup timing)
  if (!store.pairing?.qr_code && store.isMatterRunning) {
    pollInterval = window.setInterval(async () => {
      await store.fetchPairing()
      if (store.pairing?.qr_code) {
        clearInterval(pollInterval)
        pollInterval = undefined
      }
    }, 2000)
  }
})

onUnmounted(() => {
  if (pollInterval) {
    clearInterval(pollInterval)
  }
})
</script>

<template>
  <div class="pairing-view">
    <h1 class="title">HomeKit Pairing</h1>

    <!-- Loading State -->
    <div v-if="loading" class="has-text-centered py-5">
      <p>Loading pairing information...</p>
    </div>

    <!-- Commissioning Status -->
    <div v-else-if="store.isCommissioned" class="notification is-success">
      <p class="is-size-5">
        <strong>Device is paired with HomeKit!</strong>
      </p>
      <p class="mt-2">
        Your thermostat should now appear in the Apple Home app.
      </p>
    </div>

    <div v-else-if="!store.isMatterRunning" class="notification is-warning">
      <p class="is-size-5">
        <strong>Matter bridge is not running</strong>
      </p>
      <p class="mt-2">
        The Matter bridge service needs to be started before pairing.
        Check the server logs for any errors.
      </p>
    </div>

    <!-- QR Code Card -->
    <template v-else>
      <div v-if="store.pairing?.qr_code" class="card">
        <header class="card-header">
          <p class="card-header-title">Scan to Pair</p>
        </header>
        <div class="card-content">
          <div class="qr-code-container">
            <QRCodeVue
              :value="store.pairing.qr_code"
              :size="250"
              level="M"
            />
          </div>

          <div class="has-text-centered mt-4">
            <p class="is-size-7 has-text-grey">
              Open the Home app on your iPhone or iPad and tap "Add Accessory"
            </p>
          </div>
        </div>
      </div>

      <div v-else class="notification is-info is-light">
        <p>Waiting for pairing code generation...</p>
        <p class="is-size-7 has-text-grey mt-2">
          The Matter bridge is starting up. This may take a few seconds.
        </p>
      </div>

      <!-- Manual Pairing Code -->
      <div v-if="store.pairing?.manual_pair_code" class="card mt-4">
        <header class="card-header">
          <p class="card-header-title">Manual Pairing Code</p>
        </header>
        <div class="card-content">
          <p class="has-text-centered mb-3">
            If you can't scan the QR code, enter this code manually:
          </p>
          <div class="pairing-code">
            {{ store.pairing.manual_pair_code }}
          </div>
        </div>
      </div>
    </template>

    <!-- Instructions -->
    <div class="card mt-5">
      <header class="card-header">
        <p class="card-header-title">Pairing Instructions</p>
      </header>
      <div class="card-content content">
        <ol>
          <li>Open the <strong>Home</strong> app on your iPhone or iPad</li>
          <li>Tap the <strong>+</strong> button in the top right</li>
          <li>Select <strong>Add Accessory</strong></li>
          <li>Point your camera at the QR code above</li>
          <li>Follow the on-screen instructions to complete setup</li>
        </ol>
        <p class="has-text-grey is-size-7 mt-4">
          Note: Your iPhone/iPad and this device must be on the same network.
        </p>
      </div>
    </div>
  </div>
</template>
