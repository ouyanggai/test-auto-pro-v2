/*
 * @Descripttion:封装axios请求
 * @Author: Calvin
 * @Date: 2021-04-19 19:08:32
 */
import axios from 'axios';
import store from '@/store';
import router from '@/router/staticRouter';

import {
  Message,
  Loading
} from 'element-ui';
import { baseUrl } from '@/config/env';
import { localstorageGet } from '@/utils/auth';
// import Message from './resetMessage.js';
let loading = null; // 定义loading变量
// 内存中正在请求的数量
let loadingNum = 0;
let isLoading = true; // 指定异步时是否需要全屏loading

function startLoading () {
  if (loadingNum == 0) {
    loading = Loading.service({
      lock: true,
      // text: '拼命加载中...',
      background: 'rgba(255,255,255,0.1)'
    });
  }
  // 请求数量加1
  loadingNum++;
}

function endLoading () {
  // 请求数量减1
  loadingNum--;
  if (loadingNum <= 0) {
    loadingNum = 0
    loading.close();
  }
}

// 删除指定cookie
function deleteMyCookie() {
  let cookieList = ['token','customerCode','userId','userDepartmentId','userName','companyId','companyName','userDepartmentName','topCompanyId','user']
  let suffName = 'guanwang';
  cookieList.forEach(x=>{
    deleteCookie(`${suffName}-${x}`, '/', '.runshihua.com');
  })
}
// 清除cookieName--用于官网和全过程平台一键登录功能
function deleteCookie(name, path, domain) {
  console.log('deleteCookie')
  if (getCookie(name)) {
    document.cookie = name + "=" +
    (path ? ";path=" + path : "") +
    (domain ? ";domain=" + domain : "") +
    ";expires=Thu, 01 Jan 1970 00:00:01 GMT";
  }
}
function getCookie(name) {
  var nameEQ = name + "=";
  var ca = document.cookie.split(';');
  for(var i=0;i < ca.length;i++) {
    var c = ca[i];
    while (c.charAt(0)==' ') c = c.substring(1,c.length);
    if (c.indexOf(nameEQ) == 0) return c.substring(nameEQ.length,c.length);
  }
  return null;
}

const getHeaders = {
  'Content-Type': 'application/x-www-form-urlencoded'
};
const postHeaders = {
  'Content-Type': 'application/json'
};
const fileHeader = {
  'Content-Type': 'multipart/form-data'
};

const http = axios.create({
  baseURL: baseUrl,
  withCredentials: false,
  headers: getHeaders
});
http.interceptors.request.use((config) => {
  loadingNum = loadingNum < 0 ? 0 : loadingNum
  if (config && config.data && config.data.hasOwnProperty('isLoading') && !config.data.isLoading) {
    // 异步传参中存在isLoading属性且为false时不开启全屏loading
    isLoading = config.data.isLoading;
    delete config.data.isLoading;
    loading.close();
    loadingNum++;
  } else {
    // 加载loading
    if(config.noLoading === undefined || config.noLoading === false){
      isLoading = true;
      startLoading();
    }
  }
  return config;
});
// response 拦截器
http.interceptors.response.use(
  (res) => {
    // 关闭loading
    if (isLoading) {
      endLoading();
    } else {
      loadingNum--;
    }
    return res; // return res.data;
  },
  (error) => {
    // 关闭loading
    if (isLoading) {
      endLoading();
    } else {
      loadingNum--;
    }
    // console.log(error, 'errr');
    
    // 在拦截器中也检查网络错误并显示提示
    // if (isNetworkError(error)) {
    //   Message({
    //     message: '网络连接失败，请检查您的网络设置！',
    //     type: 'error',
    //     duration: 5 * 1000
    //   });
    // }
    
    return Promise.reject(error);
  }
);

