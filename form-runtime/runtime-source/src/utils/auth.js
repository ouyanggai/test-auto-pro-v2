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

/*
 * 回跳地址上按需补一个 sid 和显示名。
 * ---------------------------------------------------------------------------
 * 登录成功时会写 `guanwang-token` cookie，但它的 domain 是 `.runshihua.com`
 * —— 只有官网也在 *.runshihua.com 下时，整页跳过去才读得到登录态。测试环境的
 * 官网如果跑在内网 IP（192.168.1.220 之类）上，那个 cookie 不会被带上，落地就是
 * 未登录。这时把 sid 拼在地址上兜底，官网的 site-core.js 会从 URL 接管它、
 * 并立刻把它从地址栏抹掉。
 * 同域下通常不拼：cookie 自己会过去，地址栏里少一个会话标识。但这里要先确认
 * cookie **真的在** —— `guanwang-token` 是会话 cookie（登录时没写 expires），
 * 关掉浏览器就没了；而本系统自己的登录态存在 localStorage（utils/axios.js 读的
 * 就是它），于是「本系统还登录着、cookie 已经没了」是常态。已登录用户被带回
 * 登录页时，守卫这条路不经过登录接口、不会重写 cookie，此时再按同域省掉 sid，
 * 官网落地后既没有 cookie 也没有 sid —— 表现成「bserver 已经登录了，跳回来
 * 还是未登录」。正常登录那条路因为刚写过 cookie，仍然不会往地址上拼 sid。
 *
 * 域判断用的是「以 runshihua.com 结尾」，不是 indexOf —— 后者会把
 * evil-runshihua.com 这种仿冒域名误判成自己人。子域与裸域都对。
 *
 * 放在这里是因为有两个调用点：登录成功后的回跳（Login.vue），以及已登录用户被
 * 带回登录页时的回跳（router/index.js 的守卫）。
 */
export const withSid = (url, sid) => {
  try {
    const u = new URL(url, window.location.href);
    const sameDomain = /(^|\.)runshihua\.com$/i.test(u.hostname);
    const hasTokenCookie = /(^|;\s*)guanwang-token=/.test(document.cookie);
    // sid 为空就不拼。原先无条件 set，读不到 cookie 时会拼出一个空的 `?sid=`：
    // 官网的 adoptSidFromUrl 拿到空值直接 return，等于没接管，落地仍是未登录
    // —— 表现成「跳回来了，但还是没登录」，比不拼更难查。
    if (sid && (!sameDomain || !hasTokenCookie)) {
      u.searchParams.set('sid', sid);
    }

    // 显示名一起捎过去，官网右上角拿它当「已登录」那三个字。
    //
    // 名字存的是本系统的 `invest-power-system-userName`，官网在**另一个源**上，
    // 它的 localStorage 读不到；而官网那边的 `guanwang-userName` cookie 只有
    // culturalWorld.html 那套登录会写，从本系统登录页回跳过去时没人写它。所以
    // 地址是唯一能把它递过去的渠道。
    //
    // 与上面的 sid 不同，这里**不做同域判断**：sid 同域下大多数时候靠 cookie
    // 就能过去，但 guanwang-userName 同域下同样没人写，加判断反而拿不到名字。
    // 官网接住后会把它落成 cookie（刷新时名字还在），并把参数从地址栏抹掉。
    const userName = localstorageGet('userName');
    if (userName) u.searchParams.set('userName', userName);

    return u.toString();
  } catch (e) {
    // 地址解析不了（极少见）就原样返回，让浏览器自己去处理
    return url;
  }
};
