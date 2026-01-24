<script setup lang="ts">
defineProps<{
  title: string
  connected: boolean
  details?: string
}>()
</script>

<template>
  <div class="card">
    <div class="card-content">
      <div class="media">
        <div class="media-left">
          <span
            class="status-indicator"
            :class="connected ? 'connected' : 'disconnected'"
          ></span>
        </div>
        <div class="media-content">
          <p class="title is-5">{{ title }}</p>
          <p class="subtitle is-6" :class="connected ? 'has-text-success' : 'has-text-danger'">
            {{ connected ? 'Connected' : 'Disconnected' }}
          </p>
        </div>
      </div>
      <p v-if="details" class="has-text-grey is-size-7 mt-2">
        {{ details }}
      </p>
    </div>
  </div>
</template>

<style scoped>
.status-indicator {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  display: inline-block;
  margin-top: 4px;
}

.status-indicator.connected {
  background-color: #48c774;
  box-shadow: 0 0 8px rgba(72, 199, 116, 0.5);
}

.status-indicator.disconnected {
  background-color: #f14668;
}
</style>
