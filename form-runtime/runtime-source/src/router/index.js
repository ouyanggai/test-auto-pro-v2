import Vue from 'vue';
import VueRouter from 'vue-router';

Vue.use(VueRouter);

// Some network gateways inject a cookie-check script that reloads the page
// after removing the hash. Move old hash links to the real pathname before
// creating the router so an already published portal URL remains usable.
function migrateLegacyHashRoute() {
  if (typeof window === 'undefined' || !window.location.hash.startsWith('#/')) {
    return;
  }

  const hashRoute = window.location.hash.slice(1);
  const queryIndex = hashRoute.indexOf('?');
  const pathname = queryIndex < 0 ? hashRoute : hashRoute.slice(0, queryIndex);
  if (!pathname.startsWith('/flow/')) {
    return;
  }

  const url = new URL(window.location.href);
  url.pathname = pathname;
  const hashParams = new URLSearchParams(queryIndex < 0 ? '' : hashRoute.slice(queryIndex + 1));
  hashParams.forEach((value, key) => url.searchParams.set(key, value));
  url.hash = '';
  window.history.replaceState(window.history.state, '', url.toString());
}

migrateLegacyHashRoute();

const routes = [
  {
    path: '/',
    redirect: '/flow/approve'
  },
  {
    path: '/flow/',
    redirect: '/flow/approve'
  },
  {
    path: '/flow/index.html',
    redirect: '/flow/approve'
  },
  {
    path: '/flow/approve',
    name: 'flowApprove',
    component: () => import('@/views/GroupApproveManage/index.vue'),
    meta: { title: '流程审批' }
  },
  {
    path: '/flow/approve/:tab',
    name: 'flowApproveTab',
    component: () => import('@/views/GroupApproveManage/index.vue'),
    meta: { title: '流程审批' }
  },
  {
    path: '/flow/cost',
    name: 'flowCost',
    component: () => import('@/views/GroupApproveManage/cost.vue'),
    meta: { title: '流程统计' }
  },
  {
    path: '/flow/initiate',
    name: 'flowInitiate',
    component: () => import('@/views/FlowInitiate.vue'),
    meta: { title: '发起流程' }
  },
  {
    path: '/flow/detail/:id',
    name: 'flowDetail',
    component: () => import('@/views/FlowDetail.vue'),
    meta: { title: '流程详情' }
  }
];

const router = new VueRouter({
  mode: 'history',
  base: '/',
  routes
});

export default router;
