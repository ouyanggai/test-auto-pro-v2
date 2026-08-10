const STORAGE_PREFIX = 'invest-power-system';
const DEFAULT_PLATFORM_CODE = '200001';

function getScopedStorageValue(name) {
  if (typeof window === 'undefined' || !window.localStorage) return '';
  return window.localStorage.getItem(`${STORAGE_PREFIX}-${name}`) || '';
}

function getLocationParam(name) {
  if (typeof window === 'undefined') return '';

  try {
    const searchParams = new URLSearchParams(window.location.search || '');
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

function resolveRuntimePlatformCode(explicitPlatformCode) {
  return explicitPlatformCode
    || getLocationParam('platformCode')
    || getScopedStorageValue('platformCode')
    || (typeof window !== 'undefined' && window.localStorage
      ? window.localStorage.getItem('platformCode') || ''
      : '')
    || DEFAULT_PLATFORM_CODE;
}

module.exports = {
  getLocationParam,
  resolveRuntimePlatformCode,
};
