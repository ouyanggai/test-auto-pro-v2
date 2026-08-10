/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-01-14 11:48:40
 */
const storageKey = 'invest-power-system';
export const localstorageSet = (name, content) => {
  name = `${storageKey}-${name}`;
  if (!name) return;
  if (typeof content !== 'string') {
    content = JSON.stringify(content);
  }
  window.localStorage.setItem(name, content);
};
export const localstorageGet = name => {
  name = `${storageKey}-${name}`;
  if (!name) return;
  return window.localStorage.getItem(name);
};

export const localstorageRemove = name => {
  name = `${storageKey}-${name}`;
  if (!name) return;
  return window.localStorage.removeItem(name);
};

const normalizeStorageValue = value => value == null ? '' : String(value);

const getCurrentPermissionOwner = () => ({
  userId: normalizeStorageValue(localstorageGet('userId')),
  customerCode: normalizeStorageValue(localstorageGet('customerCode')),
  customerFlag: normalizeStorageValue(localstorageGet('customerFlag') || localstorageGet('oldCustomerFlag'))
});

const isSamePermissionOwner = (currentOwner, cacheOwner = {}) => {
  return ['userId', 'customerCode', 'customerFlag'].every(key => {
    const currentValue = normalizeStorageValue(currentOwner[key]);
    const cacheValue = normalizeStorageValue(cacheOwner[key]);
    return !currentValue || !cacheValue || currentValue === cacheValue;
  });
};

export const clearPermissionCache = store => {
  localstorageRemove('originPermission');
  localstorageRemove('originPermissionOwner');
  if (store && typeof store.commit === 'function') {
    store.commit('permission/RESET_PERMISSION');
    store.commit('permission/SET_MENU', []);
  }
};

export const isLoginUserChanged = ({ userId, customerCode, customerFlag } = {}) => {
  const currentOwner = getCurrentPermissionOwner();
  const nextOwner = {
    userId: normalizeStorageValue(userId),
    customerCode: normalizeStorageValue(customerCode),
    customerFlag: normalizeStorageValue(customerFlag)
  };

  return !!(
    (nextOwner.userId && currentOwner.userId && nextOwner.userId !== currentOwner.userId) ||
    (nextOwner.customerCode && currentOwner.customerCode && nextOwner.customerCode !== currentOwner.customerCode) ||
    (nextOwner.customerFlag && currentOwner.customerFlag && nextOwner.customerFlag !== currentOwner.customerFlag)
  );
};

export const clearPermissionCacheOnUserChange = (userInfo = {}, store) => {
  const changed = isLoginUserChanged(userInfo);
  if (changed) {
    clearPermissionCache(store);
  }
  return changed;
};

export const setOriginPermissionCache = permissionList => {
  localstorageSet('originPermission', permissionList);
  localstorageSet('originPermissionOwner', getCurrentPermissionOwner());
};

export const getOriginPermissionCache = () => {
  const originPermission = localstorageGet('originPermission');
  if (!originPermission) return null;

  const ownerText = localstorageGet('originPermissionOwner');
  if (!ownerText) {
    localstorageRemove('originPermission');
    return null;
  }

  let owner;
  try {
    owner = JSON.parse(ownerText);
  } catch (e) {
    clearPermissionCache();
    return null;
  }

  if (!isSamePermissionOwner(getCurrentPermissionOwner(), owner)) {
    clearPermissionCache();
    return null;
  }

  try {
    return JSON.parse(originPermission);
  } catch (e) {
    clearPermissionCache();
    return null;
  }
};
