import { createRouter, createWebHistory } from 'vue-router'

import PlansView from '../views/PlansView.vue'
import NewPlanView from '../views/NewPlanView.vue'
import RunsView from '../views/RunsView.vue'
import SettingsView from '../views/SettingsView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/plans' },
    { path: '/plans', component: PlansView },
    { path: '/plans/new', component: NewPlanView },
    { path: '/runs', component: RunsView },
    { path: '/settings', component: SettingsView },
  ],
})

export default router
