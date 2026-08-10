/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-05-13 14:43:24
 */
import Vue from 'vue';
import store from '@/store';

function normalizePermissionPath(value) {
  return String(value || '')
    .trim()
    .replace(/\/+/g, '/')
    .replace(/^\/+|\/+$/g, '');
}

function hasPermissionMatch(permissionList = [], value) {
  const normalizedValue = normalizePermissionPath(value);
  if (!normalizedValue) return false;
  return permissionList.some(item => {
    return normalizePermissionPath(item && item.href) === normalizedValue;
  });
}

Vue.directive('permission', {
  // 当被绑定的元素插入到 DOM 中时……
  inserted: function (el, binding) {
    const btnPermissionList = store.getters.btnPermissionList;
    const value = binding.value;
    const flag = hasPermissionMatch(btnPermissionList, value);
    if (!flag) {
      if (!el.parentNode) {
        el.style.display = 'none';
      } else {
        el.parentNode.removeChild(el);
      }
    }
  }
});

Vue.directive('elTabPermission', { // 饿了么的el-tabs直接使用v-permission只能获取到下面的el-tab-pane部分，所以需要单独另外获取按钮的节点
  // 当被绑定的元素插入到 DOM 中时……
  inserted: function (el, binding) {
    const btnPermissionList = store.getters.btnPermissionList;
    const value = binding.value;
    const flag = hasPermissionMatch(btnPermissionList, value);
    if (!flag) {
      const elId = el.getAttribute('aria-labelledby');
      Vue.nextTick(() => {
        const elDiv = document.getElementById(elId);
        elDiv.style.display = 'none';
        el.parentNode.removeChild(el);
      });
    }
  }
});

//获取焦点全选指令
Vue.directive('focusSelect', {
  inserted: function (el) {
    let input = el.querySelector('input')
    input.onfocus = (e)=>{
      e.target.select()
    }
  }
});

Vue.directive('focus', {
  inserted: function (el) {
    el.querySelector('input').focus();
  }
});
