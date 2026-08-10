/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-07-06 12:44:33
 */
import axios from 'axios';

import { baseUrl } from '@/config/env';
// 创建实例
const service = axios.create({
  baseURL: baseUrl,
  timeout: 30000, // 请求超时时间
  withCredentials: false
});
service.interceptors.request.use(config => {
  return config;
});
// respone拦截器
service.interceptors.response.use(
  response => {
    const data = response.data;
    return data;
  },
  error => {
    return Promise.reject(error);
  }
);

export default service;
