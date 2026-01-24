<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useThermostatStore } from '../stores/thermostat'

const store = useThermostatStore()

const username = ref('')
const password = ref('')
const testResult = ref<{ success: boolean; error?: string } | null>(null)
const saving = ref(false)
const testing = ref(false)

onMounted(async () => {
  await store.fetchConfig()
  if (store.config?.username) {
    username.value = store.config.username
  }
})

async function testConnection() {
  if (!username.value || !password.value) return

  testing.value = true
  testResult.value = null

  try {
    testResult.value = await store.testCredentials(username.value, password.value)
  } catch (e) {
    testResult.value = { success: false, error: e instanceof Error ? e.message : 'Test failed' }
  } finally {
    testing.value = false
  }
}

async function saveCredentials() {
  if (!username.value || !password.value) return

  saving.value = true

  try {
    await store.saveCredentials(username.value, password.value)
    password.value = ''
    testResult.value = null
  } catch {
    // Error is handled by store
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="config-view">
    <h1 class="title">Configuration</h1>

    <div class="card">
      <header class="card-header">
        <p class="card-header-title">TCC Credentials</p>
      </header>
      <div class="card-content">
        <div v-if="store.config?.has_credentials" class="notification is-success is-light mb-4">
          Credentials configured for: <strong>{{ store.config.username }}</strong>
        </div>

        <div class="field">
          <label class="label">Username (Email)</label>
          <div class="control">
            <input
              v-model="username"
              class="input"
              type="email"
              placeholder="your-email@example.com"
            />
          </div>
        </div>

        <div class="field">
          <label class="label">Password</label>
          <div class="control">
            <input
              v-model="password"
              class="input"
              type="password"
              placeholder="Enter your TCC password"
            />
          </div>
        </div>

        <!-- Test Result -->
        <div v-if="testResult" class="notification mt-4" :class="testResult.success ? 'is-success' : 'is-danger'">
          <template v-if="testResult.success">
            Connection successful! Credentials are valid.
          </template>
          <template v-else>
            Connection failed: {{ testResult.error }}
          </template>
        </div>

        <div class="field is-grouped mt-4">
          <div class="control">
            <button
              class="button is-primary"
              :class="{ 'is-loading': saving }"
              :disabled="!username || !password || saving"
              @click="saveCredentials"
            >
              Save Credentials
            </button>
          </div>
          <div class="control">
            <button
              class="button is-info"
              :class="{ 'is-loading': testing }"
              :disabled="!username || !password || testing"
              @click="testConnection"
            >
              Test Connection
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="card mt-5">
      <header class="card-header">
        <p class="card-header-title">About TCC Bridge</p>
      </header>
      <div class="card-content">
        <p>
          This bridge connects your Honeywell Total Connect Comfort thermostat
          to Apple HomeKit using the Matter protocol.
        </p>
        <ul class="mt-3">
          <li>Enter your TCC credentials above</li>
          <li>Pair with HomeKit using the QR code on the Pairing page</li>
          <li>Control your thermostat from the Home app</li>
        </ul>
      </div>
    </div>
  </div>
</template>
