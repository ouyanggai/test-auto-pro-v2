/*
 * @Descripttion:封装webSocket
 */
import { Message } from 'element-ui';
// import {
//   localstorageGet
// } from '@/utils/auth';
let websock = null;
let messageCallback = null;
let errorCallback = null;
let lockReconnect = false;
let userClose = false;
let wsUrl = '';
// 接收ws后端返回的数据
function websocketonmessage(e) {
  messageCallback(JSON.parse(e.data));
}

/**
 * 发起websocket连接
 * @param {Object} agentData 需要向后台传递的参数数据
 */
function websocketSend(agentData) {
  // 加延迟是为了尽量让ws连接状态变为OPEN
  setTimeout(() => {
    // 添加状态判断，当为OPEN时，发送消息
    if (websock.readyState === websock.OPEN) { // websock.OPEN = 1
      // 发给后端的数据需要字符串化
      websock.send(JSON.stringify(agentData));
    }
    if (websock.readyState === websock.CLOSED) { // websock.CLOSED = 3
      console.log('websock.readyState=3');
      // Message.error('ws连接异常，请稍候重试');
      errorCallback();
    }
  }, 500);
}
// 建立ws连接
function websocketOpen(e) {
  console.log('ws连接成功');
}

// 初始化weosocket
function initWebSocket() {
  if (typeof (WebSocket) === 'undefined') {
    Message.error('您的浏览器不支持WebSocket，无法获取数据');
    return false;
  }
  // const sid = localstorageGet('token') || '';
  // const userId = localstorageGet('userId') || '';

  // ws请求完整地址
  const requstWsUrl = wsUrl;
  // const requstWsUrl = wsUrl + '/' + userId;
  // websock = new WebSocket(requstWsUrl);
  try {
    websock = new WebSocket(requstWsUrl);
    initEventHandle();
  } catch (e) {
    reconnect();
  }
}
// 初始化
function initEventHandle() {
  // 连接成功建立后响应
  websock.onopen = function () {
    heartCheck.reset().start();
    websocketOpen();
  };
  // 连接关闭后响应
  websock.onclose = function (e) {
    heartCheck.reset();
    if (!userClose) {
      reconnect(); // 重连
    }
  };
  websock.onerror = function () {
    heartCheck.reset();
    if (!userClose) {
      reconnect(); // 重连
    }
  };
  websock.onmessage = function (e) {
    heartCheck.reset().start();
    websocketonmessage(e);
  };
}
var heartCheck = {
  // 55秒发起一次心跳，比Server端设置的连接时间稍微小一点，在接近断开的情况下以通信的方式去重置连接时间
  timeout: 55000,
  serverTimeoutObj: null,
  reset: function () {
    clearTimeout(this.serverTimeoutObj);
    return this;
  },
  start: function () {
    this.serverTimeoutObj = setInterval(function () {
      if (websock.readyState == 1) {
        websock.send('webSocketPing');
        // 如果获取到消息，说明连接正常，重置心跳检测
        heartCheck.reset().start();
      } else {
        reconnect();
      }
    }, this.timeout);
  }
};
function reconnect() {
  if (lockReconnect) return;
  lockReconnect = true; // 没连接上会一直重连，设置延迟避免请求过多
  setTimeout(() => {
    initWebSocket();
    lockReconnect = false;
  }, 3000);
}

/**
 * 发起websocket请求函数
 * @param {string} url ws连接地址
 * @param {Object} agentData 传给后台的参数
 * @param {function} successCallback 接收到ws数据，对数据进行处理的回调函数
 * @param {function} errCallback ws连接错误的回调函数
 */
export function sendWebsocket(url, agentData, successCallback, errCallback) {
  setTimeout(() => {
    userClose = false;
  }, 100);
  wsUrl = url;
  initWebSocket();
  messageCallback = successCallback;
  errorCallback = errCallback;
  // websocketSend(agentData);
  return websocketSend; // 返回发送消息的方法，调用传参即可
}

/**
 * 关闭websocket函数
 */
export function closeWebsocket() {
  if (websock) {
    userClose = true;
    websock.close(); // 关闭websocket
    websock.onclose(); // 关闭websocket
  }
}
