import { createRouter, createWebHistory } from 'vue-router'
import StatusView from '../views/StatusView.vue'
import ConfigView from '../views/ConfigView.vue'
import PairingView from '../views/PairingView.vue'
import LogsView from '../views/LogsView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'status',
      component: StatusView,
    },
    {
      path: '/config',
      name: 'config',
      component: ConfigView,
    },
    {
      path: '/pairing',
      name: 'pairing',
      component: PairingView,
    },
    {
      path: '/logs',
      name: 'logs',
      component: LogsView,
    },
  ],
})

export default router
