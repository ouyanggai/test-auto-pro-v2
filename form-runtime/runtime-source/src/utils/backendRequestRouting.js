const installedBrowsers = new WeakSet();
const legacyBackendHosts = new Set(['iserver.runshihua.com']);

function isLegacyBackendHost(hostname) {
  return legacyBackendHosts.has(hostname) || /^192\.168\.1\.\d+$/.test(hostname);
}

let routingSettings = { apiBaseUrl: '' };
const configuredBackendOrigins = new Set();

function normalizeApiBaseUrl(value) {
  return String(value || '').trim().replace(/\/+$/, '');
}

function getAbsoluteOrigin(value) {
  try {
    return new URL(value).origin;
  } catch (_error) {
    return '';
  }
}

// FormMaking stores absolute API URLs in form definitions. Resolve only backend
// /api requests so file previews, OnlyOffice and unrelated services stay intact.
export function routeBackendRequestUrl(value, settings = routingSettings) {
  if (typeof value !== 'string' || !/^(?:https?:)?\/\//i.test(value)) return value;

  let source;
  let target;
  try {
    source = new URL(value, 'http://localhost');
    target = normalizeApiBaseUrl(settings.apiBaseUrl);
  } catch (_error) {
    return value;
  }
  if (!target) return value;

  const path = source.pathname || '/';
  const isLegacyBackend = isLegacyBackendHost(source.hostname);
  const targetOrigin = getAbsoluteOrigin(target);
  const isConfiguredBackend = configuredBackendOrigins.has(source.origin) ||
    (targetOrigin && source.origin === targetOrigin);
  if (!isLegacyBackend && !isConfiguredBackend) return value;

  let apiPrefix = '/api';
  if (!isLegacyBackend && isConfiguredBackend && targetOrigin) {
    try {
      apiPrefix = new URL(target).pathname.replace(/\/+$/, '') || '/api';
    } catch (_error) {
      apiPrefix = '/api';
    }
  }
  if (path !== apiPrefix && !path.startsWith(`${apiPrefix}/`)) return value;

  let suffix = path.slice(apiPrefix.length);
  if (target.endsWith('/api/v2') && (suffix === '/v2' || suffix.startsWith('/v2/'))) {
    suffix = suffix.slice('/v2'.length);
  }
  if (/^(?:https?:)?\/\//i.test(target)) {
    return `${target}${suffix}${source.search}${source.hash}`;
  }
  return `${target}${suffix}${source.search}${source.hash}`;
}

export function setBackendRequestRoutingConfig(config = {}) {
  if (config.apiBaseUrl !== undefined) {
    routingSettings = { ...routingSettings, apiBaseUrl: config.apiBaseUrl };
    const origin = getAbsoluteOrigin(config.apiBaseUrl);
    if (origin) configuredBackendOrigins.add(origin);
  }
}

export function installBackendRequestRouting(browser, config = {}) {
  if (!browser || installedBrowsers.has(browser)) return;
  setBackendRequestRoutingConfig(config);
  installedBrowsers.add(browser);

  const xhrPrototype = browser.XMLHttpRequest && browser.XMLHttpRequest.prototype;
  if (xhrPrototype && typeof xhrPrototype.open === 'function') {
    const originalOpen = xhrPrototype.open;
    xhrPrototype.open = function (...args) {
      if (typeof args[1] === 'string') {
        args[1] = routeBackendRequestUrl(args[1]);
      } else if (browser.URL && args[1] instanceof browser.URL) {
        args[1] = routeBackendRequestUrl(String(args[1]));
      }
      return originalOpen.apply(this, args);
    };
  }

  if (typeof browser.fetch === 'function') {
    const originalFetch = browser.fetch;
    browser.fetch = function (input, init) {
      if (typeof input === 'string' || (browser.URL && input instanceof browser.URL)) {
        return originalFetch.call(this, routeBackendRequestUrl(String(input)), init);
      }
      if (browser.Request && input instanceof browser.Request) {
        const routedUrl = routeBackendRequestUrl(input.url);
        if (routedUrl !== input.url) {
          const absoluteUrl = new browser.URL(routedUrl, browser.location.href).href;
          return originalFetch.call(this, new browser.Request(absoluteUrl, input), init);
        }
      }
      return originalFetch.call(this, input, init);
    };
  }
}
