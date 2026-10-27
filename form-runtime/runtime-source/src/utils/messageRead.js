/**
 * 构造消息已读请求。
 *
 * 全部已读不携带当前页面的消息 ID，由后端按当前用户、租户和平台更新全部有效未读消息。
 *
 * @param {string} type 操作类型：single 或 all
 * @param {string} messageId 单条消息 ID
 * @returns {{data: Object}|null} 已读请求参数；参数无效时返回 null
 */
export function buildMessageReadParams(type, messageId) {
  if (type === 'single') {
    if (!messageId) return null;
    return {
      data: {
        id: messageId
      }
    };
  }

  if (type === 'all') {
    return {
      data: {}
    };
  }

  return null;
}
