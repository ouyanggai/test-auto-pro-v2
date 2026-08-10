/**
 * iframe 认证初始化
 * 从 URL 参数或 postMessage 获取认证信息
 */
import { localstorageGet } from '@/utils/auth';
import store from '@/store';

export function isAuthenticated() {
  return !!localstorageGet('token');
}

export async function waitForAuth(timeout = 10000) {
  if (isAuthenticated()) return true;

  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve(false), timeout);

    const handler = (event) => {
      if (event.data && event.data.type === 'RSH_FLOW_AUTH') {
        clearTimeout(timer);
        window.removeEventListener('message', handler);
        resolve(true);
      }
    };

    window.addEventListener('message', handler);
  });
}
