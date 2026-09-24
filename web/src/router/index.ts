import { createRouter, createWebHistory } from 'vue-router'

const PlansView = () => import('../views/PlansView.vue')
const NewPlanView = () => import('../views/NewPlanView.vue')
const EditPlanView = () => import('../views/EditPlanView.vue')
const PlanPathsView = () => import('../views/PlanPathsView.vue')
const PlanPathConfigurationView = () => import('../views/PlanPathConfigurationView.vue')
const RunsView = () => import('../views/RunsView.vue')
const RunPathsView = () => import('../views/RunPathsView.vue')
const RunDetailView = () => import('../views/RunDetailView.vue')
const SettingsView = () => import('../views/SettingsView.vue')
const FormRuntimeMaintenanceView = () => import('../views/FormRuntimeMaintenanceView.vue')
const AddSignExclusionView = () => import('../views/AddSignExclusionView.vue')
const ReportsView = () => import('../views/ReportsView.vue')
const ReportDetailView = () => import('../views/ReportDetailView.vue')
const PathReportView = () => import('../views/PathReportView.vue')

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/plans' },
    { path: '/plans', component: PlansView },
    { path: '/plans/new', component: NewPlanView },
    { path: '/plans/:id/edit', component: EditPlanView },
    { path: '/plans/:id/paths', component: PlanPathsView },
    { path: '/plans/:planId/paths/:pathId/configure', component: PlanPathConfigurationView },
    { path: '/runs', component: RunsView },
    // 三层导航（2026-09-06）：运行列表 → 本次运行的执行路径 → 运行面板。
    { path: '/runs/:runId', component: RunPathsView },
    { path: '/runs/:runId/paths/:pathRunId', component: RunDetailView },
    { path: '/reports', component: ReportsView },
    { path: '/reports/:id', component: ReportDetailView },
    { path: '/reports/:planId/paths/:pathId', component: PathReportView },
    { path: '/settings', component: SettingsView },
    { path: '/settings/reports', redirect: '/reports' },
    { path: '/settings/reports/:id', redirect: (to) => `/reports/${to.params.id}` },
    { path: '/settings/form-runtime', component: FormRuntimeMaintenanceView },
    { path: '/settings/add-sign-exclusion', component: AddSignExclusionView },
  ],
})

export default router
