/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-05-07 09:26:25
 */
/**
 * Created by PanJiaChen on 16/11/18.
 */

/**
 * @param {string} path
 * @returns {Boolean}
 */
export function isExternal(path) {
  return /^(https?:|mailto:|tel:)/.test(path);
}

/**
 * @param {string} str
 * @returns {Boolean}
 */
export function validUsername(str) {
  const valid_map = ['admin', 'editor'];
  return valid_map.indexOf(str.trim()) >= 0;
}

// 手机号码
export function isValidTel(str) {
  const reg = /^(1\d{10})?$/;
  return reg.test(str);
}
