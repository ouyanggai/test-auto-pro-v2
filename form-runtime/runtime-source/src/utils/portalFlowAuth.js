import { baseUrl } from '@/config/env';
import store from '@/store';
import { getLocationParam, persistRuntimeAuth, syncRuntimeAuthUrl } from '@/utils/runtimeAuthSync';

const refreshes = new Map();

// 容器里的 /api 由门户提供，使用门户 HttpOnly 会话换 SID。
// 独立部署使用其他 API 地址时保留原来的登录方式。
export function usesPortalFlowAuth() {
  const api = new URL(baseUrl, window.location.href);
  return api.origin === window.location.origin && api.pathname.replace(/\/$/, '') === '/api';
}

export function isFlowAuthExpired(response) {
  const payload = response && response.data;
  return !!response && (response.status === 401 ||
    (payload && (['AUTH_401', 'RESP401'].includes(payload.code) ||
      (payload.isSuccess !== true && /SID\s*(已)?失效|登录上下文已失效/i.test(payload.message || '')))));
}

export function setFlowRequestSid(url, sid, platformCode) {
  const queryIndex = url.indexOf('?');
  const path = queryIndex < 0 ? url : url.slice(0, queryIndex);
  const query = new URLSearchParams(queryIndex < 0 ? '' : url.slice(queryIndex + 1));
  query.set('sid', sid);
  query.set('platformCode', platformCode);
  return `${path}?${query.toString()}`;
}

// 只重试一次；最终的业务错误交给原有响应处理，避免无限换 SID。
export async function requestWithSidRefresh(send, refresh) {
  let response;
  let requestError;
  try {
    response = await send();
  } catch (error) {
    if (!isFlowAuthExpired(error.response)) throw error;
    requestError = error;
  }
  if (!isFlowAuthExpired(requestError ? requestError.response : response)) return response;
  const context = await refresh();
  if (context) return send(context);
  if (requestError) throw requestError;
  return response;
}

async function readPortalJson(path, options = {}) {
  const response = await fetch(`/api/auth/${path}`, {
    credentials: 'same-origin',
    cache: 'no-store',
    ...options
  });
  const payload = await response.json();
  if (!response.ok) {
    if (response.status === 401) throw new Error('门户登录已过期，请重新登录门户');
    throw new Error(payload.error || payload.message || '门户登录上下文获取失败，请刷新后重试');
  }
  return payload;
}

async function resolvePortalContext(platformCode, customerCode) {
  const previousUserId = store.state.user.userId;
  const session = await readPortalJson('session-check');
  if (!session.active || !session.csrfToken) {
    throw new Error('门户登录已过期，请重新登录门户');
  }
  // 兼容已发布的 iframe URL：它们只有客户码，没有 appId。
  const { applications = [] } = await readPortalJson('applications');
  const candidates = applications.flatMap(app => [
    ...(app.flowEnabled ? [{ appId: app.id, platformCode: app.platformCode, customerCode: app.customerCode }] : []),
    ...(app.flowContextCandidates || [])
  ]).filter(candidate => String(candidate.platformCode) === platformCode && candidate.customerCode === customerCode);
  const appId = getLocationParam('appId');
  const candidate = candidates.find(item => item.appId === appId) || candidates[0];
  if (!candidate) throw new Error('未找到当前客户的流程平台，请从门户重新打开页面');

  const context = await readPortalJson('flow-context', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'x-csrf-token': session.csrfToken },
    body: JSON.stringify({ appId: candidate.appId, platformCode, customerCode })
  });
  if (!context.sid || !context.userId || context.platformCode !== platformCode || context.customerCode !== customerCode) {
    throw new Error('流程平台登录上下文无效，请刷新后重试');
  }
  if (previousUserId && context.userId !== previousUserId) {
    throw new Error('登录用户已变化，请从门户重新打开页面');
  }
  // 公司字段可能表示当前业务公司，换 SID 时只更新登录人员信息。
  persistRuntimeAuth({ sid: context.sid, userId: context.userId });
  store.commit('user/SET_TOKEN', context.sid);
  store.commit('user/SET_USER_ID', context.userId);
  syncRuntimeAuthUrl({ sid: context.sid, userId: context.userId });
  return context;
}

export function refreshPortalFlowAuth({ sid, platformCode, customerCode }) {
  if (!usesPortalFlowAuth()) return Promise.resolve(null);
  if (!customerCode) return Promise.reject(new Error('缺少流程客户码，请从门户重新打开页面'));
  platformCode = String(platformCode);
  const key = JSON.stringify([platformCode, customerCode, sid]);
  if (refreshes.has(key)) return refreshes.get(key);
  const pending = resolvePortalContext(platformCode, customerCode);
  refreshes.set(key, pending);
  pending.then(() => {
    // 短暂保留结果，供同批次稍晚返回的过期请求复用。
    setTimeout(() => refreshes.delete(key), 1000);
  }, () => refreshes.delete(key));
  return pending;
}