function apiAxios (method, url, params, response, file, config) {
  let sid = localstorageGet('token') || '';
  const customerCode = localstorageGet('customerCode') || null; // 没有客户码就传null
  // console.log('axios-params',params)
  if (params.hasOwnProperty('sid')) {
    sid = params.sid;
  }
  // console.log('sid',sid)
  if (sid && !params.hasOwnProperty('isBid')) {
    url = `${url}?sid=${sid}&platformCode=${params.platformCode||200001}`;
    // url = `${url}?sid=${sid}&platformCode=200001`;
    if(config?.noLoading)url += '&unStatistics=1'
    params.sid = sid;
    params.projectId = store.state.user.projectId || '';
    if (url.includes('/web/user/api/resources/getRoleResourceTree') && store.state.user.groupDepartment == 'group' && params.data.isReload) {
      params.projectId = '';
    }
    if (method === 'POST') {
      // 给所有的post方法data中加上客户码
      params.data = params.data ? params.data : {};
      if (!params.data.customerCode) {
        params.data.customerCode = customerCode;
      }
    }
  } else {
    delete params.isBid;
    delete params.sid;
  }

  const dd = getHeaders;
  const po = postHeaders;
  const fi = file ? fileHeader : {};
  // fi['rsh-href'] = po['rsh-href'] = dd['rsh-href'] = router.history.current.path;
  dd.sid = sid;
  po.sid = sid;
  fi.sid = sid;

  return new Promise((resolve, reject) => {
    return http({
      method: method,
      url: url,
      data: method === 'POST' || method === 'PUT' ? params : null,
      params: method === 'GET' || method === 'DELETE' ? params : null,
      headers: file ? fi : method === 'GET' ? dd : po,
      ...config
    })
      .then(function (originResponse) {
        var res = originResponse.data;
        if (
          (res.code && res.code === 'AUTH_401') ||
          (res.code && res.code === 'RESP401')
        ) {
          // return;
          Message({
            message: '登录过期，请重新登录',
            type: 'error',
            duration: 5 * 1000
          });
          // 接口报错删除cookie
          deleteMyCookie();

          store.commit('user/RESET_STATE');
          store.commit('permission/RESET_PERMISSION');
          store.commit('app/SAVE_TITLE', '润世华新能源投资建设管理平台');
          store.commit(
            'app/SAVE_DEFAULT_TITLE',
            '润世华新能源投资建设管理平台'
          );
          if (router.history.current.path.indexOf('commonApprove') > -1) {
            // 如果是统一审核页面，跳回去？？
          } else {
            console.log('401重新跳转login')
            router.replace({
              path: '/login'
            });
          }
        } else {
          if (res instanceof Blob) { // 返回文件流直接调用callback返回
            response(res, originResponse);
            return;
          };
          typeof response == 'function' && response(res, originResponse);
          // console.log('res.message',res.message)
          // console.log('res.data.errorType',res.data?.errorType)
          if (res.data && res.data.errorType) { // 流程审批弹窗选人不进行报错提示
            if (res.data.errorType == 'run_node_choose' || res.data.errorType == 'parallel_choose' || res.data.errorType == 'custom_choose') {
              reject(res, originResponse)
            }
          } else {
            if (!res.isSuccess) {
              if (!Array.isArray(res.data)) { // 数组错误提示就自己处理，其他统一提示
                /[$]message[.]error/.test(String(response)) || Message.error(res.message);
              }
            }
            res.isSuccess ? resolve(res, originResponse) : reject(res, originResponse);
          }
        }
      })
      .catch(function (err) {
        // console.log('err', err);
        if (isNetworkError(err)) {
          // Message({
          //   message: '网络连接失败，请检查您的网络设置！',
          //   type: 'error',
          //   duration: 5 * 1000
          // });
          typeof response == 'function' && response(err);
          reject(err);
        } else if (
          err.response && (err.response.status === 401 ||
          err.response.data.code == 'RESP401')
        ) {
          if (store.state.user.token) {
            Message({
              message: '登录过期，请重新登录',
              type: 'error',
              duration: 5 * 1000
            });
            // 接口报错删除cookie
            deleteMyCookie();

            store.commit('user/RESET_STATE');
            store.commit('permission/RESET_PERMISSION');
            store.commit('app/SAVE_TITLE', '润世华新能源投资建设管理平台');
            store.commit(
              'app/SAVE_DEFAULT_TITLE',
              '润世华新能源投资建设管理平台'
            );
            console.log('catch-401重新跳转login')
            router.replace({
              path: '/login'
            });
          }
        } else {
          const errMsg = err.toString();
          if (errMsg.indexOf('504') > -1) {
            // reject('网络超时，请稍后再试');
            Message({
              message: '网络超时，请稍后再试',
              type: 'error',
              duration: 5 * 1000
            });
          } else {
            typeof response == 'function' && response(err);
            /[$]message[.]error/.test(String(response)) || Message.error(err.message);
            reject(err);
          }
        }
      });
  });
}

function isNetworkError (err) {
  // Check for various network error indicators
  return (
    err.message === 'Network Error' ||
    err.code === 'ERR_NETWORK' ||
    (err.response && err.response.status === 0) ||
    (!err.response && err.request) || // Request made but no response received (likely network error)
    (err.message && err.message.includes('Network Error')) ||
    (err.toString && err.toString().includes('Network Error'))
  );
}

export default {
  get: function (url, params, response) {
    return apiAxios('GET', url, params, response);
  },
  post: function (url, params, response, file, config) {
    return apiAxios('POST', url, params, response, file, config);
  },
  put: function (url, params, response) {
    return apiAxios('PUT', url, params, response);
  },
  delete: function (url, params, response) {
    return apiAxios('DELETE', url, params, response);
  }
};
