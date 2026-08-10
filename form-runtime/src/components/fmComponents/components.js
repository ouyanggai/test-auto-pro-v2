// import demo from './pages/demo.vue';
export default {
  demo: () => import('./pages/demo.vue'),
  costAllocationAdjustment: () => import('./pages/costAllocationAdjustment.vue'),
  questionnaireSurvey: () => import('./pages/questionnaireSurvey.vue'),
  QuarterPerfAssess: () => import('./pages/QuarterPerfAssess.vue'),
  QuarterPerfScore: () => import('./pages/QuarterPerfScore.vue'),
  QuarterPerfRating: () => import('./pages/QuarterPerfRating.vue'),
};
