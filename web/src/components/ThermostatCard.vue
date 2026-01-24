<script setup lang="ts">
import { ref } from 'vue'
import { useThermostatStore } from '../stores/thermostat'
import type { ThermostatState } from '../api/client'

const props = defineProps<{
  thermostat: ThermostatState
}>()

const store = useThermostatStore()

const heatSetpoint = ref(props.thermostat.heat_setpoint)
const coolSetpoint = ref(props.thermostat.cool_setpoint)
const updating = ref(false)

const modes = ['off', 'heat', 'cool', 'auto'] as const

async function setMode(mode: string) {
  if (mode === props.thermostat.system_mode) return
  updating.value = true
  try {
    await store.setMode(props.thermostat.device_id, mode)
  } finally {
    updating.value = false
  }
}

async function adjustHeatSetpoint(delta: number) {
  const newValue = Math.round((heatSetpoint.value + delta) * 10) / 10
  if (newValue < 10 || newValue > 32) return
  heatSetpoint.value = newValue
  updating.value = true
  try {
    await store.setSetpoint(props.thermostat.device_id, 'heat', newValue)
  } finally {
    updating.value = false
  }
}

async function adjustCoolSetpoint(delta: number) {
  const newValue = Math.round((coolSetpoint.value + delta) * 10) / 10
  if (newValue < 10 || newValue > 35) return
  coolSetpoint.value = newValue
  updating.value = true
  try {
    await store.setSetpoint(props.thermostat.device_id, 'cool', newValue)
  } finally {
    updating.value = false
  }
}

function formatTemp(temp: number): string {
  return temp.toFixed(1)
}
</script>

<template>
  <div class="card">
    <header class="card-header">
      <p class="card-header-title">
        {{ thermostat.name }}
        <span v-if="thermostat.is_heating" class="tag is-danger ml-2">Heating</span>
        <span v-if="thermostat.is_cooling" class="tag is-info ml-2">Cooling</span>
      </p>
    </header>
    <div class="card-content">
      <!-- Current Temperature -->
      <div class="has-text-centered mb-4">
        <div class="temperature-display">
          {{ formatTemp(thermostat.current_temp) }}
          <span class="temperature-unit">°C</span>
        </div>
        <p class="has-text-grey">
          Humidity: {{ thermostat.humidity }}%
        </p>
      </div>

      <!-- Mode Selection -->
      <div class="field">
        <label class="label">Mode</label>
        <div class="buttons mode-buttons">
          <button
            v-for="mode in modes"
            :key="mode"
            class="button"
            :class="{
              'is-primary': thermostat.system_mode === mode,
              'is-loading': updating,
            }"
            :disabled="updating"
            @click="setMode(mode)"
          >
            {{ mode.charAt(0).toUpperCase() + mode.slice(1) }}
          </button>
        </div>
      </div>

      <!-- Heat Setpoint -->
      <div v-if="thermostat.system_mode === 'heat' || thermostat.system_mode === 'auto'" class="field">
        <label class="label">Heat Setpoint</label>
        <div class="setpoint-control">
          <button
            class="button"
            :disabled="updating || heatSetpoint <= 10"
            @click="adjustHeatSetpoint(-0.5)"
          >
            -
          </button>
          <span class="setpoint-value">{{ formatTemp(heatSetpoint) }}°C</span>
          <button
            class="button"
            :disabled="updating || heatSetpoint >= 32"
            @click="adjustHeatSetpoint(0.5)"
          >
            +
          </button>
        </div>
      </div>

      <!-- Cool Setpoint -->
      <div v-if="thermostat.system_mode === 'cool' || thermostat.system_mode === 'auto'" class="field">
        <label class="label">Cool Setpoint</label>
        <div class="setpoint-control">
          <button
            class="button"
            :disabled="updating || coolSetpoint <= 10"
            @click="adjustCoolSetpoint(-0.5)"
          >
            -
          </button>
          <span class="setpoint-value">{{ formatTemp(coolSetpoint) }}°C</span>
          <button
            class="button"
            :disabled="updating || coolSetpoint >= 35"
            @click="adjustCoolSetpoint(0.5)"
          >
            +
          </button>
        </div>
      </div>

      <!-- Last Update -->
      <p class="has-text-grey is-size-7 mt-4">
        Last updated: {{ new Date(thermostat.updated_at).toLocaleString() }}
      </p>
    </div>
  </div>
</template>
