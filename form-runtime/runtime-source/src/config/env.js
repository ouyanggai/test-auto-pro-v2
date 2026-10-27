// Build-time values are injected by Docker/Jenkins. Runtime config remains
// available for iframe hosts that need to override the API endpoint.
const stageDefaults = (() => {
  const isDev = process.env.NODE_ENV === 'development';
  const flag = process.env.VUE_APP_FLAG;

  if (isDev) {
    return {
      baseUrl: 'http://192.168.1.220:28081/api',
      viewFileUrl: 'http://192.168.1.220:28081',
      onlyOfficeUrl: 'http://192.168.1.218:8085'
    };
  }

  if (flag === 'dev') {
    return {
      baseUrl: 'http://192.168.1.218:8077/api',
      viewFileUrl: 'http://192.168.1.218:8077',
      onlyOfficeUrl: 'http://192.168.1.218:8085'
    };
  }

  if (flag === 'test') {
    return {
      baseUrl: 'http://192.168.1.220:38081/api',
      viewFileUrl: 'http://192.168.1.220:38081',
      onlyOfficeUrl: 'http://192.168.1.195:1080'
    };
  }

  return {
    baseUrl: 'https://iserver.runshihua.com/api',
    viewFileUrl: 'https://reader.runshihua.com',
    onlyOfficeUrl: 'https://office.runshihua.com'
  };
})();

const defaultConfig = {
  baseUrl: process.env.VUE_APP_BASE_URL || stageDefaults.baseUrl,
  viewFileUrl: process.env.VUE_APP_VIEW_FILE_URL || stageDefaults.viewFileUrl,
  onlyOfficeUrl: process.env.VUE_APP_ONLY_OFFICE_URL || stageDefaults.onlyOfficeUrl,
  wsUrl: process.env.VUE_APP_WS_URL || ''
};

const runtimeConfig = typeof window !== 'undefined' ? window.__RSH_FLOW_CONFIG__ || {} : {};

export let baseUrl = runtimeConfig.baseUrl || defaultConfig.baseUrl;
export let viewFileUrl = runtimeConfig.viewFileUrl || defaultConfig.viewFileUrl;
export let onlyOfficeUrl = runtimeConfig.onlyOfficeUrl || defaultConfig.onlyOfficeUrl;
export let wsUrl = runtimeConfig.wsUrl || defaultConfig.wsUrl;

export function setConfig(config) {
  if (config.baseUrl) baseUrl = config.baseUrl;
  if (config.viewFileUrl) viewFileUrl = config.viewFileUrl;
  if (config.onlyOfficeUrl) onlyOfficeUrl = config.onlyOfficeUrl;
  if (config.wsUrl) wsUrl = config.wsUrl;
}
