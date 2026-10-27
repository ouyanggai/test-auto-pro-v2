import { localstorageGet, localstorageRemove, localstorageSet } from '@/utils/auth';

const storagePrefix = 'invest-power-system';
const runtimeKeyConfig = {
  sid: {
    storageKey: 'token',
    readAliases: ['sid', 'token', `${storagePrefix}-token`],
    writeAliases: ['sid', 'token']
  },
  customerCode: {
    storageKey: 'customerCode',
    readAliases: ['customerCode', `${storagePrefix}-customerCode`],
    writeAliases: ['customerCode']
  },
  userId: {
    storageKey: 'userId',
    readAliases: ['userId', `${storagePrefix}-userId`],
    writeAliases: []
  },
  userName: {
    storageKey: 'userName',
    readAliases: ['userName', `${storagePrefix}-userName`],
    writeAliases: []
  },
  companyId: {
    storageKey: 'companyId',
    readAliases: ['companyId', `${storagePrefix}-companyId`],
    writeAliases: []
  },
  companyName: {
    storageKey: 'companyName',
    readAliases: ['companyName', `${storagePrefix}-companyName`],
    writeAliases: []
  },
  userDepartmentName: {
    storageKey: 'userDepartmentName',
    readAliases: ['userDepartmentName', `${storagePrefix}-userDepartmentName`],
    writeAliases: []
  },
  topCompanyId: {
    storageKey: 'topCompanyId',
    readAliases: ['topCompanyId', `${storagePrefix}-topCompanyId`],
    writeAliases: []
  },
  groupDepartment: {
    storageKey: 'groupDepartment',
    readAliases: ['groupDepartment', `${storagePrefix}-groupDepartment`],
    writeAliases: []
  }
};

function setStorageValue(key, value) {
  if (!key) return;
  if (value === undefined || value === null || value === '') {
    window.localStorage.removeItem(key);
    return;
  }

  let nextValue = value;
  if (typeof nextValue !== 'string') {
    nextValue = JSON.stringify(nextValue);
  }
  window.localStorage.setItem(key, nextValue);
}

function setScopedStorageValue(key, value) {
  if (!key) return;
  if (value === undefined || value === null || value === '') {
    localstorageRemove(key);
    return;
  }
  localstorageSet(key, value);
}

function readRuntimeValue(key) {
  const keyConfig = runtimeKeyConfig[key];
  if (!keyConfig) return '';

  const locationValue = getLocationParam(key);
  if (locationValue) return locationValue;

  const scopedValue = localstorageGet(keyConfig.storageKey);
  if (scopedValue) return scopedValue;

  for (const aliasKey of keyConfig.readAliases) {
    const aliasValue = window.localStorage.getItem(aliasKey);
    if (aliasValue) {
      return aliasValue;
    }
  }

  return '';
}

export function getLocationParam(name) {
  if (typeof window === 'undefined') return '';

  try {
    const searchParams = new URLSearchParams(window.location.search);
    const searchValue = searchParams.get(name);
    if (searchValue) return searchValue;

    const hash = window.location.hash || '';
    if (hash.includes('?')) {
      const hashParams = new URLSearchParams(hash.split('?')[1]);
      const hashValue = hashParams.get(name);
      if (hashValue) return hashValue;
    }
  } catch (_error) {
    return '';
  }

  return '';
}

export function persistRuntimeAuth(payload = {}) {
  Object.keys(runtimeKeyConfig).forEach((field) => {
    if (!Object.prototype.hasOwnProperty.call(payload, field)) return;

    const value = payload[field];
    const keyConfig = runtimeKeyConfig[field];

    setScopedStorageValue(keyConfig.storageKey, value);
    keyConfig.writeAliases.forEach((key) => setStorageValue(key, value));
  });
}

export function syncRuntimeAuthUrl(payload = {}) {
  const url = new URL(window.location.href);
  const hashIndex = url.hash.indexOf('?');
  const hashPath = hashIndex < 0 ? url.hash : url.hash.slice(0, hashIndex);
  const hashParams = new URLSearchParams(hashIndex < 0 ? '' : url.hash.slice(hashIndex + 1));
  Object.keys(payload).forEach(field => {
    if (!payload[field]) return;
    if (!hashPath.startsWith('#/')) {
      url.searchParams.set(field, payload[field]);
      return;
    }
    // 同时更新 search 和 hash 中已有的参数，避免旧 URL 在刷新后覆盖缓存。
    if (url.searchParams.has(field)) url.searchParams.set(field, payload[field]);
    if (hashParams.has(field) || !url.searchParams.has(field)) hashParams.set(field, payload[field]);
  });
  const query = hashParams.toString();
  url.hash = `${hashPath}${query ? `?${query}` : ''}`;
  window.history.replaceState(window.history.state, '', url.toString());
}

export function clearRuntimeAuthAliases() {
  Object.values(runtimeKeyConfig).forEach((keyConfig) => {
    keyConfig.writeAliases.forEach((key) => window.localStorage.removeItem(key));
  });
}

export function getRuntimeAuth() {
  const runtimeAuth = {
    sid: readRuntimeValue('sid'),
    customerCode: readRuntimeValue('customerCode'),
    userId: readRuntimeValue('userId'),
    userName: readRuntimeValue('userName'),
    companyId: readRuntimeValue('companyId'),
    companyName: readRuntimeValue('companyName'),
    userDepartmentName: readRuntimeValue('userDepartmentName'),
    topCompanyId: readRuntimeValue('topCompanyId'),
    groupDepartment: readRuntimeValue('groupDepartment')
  };

  if (Object.values(runtimeAuth).some((value) => value)) {
    persistRuntimeAuth(runtimeAuth);
  }

  return runtimeAuth;
}

export function bootstrapRuntimeAuth() {
  return getRuntimeAuth();
}

export function setupRuntimeAuthSync(store) {
  bootstrapRuntimeAuth();

  store.watch(
    (state) => ({
      sid: state.user && state.user.token,
      customerCode: state.user && state.user.customerCode,
      userId: state.user && state.user.userId,
      userName: state.user && state.user.userName,
      companyId: state.user && state.user.companyId,
      companyName: state.user && state.user.companyName,
      userDepartmentName: state.user && state.user.userDepartmentName,
      groupDepartment: state.user && state.user.groupDepartment
    }),
    ({ sid, customerCode, userId, userName, companyId, companyName, userDepartmentName, groupDepartment }) => {
      if (sid || customerCode || userId || userName || companyId || companyName || userDepartmentName || groupDepartment) {
        persistRuntimeAuth({ sid, customerCode, userId, userName, companyId, companyName, userDepartmentName, groupDepartment });
      } else {
        clearRuntimeAuthAliases();
      }
    },
    { immediate: true }
  );
}
