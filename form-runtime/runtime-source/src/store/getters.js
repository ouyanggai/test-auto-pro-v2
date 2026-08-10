/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-01-14 14:22:24
 */
const getters = {
  sidebar: state => state.app.sidebar,
  fromRouteType: state => state.app.fromRouteType,
  toRouteType: state => state.app.toRouteType,
  device: state => state.app.device,
  token: state => state.user.token,
  sidebarMenu: state => state.permission.sidebarMenu,
  permissionList: state => state.permission.permissionList,
  btnPermissionList: state => state.permission.btnPermissionList,
  groupDepartment: state => state.user.groupDepartment,
  keepAlivePage: state => state.app.keepAlivePage
};
export default getters;
