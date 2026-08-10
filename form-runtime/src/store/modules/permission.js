/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-05-12 12:44:02
 */

import Api from '@/api';
import axios from '@/utils/axios';
import {
  getOriginPermissionCache, localstorageGet, setOriginPermissionCache
} from '@/utils/auth';
import {
  dynamicRoutes
} from '@/router/dynamicRouter';
import router, {
  staticRoutes
} from '@/router/staticRouter';
import {
  recursionRouter,
  setDefaultRoute
} from '@/utils/recursionRouter';
import {
  resetResult,
  recursionBtn
} from '@/utils/recursionBtn';
import MobileDetect from 'mobile-detect';
import { deepClone } from '@/utils';
// 获取userAgent信息
var user_agent = navigator.userAgent;
// 初始化mobile-detect
var md = new MobileDetect(user_agent);
var os = md.os();// 获取系统

const state = {
  permissionList: [],
  btnPermissionList: [],
  sidebarMenu: [], /** 导航菜单 */
  isPhoneOs: !!(os == 'iOS' || os == 'AndroidOS')
};

const mutations = {
  RESET_PERMISSION: (state) => {
    state.permissionList = [];
    state.btnPermissionList = [];
  },
  SET_MENU(state, menu) {
    state.sidebarMenu = menu;
  },
  SET_PERMISSION_LIST: (state, routes) => {
    state.permissionList = routes;
  },
  SET_BTN_PERMISSION_LIST: (state, data) => {
    state.btnPermissionList = data;
  }
};
const actions = {
  // 获表

  getPermissionList({
    dispatch, commit, state
  }, payload) {
    console.log(payload, 'payload++++');
    return new Promise((resolve, reject) => {
      // 保存先前的权限
      // var originPermission = deepClone(state.permissionList)
      // 平台
      const groupDepartment = localstorageGet('groupDepartment') || 'group';
      let data = {};
      let httpUrl = '';
      // let projectMainDeptId = null;
      if (groupDepartment == 'group') {
        data = {
          userId: localstorageGet('userId'),
          departmentId: localstorageGet('userDepartmentId'),
          isReload: payload && typeof (payload) == 'object' ? payload.isReload : ''
        };
        httpUrl = Api.user.getGroupPermissionList;
      } else {
        // data = {
        //   // departmentId: localstorageGet('projectId'),
        //   userId: localstorageGet('userId')
        // };
        // projectMainDeptId = {
        //   projectMainDeptId: localstorageGet('projectDepartmentId')
        // };
        // httpUrl = Api.user.getPermissionList;
        // data = { companyId: localstorageGet('companyId'), projectId: localstorageGet('projectId')};
        // httpUrl = '/web/user/api/projectCompanyResource/findByProjectIdAndCompanyId';
        let departmentId = typeof(payload) == 'object' ? payload.departmentId :payload
        data = { departmentId: departmentId || localstorageGet('departmentId'), userId: localstorageGet('userId'), customerCode: localstorageGet('customerCode') };
        httpUrl = '/web/user/api/resources/getRoleResourceTree';
      }
      axios.post(
        // httpUrl, { data, ...projectMainDeptId },
        httpUrl, { data, platformCode: '200001' },
        res => {
          if (res.isSuccess) {
            // 清除菜单
            if (!res.data || !res.data.length) {
              reject(console.error());
            } else {
              commit('RESET_PERMISSION');
              const permissionList = res.data;
              resetResult();
              var btnPermissionList = recursionBtn(res.data);
              if (groupDepartment == 'department') {
                const originPermission = getOriginPermissionCache();
                if (originPermission) {
                  const originBtnPermissionList = recursionBtn(originPermission);
                  btnPermissionList = btnPermissionList.concat(originBtnPermissionList);
                }
              }
              commit('SET_BTN_PERMISSION_LIST', btnPermissionList);
              /* 根据权限刷选出我们设置好的路由并加入到 path='/' 的children */
              var routes = recursionRouter(permissionList, dynamicRoutes);
              // const routes = dynamicRoutes; // 本地权限菜单调试
              dispatch('createKeepAlivePage', { routes });
              const MainContainer = staticRoutes.find(v => v.path === '/');
              // 初始化children路由
              var children = [];
              // 将当前用户的权限路由添加到动态路由中
              children.push(...routes);
              // 如果是进入项目空间，公司空间的路由继续有效，只是隐藏起来，避免出现404的问题
              if (groupDepartment == 'department') {
                const originPermission = getOriginPermissionCache();
                if (originPermission) {
                  const copyDynamicRoutes = deepClone(dynamicRoutes);
                  const originRoutes = recursionRouter(originPermission, copyDynamicRoutes);
                  const copyOriginRoutes = deepClone(originRoutes);
                  copyOriginRoutes.forEach(item => {
                    item.hidden = true;
                  });
                  children = children.concat(copyOriginRoutes);
                }
              }
              /* 生成左侧导航菜单 */
              commit('SET_MENU', children);
              MainContainer.children = children;
              setDefaultRoute([MainContainer]);
              /* 动态路由 */
              router.addRoutes(staticRoutes);
              /* 完整的路由表 */
              commit('SET_PERMISSION_LIST', [...staticRoutes]);
              console.log('[...staticRoutes]',[...staticRoutes])
              // 如果是公司空间，保存原始权限
              if (localstorageGet('groupDepartment') === 'group') {
                setOriginPermissionCache(permissionList);
              }
              resolve();
            }
          } else {
            console.log('用户没有访问权限');
            // 用户没有权限
            reject(console.error());
          }
        }
      );
    });
  },
  getGroupPermissionList() {

  },
  async createKeepAlivePage({ commit }, { routes }) {
    // let keepAlivePage = []
    routes.forEach(item => {
      const children = item.children || [];
      children.forEach(el => {
        if (el.meta.keepAlive) {
          const name = el.name;
          commit('app/ADD_KEEP_ALIVE', name, { root: true });
        }
      });
    });
  }
};

export default {
  namespaced: true,
  state,
  mutations,
  actions
};
