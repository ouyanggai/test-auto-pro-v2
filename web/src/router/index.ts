import { createRouter, createWebHistory } from 'vue-router'

const PlansView = () => import('../views/PlansView.vue')
const NewPlanView = () => import('../views/NewPlanView.vue')
const RunsView = () => import('../views/RunsView.vue')
const SettingsView = () => import('../views/SettingsView.vue')

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
