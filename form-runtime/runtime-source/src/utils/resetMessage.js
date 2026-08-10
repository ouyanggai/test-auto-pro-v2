/**
 * 重写message，解决返回数据，弹框出现叠加的问题
 */

import { Message } from 'element-ui';
let messageInstance = null;
const resetMessage = (options) => {
  if (messageInstance) {
    messageInstance.close();
  }
  messageInstance = Message(options);
};
['error', 'success', 'info', 'warning'].forEach(type => {
  resetMessage[type] = options => {
    if (typeof options === 'string') {
      options = {
        message: options
      };
    }
    options.type = type;
    return resetMessage(options);
  };
});
const message = resetMessage;
export default message;
