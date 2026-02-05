<script setup lang="ts">
import type { EventLog } from '../api/client'

defineProps<{
  logs: EventLog[]
}>()

function formatTimestamp(timestamp: string): string {
  return new Date(timestamp).toLocaleString()
}

function getSourceClass(source: string): string {
  switch (source) {
    case 'tcc':
      return 'is-info'
    case 'matter':
      return 'is-primary'
    case 'homekit':
      return 'is-link'
    case 'user':
      return 'is-success'
    case 'system':
      return 'is-warning'
    default:
      return 'is-light'
  }
}

function getEventTypeClass(eventType: string): string {
  switch (eventType) {
    case 'error':
      return 'has-text-danger'
    case 'temp_change':
    case 'mode_change':
      return 'has-text-info'
    case 'connection':
    case 'commissioning':
      return 'has-text-success'
    default:
      return ''
  }
}
</script>

<template>
  <div class="table-container">
    <table class="table is-fullwidth is-striped log-table">
      <thead>
        <tr>
          <th style="width: 160px">Timestamp</th>
          <th style="width: 80px">Source</th>
          <th style="width: 100px">Type</th>
          <th>Message</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="log in logs" :key="log.id">
          <td class="timestamp">{{ formatTimestamp(log.timestamp) }}</td>
          <td>
            <span class="tag source" :class="getSourceClass(log.source)">
              {{ log.source }}
            </span>
          </td>
          <td :class="getEventTypeClass(log.event_type)">
            {{ log.event_type }}
          </td>
          <td>{{ log.message }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.table-container {
  overflow-x: auto;
}

.log-table {
  font-size: 0.9rem;
}

.timestamp {
  white-space: nowrap;
  color: #7a7a7a;
  font-family: monospace;
  font-size: 0.85rem;
}

.source {
  text-transform: uppercase;
  font-size: 0.7rem;
  font-weight: 600;
}
</style>
