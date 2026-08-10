import fmCustomPage from './fmCustomPage.vue';
export function install (Vue) {
  if (install.installed) {
    return;
  }
  install.installed = true;
  Vue.component(fmCustomPage.name, fmCustomPage);
}

fmCustomPage.install = install;

if (typeof window !== 'undefined' && window.Vue) {
  window.Vue.use(fmCustomPage);
}

export default fmCustomPage;
