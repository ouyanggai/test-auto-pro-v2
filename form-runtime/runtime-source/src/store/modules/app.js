/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-01-14 14:22:24
 */
import {
  localstorageSet,
  localstorageGet
} from '@/utils/auth';
const state = {
  sidebar: {
    opened: localstorageGet('sidebarStatus') ? !!+localstorageGet('sidebarStatus') : true,
    withoutAnimation: false
  },
  title: localstorageGet('srhTitle') || '润世华新能源投资建设管理平台',
  defaultTitle: localstorageGet('srhDefaultTitle') || '润世华新能源投资建设管理平台',
  platformIcon: localstorageGet('platformIcon') || '',
  platformLogo: localstorageGet('platformLogo') || '',
  device: 'desktop',
  fromRouteType: '',
  toRouteType: '',
  resourceId: '', // 资源id
  isDepartmentFramework: '',
  keepAlivePage: [], // 需要路由缓存的页面
  isBigBoard: false
};

const mutations = {
  ADD_KEEP_ALIVE: (state, name) => {
    if (!state.keepAlivePage.includes(name)) {
      state.keepAlivePage = state.keepAlivePage.concat(name);
    }
  },
  TOGGLE_SIDEBAR: state => {
    state.sidebar.opened = !state.sidebar.opened;
    state.sidebar.withoutAnimation = false;
    if (state.sidebar.opened) {
      localstorageSet('sidebarStatus', 1);
    } else {
      localstorageSet('sidebarStatus', 0);
    }
  },
  SET_BIGBOARD: (state, data) => {
    state.isBigBoard = data == undefined ? true : data;
  },
  CLOSE_SIDEBAR: (state, withoutAnimation) => {
    localstorageSet('sidebarStatus', 0);
    state.sidebar.opened = false;
    state.sidebar.withoutAnimation = withoutAnimation;
  },
  TOGGLE_DEVICE: (state, device) => {
    state.device = device;
  },
  SAVE_platformIcon: (state, icon) => {
    state.platformIcon = icon;
    localstorageSet('platformIcon', icon);
  },
  SAVE_platformLogo: (state, logo) => {
    state.platformLogo = logo;
    localstorageSet('platformLogo', logo);
  },
  SAVE_TITLE: (state, title) => {
    state.title = title;
    localstorageSet('srhTitle', title);
  },
  SAVE_DEFAULT_TITLE: (state, title) => {
    state.defaultTitle = title;
    localstorageSet('srhDefaultTitle', title);
  },
  SAVE_ROUTE_TYPE: (state, data) => {
    localstorageSet('fromRouteType', data.fromRouteType);
    localstorageSet('toRouteType', data.toRouteType);
    localstorageSet('isDepartmentFramework', data.isDepartmentFramework);
    state.fromRouteType = data.fromRouteType;
    state.toRouteType = data.toRouteType;
    state.isDepartmentFramework = data.isDepartmentFramework;
    state.resourceId = data.resourceId;
  }
};

const actions = {
  addKeepAlivePage({
    commit
  }, name) {
    commit('ADD_KEEP_ALIVE', name);
  },
  toggleSideBar({
    commit
  }) {
    commit('TOGGLE_SIDEBAR');
  },
  setBigBoard({
    commit
  }, data) {
    commit('SET_BIGBOARD', data);
  },
  closeSideBar({
    commit
  }, {
    withoutAnimation
  }) {
    commit('CLOSE_SIDEBAR', withoutAnimation);
  },
  toggleDevice({
    commit
  }, device) {
    commit('TOGGLE_DEVICE', device);
  }
};

export default {
  namespaced: true,
  state,
  mutations,
  actions
};
