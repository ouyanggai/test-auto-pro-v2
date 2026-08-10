import Vue from 'vue';
import Vuex from 'vuex';

Vue.use(Vuex);

// 自动导入 modules 目录下的所有模块
const modulesFiles = require.context('./modules', true, /\.js$/);
const modules = modulesFiles.keys().reduce((modules, modulePath) => {
  const moduleName = modulePath.replace(/^\.\/(.*)\.\w+$/, '$1');
  const value = modulesFiles(modulePath);
  modules[moduleName] = value.default;
  return modules;
}, {});

import getters from './getters';

const store = new Vuex.Store({
  modules,
  getters
});

export default store;
