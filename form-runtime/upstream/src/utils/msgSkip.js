/*
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-05-11 10:54:54
 */
import router from '@/router/staticRouter';
import { localstorageRemove } from '@/utils/auth.js';
// 统一处理消息跳转路由
function getMsgPath(msgType, relevanceId, tab) {
  // console.log('=========getMsgPath==========')
  const MSGTYPE = {
    WORKTASK: {
      path: '/groupTaskManage/myTask/detail', query: { id: relevanceId, currentPage: 1, fromMsg: true }
    },
    WORKPLAN: {
      path: '/groupTaskManage/myWork', query: { id: relevanceId, currentPage: 1, fromMsg: true, isPass: false }
    },
    WORKFLOW: {
      path: '/groupApproveManage', query: { id: relevanceId, tab, fromMsg: true }
    }
    // /groupTaskManage/taskArrange //需要审核的
    // taskDetailDialogVisible = true
    // taskDetailDialogVisible = true
    // detailId = true
    // arrangeType='audit'

  };
  const type = MSGTYPE[msgType] || '';
  return type;
}
const msgSkip = (message) => {
  console.log('==========msgSkip========', message);
  // return
  var type = message.businessType;
  var relevanceId = message.relevanceId;
  var tab = message.title.indexOf('发起') > -1 ? 'submitted' : 'finished';
  var projectId = message?.projectId || '';

  // console.log('projectId',projectId)
  // console.log('d',message)
  // return
  var page = getMsgPath(type, relevanceId, tab, projectId);
  console.log('page', page);

  // 如果是审核通过的计划，修改path
  if (type == 'WORKPLAN') {
    if (message.title.indexOf('通过') > -1) {
      page.query.isPass = true;
    }
  } else if (message.title.indexOf('你') > -1 && message.title.indexOf('审核') > -1 && type == 'WORKTASK') { // 任务需要审核
    page.path = '/groupTaskManage/taskArrange/audit';
    // page.path = '/groupTaskManage/taskArrange/index'
    page.query.type = 'audit';
  }

  // return;
  // 没有特殊判断的统一走这里
  if (page) {
    // console.log('=========不需要审核任务1==========',page)
    var pageInfo = router.resolve(page);
    // console.log('=========不需要审核任务2==========',pageInfo)
    if (pageInfo) {
      window.open(pageInfo.href);
    }
  }
};
export default msgSkip;
