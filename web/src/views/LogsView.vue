<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useThermostatStore } from '../stores/thermostat'
import LogTable from '../components/LogTable.vue'

const store = useThermostatStore()

const sourceFilter = ref<string>('')
const limit = ref(50)

async function loadLogs() {
  await store.fetchLogs({
    limit: limit.value,
    source: sourceFilter.value || undefined,
  })
}

onMounted(loadLogs)

watch([sourceFilter, limit], loadLogs)

function refresh() {
  loadLogs()
}
</script>

<template>
  <div class="logs-view">
    <h1 class="title">Event Logs</h1>

    <div class="card">
      <header class="card-header">
        <p class="card-header-title">Filters</p>
      </header>
      <div class="card-content">
        <div class="columns">
          <div class="column is-4">
            <div class="field">
              <label class="label">Source</label>
              <div class="control">
                <div class="select is-fullwidth">
                  <select v-model="sourceFilter">
                    <option value="">All Sources</option>
                    <option value="tcc">TCC</option>
                    <option value="matter">Matter</option>
                    <option value="homekit">HomeKit</option>
                    <option value="user">User</option>
                    <option value="system">System</option>
                  </select>
                </div>
              </div>
            </div>
          </div>
          <div class="column is-4">
            <div class="field">
              <label class="label">Limit</label>
              <div class="control">
                <div class="select is-fullwidth">
                  <select v-model="limit">
                    <option :value="25">25</option>
                    <option :value="50">50</option>
                    <option :value="100">100</option>
                    <option :value="200">200</option>
                  </select>
                </div>
              </div>
            </div>
          </div>
          <div class="column is-4">
            <div class="field">
              <label class="label">&nbsp;</label>
              <div class="control">
                <button class="button is-primary is-fullwidth" @click="refresh">
                  Refresh
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="card mt-4">
      <div class="card-content">
        <div v-if="store.loading" class="has-text-centered py-5">
          <p>Loading logs...</p>
        </div>
        <div v-else-if="store.logs.length === 0" class="has-text-centered py-5 has-text-grey">
          No log entries found
        </div>
        <LogTable v-else :logs="store.logs" />
      </div>
    </div>
  </div>
</template>
