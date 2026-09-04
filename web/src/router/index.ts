import { createRouter, createWebHistory } from 'vue-router'

const PlansView = () => import('../views/PlansView.vue')
const NewPlanView = () => import('../views/NewPlanView.vue')
const PlanPathsView = () => import('../views/PlanPathsView.vue')
const PlanPathConfigurationView = () => import('../views/PlanPathConfigurationView.vue')
const RunsView = () => import('../views/RunsView.vue')
const RunDetailView = () => import('../views/RunDetailView.vue')
const SettingsView = () => import('../views/SettingsView.vue')
const FormRuntimeMaintenanceView = () => import('../views/FormRuntimeMaintenanceView.vue')

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/plans' },
    { path: '/plans', component: PlansView },
    { path: '/plans/new', component: NewPlanView },
    { path: '/plans/:id/paths', component: PlanPathsView },
    { path: '/plans/:planId/paths/:pathId/configure', component: PlanPathConfigurationView },
    { path: '/runs', component: RunsView },
    { path: '/runs/:runId', component: RunDetailView },
    { path: '/settings', component: SettingsView },
    { path: '/settings/form-runtime', component: FormRuntimeMaintenanceView },
  ],
})

export default router
