/**
 * iframe postMessage 适配器
 * 处理父窗口与流程应用之间的通信
 */
import { setConfig } from '@/config/env';
import { getLocationParam, persistRuntimeAuth } from '@/utils/runtimeAuthSync';
import store from '@/store';
import router from '@/router';

const MESSAGE_TYPES = {
  // Parent → iframe
  AUTH: 'RSH_FLOW_AUTH',
  CONFIG: 'RSH_FLOW_CONFIG',
  NAVIGATE: 'RSH_FLOW_NAVIGATE',
  // iframe → Parent
  READY: 'RSH_FLOW_READY',
  AUTH_EXPIRED: 'RSH_FLOW_AUTH_EXPIRED',
  EVENT: 'RSH_FLOW_EVENT',
  RESIZE: 'RSH_FLOW_RESIZE'
};

function sendToParent(type, data = {}) {
  if (window.parent !== window) {
    window.parent.postMessage({ type, ...data }, '*');
  }
}

function handleAuth(payload) {
  const {
    sid,
    customerCode,
    userId,
    userName,
    companyId,
    companyName,
    topCompanyId,
    groupDepartment,
    userData
  } = payload;

  const nextCustomerCode = customerCode || userData?.customerCode;
  const nextUserId = userId || userData?.userId || userData?.id;
  const nextUserName = userName || userData?.displayName || userData?.name || userData?.userName;
  const nextCompanyId = companyId || userData?.companyId;
  const nextCompanyName = companyName || userData?.companyName || userData?.companyVo?.name;
  const nextTopCompanyId = topCompanyId || userData?.topCompanyId || userData?.flag;
  const nextGroupDepartment = groupDepartment || userData?.groupDepartment;

  persistRuntimeAuth({
    sid,
    customerCode: nextCustomerCode,
    userId: nextUserId,
    userName: nextUserName,
    companyId: nextCompanyId,
    companyName: nextCompanyName,
    topCompanyId: nextTopCompanyId,
    groupDepartment: nextGroupDepartment
  });

  if (sid) {
    store.commit('user/SET_TOKEN', sid);
  }
  if (nextCustomerCode) {
    store.commit('user/SET_CUSTOMERCODE', nextCustomerCode);
  }
  if (nextUserId) {
    store.commit('user/SET_USER_ID', nextUserId);
  }
  if (nextUserName) {
    store.commit('user/SET_USER_NAME', nextUserName);
  }
  if (nextCompanyId) {
    store.commit('user/SET_COMPANY_ID', nextCompanyId);
  }
  if (nextCompanyName) {
    store.commit('user/SET_COMPANY_NAME', nextCompanyName);
  }
  if (nextGroupDepartment) {
    store.commit('user/SET_GROUP_DEPARTMENT', nextGroupDepartment);
  }
  if (userData) {
    if (store._mutations['user/SET_USER_DATA']) {
      store.commit('user/SET_USER_DATA', userData);
    }
  }
}

function handleConfig(payload) {
  setConfig(payload);
}

function handleNavigate(payload) {
  const { path, query, params } = payload;
  if (path) {
    router.push({ path, query, params }).catch(() => {});
  }
}

function initFromUrlParams() {
  const sid = getLocationParam('sid');
  const customerCode = getLocationParam('customerCode');
  const userId = getLocationParam('userId');
  const userName = getLocationParam('userName');
  const companyId = getLocationParam('companyId');
  const companyName = getLocationParam('companyName');
  const topCompanyId = getLocationParam('topCompanyId');
  const groupDepartment = getLocationParam('groupDepartment');
  const baseUrl = getLocationParam('baseUrl');
  const route = getLocationParam('route');

  if (sid || customerCode || userId || userName || companyId || companyName || topCompanyId || groupDepartment) {
    handleAuth({
      sid,
      customerCode,
      userId,
      userName,
      companyId,
      companyName,
      topCompanyId,
      groupDepartment
    });
  }
  if (baseUrl) {
    handleConfig({ baseUrl });
  }
  if (route) {
    handleNavigate({ path: route });
  }
}

export function initPostMessage() {
  // 监听来自父窗口的消息
  window.addEventListener('message', (event) => {
    const { type, ...payload } = event.data || {};

    switch (type) {
      case MESSAGE_TYPES.AUTH:
        handleAuth(payload);
        break;
      case MESSAGE_TYPES.CONFIG:
        handleConfig(payload);
        break;
      case MESSAGE_TYPES.NAVIGATE:
        handleNavigate(payload);
        break;
    }
  });

  // 支持 URL 参数降级
  initFromUrlParams();

  // 通知父窗口已就绪
  sendToParent(MESSAGE_TYPES.READY);
}

export function notifyAuthExpired() {
  sendToParent(MESSAGE_TYPES.AUTH_EXPIRED);
}

export function notifyFlowEvent(eventName, data) {
  sendToParent(MESSAGE_TYPES.EVENT, { eventName, data });
}

export function notifyResize() {
  sendToParent(MESSAGE_TYPES.RESIZE, {
    height: document.documentElement.scrollHeight
  });
}

export { MESSAGE_TYPES };
