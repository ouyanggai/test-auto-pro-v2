/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-05-13 16:28:34
 */
/**
 *
 * @param  {Array} userRouter 后台返回的用户权限json
 * @param  {Array} allRouter  前端配置好的所有动态路由的集合
 * @return {Array} realRoutes 过滤后的路由
 */

let result = [];
export function resetResult() {
  result = [];
}
export function recursionBtn(btnPermissionList = [], type) {
  if (!btnPermissionList.length) return [];
  const recursionBtnList = [];
  // if (type == 'group') {
    btnPermissionList.forEach(item => {
      if (item.resourcesVoList) {
        recursionBtnList.push(...item.resourcesVoList);
      }
      if (item.children && item.children.length > 0) {
        recursionBtn(item.children, type);
      }
    });
    return filterBtn(recursionBtnList);
  // } else {
  //   btnPermissionList.forEach(item => {
  //     if (item.children) {
  //       recursionBtnList.push(...item.children.filter(e => e.type == 'button'));
  //     }
  //     if (item.children && item.children.length > 0) {
  //       recursionBtn(item.children, type);
  //     }
  //   });
  //   return filterBtn(recursionBtnList);
  // }
}
function filterBtn(btnList = []) {
  btnList.forEach(item => {
    result.push(item);
    if (item.resourcesVoList && item.resourcesVoList.length) {
      filterBtn(item.resourcesVoList);
    }
  });
  return result;
}

