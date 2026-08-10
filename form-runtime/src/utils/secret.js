/*
 * @Author: junshao
 * @Date: 2023-02-28 14:52:06
 * @LastEditors: Please set LastEditors
 * @LastEditTime: 2024-08-15 09:32:36
 * @Description: file content
 */
import Vue from 'vue';
import cryptoJs from 'crypto-js';
// 把AES加密vue原型里
const keyOne = 'fff65019-27a2-49';
const key = cryptoJs.enc.Utf8.parse(keyOne);
// 加密
function Encrypt (word) {
  let enc = '';
  if (typeof word === 'string') {
    enc = cryptoJs.AES.encrypt(word, key, {
      // iv: iv
      mode: cryptoJs.mode.ECB,
      padding: cryptoJs.pad.Pkcs7
    });
  } else if (typeof word === 'object') {
    const data = JSON.stringify(word);
    enc = cryptoJs.AES.encrypt(data, key, {
      mode: cryptoJs.mode.ECB,
      padding: cryptoJs.pad.Pkcs7
    });
  }
  const encResult = enc.toString();
  return encResult;
};

// 解密方法
function Decrypt(word) {
  const encryptedHexStr = cryptoJs.enc.Hex.parse(word);
  console.log(encryptedHexStr);
  const srcs = cryptoJs.enc.Base64.stringify(encryptedHexStr);
  console.log(srcs);
  const decrypt = cryptoJs.AES.decrypt(srcs, key, {
    mode: cryptoJs.mode.ECB,
    padding: cryptoJs.pad.Pkcs7
  });
  const decryptedStr = decrypt.toString(cryptoJs.enc.Utf8);
  console.log(decryptedStr);
  return decryptedStr.toString();
}
Vue.prototype.$Encrypt = Encrypt;
Vue.prototype.$Decrypt = Decrypt;

