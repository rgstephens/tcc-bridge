<script setup lang="ts">
import { RouterView, RouterLink } from 'vue-router'
import { ref, onMounted } from 'vue'
import { api } from './api/client'

const version = ref('')
const buildDate = ref('')

onMounted(async () => {
  try {
    const info = await api.getVersion()
    version.value = info.version
    buildDate.value = info.build_date
  } catch (e) {
    console.error('Failed to fetch version:', e)
  }
})
</script>

<template>
  <div class="app">
    <nav class="navbar is-primary" role="navigation">
      <div class="navbar-brand">
        <RouterLink class="navbar-item" to="/">
          <strong>TCC Bridge</strong>
        </RouterLink>
      </div>

      <div class="navbar-menu">
        <div class="navbar-start">
          <RouterLink class="navbar-item" to="/">Status</RouterLink>
          <RouterLink class="navbar-item" to="/config">Configuration</RouterLink>
          <RouterLink class="navbar-item" to="/pairing">Pairing</RouterLink>
          <RouterLink class="navbar-item" to="/logs">Logs</RouterLink>
        </div>
      </div>
    </nav>

    <section class="section">
      <div class="container">
        <RouterView />
      </div>
    </section>

    <footer class="footer">
      <div class="content has-text-centered">
        <p class="is-size-7 has-text-grey">
          TCC Bridge v{{ version || 'dev' }}
          <span v-if="buildDate"> &middot; Built {{ buildDate }}</span>
        </p>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.navbar-item.router-link-active {
  background-color: rgba(255, 255, 255, 0.1);
}

.app {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

.section {
  flex: 1;
}

.footer {
  padding: 1rem;
  background-color: #f5f5f5;
}
</style>
