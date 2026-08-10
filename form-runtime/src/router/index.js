import Vue from 'vue';
import VueRouter from 'vue-router';

Vue.use(VueRouter);

const routes = [
  {
    path: '/',
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
  mode: 'hash',
  base: '/flow/',
  routes
});

export default router;
