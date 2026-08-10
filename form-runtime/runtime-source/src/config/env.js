// 运行时可配置环境变量
// 可通过 window.__RSH_FLOW_CONFIG__ 在运行时覆盖

const defaultConfig = (() => {
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
    onlyOfficeUrl: 'https://ioffice.runshihua.com'
  };
})();

const runtimeConfig = window.__RSH_FLOW_CONFIG__ || {};

export let baseUrl = runtimeConfig.baseUrl || defaultConfig.baseUrl;
export let viewFileUrl = runtimeConfig.viewFileUrl || defaultConfig.viewFileUrl;
export let onlyOfficeUrl = runtimeConfig.onlyOfficeUrl || defaultConfig.onlyOfficeUrl;
export let wsUrl = '';

export function setConfig(config) {
  if (config.baseUrl) baseUrl = config.baseUrl;
  if (config.viewFileUrl) viewFileUrl = config.viewFileUrl;
  if (config.onlyOfficeUrl) onlyOfficeUrl = config.onlyOfficeUrl;
}
