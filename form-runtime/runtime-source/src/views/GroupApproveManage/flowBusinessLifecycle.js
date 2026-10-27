import Api from '@/api';
import { isCaseFlowType } from './caseFlowBizRelevance';

// 各业务按流程类型注册实例绑定规则，公共流程组件不直接依赖具体业务 API。
const flowInstanceBinders = [
  {
    supports: isCaseFlowType,
    endpoint: Api.caseManage.bindFlowInstance,
    resolveBusinessId(flowType, relevanceList) {
      const relevance = (relevanceList || []).find(item =>
        item && item.otherBiz === flowType && item.otherBizId
      );
      return relevance && relevance.otherBizId;
    }
  }
];

function resolveFlowInstanceId(flowInstanceData) {
  if (flowInstanceData && typeof flowInstanceData === 'object') {
    return flowInstanceData.id || flowInstanceData.flowInstanceId;
  }
  return flowInstanceData;
}

/**
 * 流程创建成功后的通用业务实例绑定入口。
 * 新业务只需在 flowInstanceBinders 中注册，不需要继续修改 FlowDialog。
 */
export function bindBusinessFlowInstance({
  flowType,
  flowInstanceData,
  bizRelevanceList,
  request
}) {
  const binder = flowInstanceBinders.find(item => item.supports(flowType));
  if (!binder) {
    return Promise.resolve({ skipped: true });
  }

  const flowInstanceId = resolveFlowInstanceId(flowInstanceData);
  const businessId = binder.resolveBusinessId(flowType, bizRelevanceList);
  if (!flowInstanceId || !businessId) {
    return Promise.resolve({
      success: false,
      message: '流程已创建，但业务流程实例ID回写参数不完整'
    });
  }

  return request(binder.endpoint, {
    data: {
      id: businessId,
      flowInstanceId
    }
  }).then(response => ({
    success: Boolean(response && response.isSuccess),
    message: response && response.message
  }));
}

/**
 * 回退成功后同步案件状态。流程引擎回退接口是通用接口，只有案件流程需要补充业务状态。
 * 回退后的节点由流程引擎维护，本次只恢复案件为审批中，不根据节点名称推断状态。
 */
export function syncCaseFlowStatusAfterRollback({
  flowType,
  flowInstanceId,
  bizRelevanceList,
  request
}) {
  if (!isCaseFlowType(flowType)) {
    return Promise.resolve({ skipped: true });
  }

  const relevance = (bizRelevanceList || []).find(item =>
    item && item.otherBiz === flowType && item.otherBizId
  );
  if (!relevance || !relevance.otherBizId || !flowInstanceId) {
    return Promise.reject(new Error('案件流程回退后缺少案件业务ID或流程实例ID'));
  }

  return request(Api.caseManage.syncFlowStatus, {
    data: {
      id: relevance.otherBizId,
      flowInstanceId,
      caseStatus: 'IN_APPROVAL'
    }
  }).then(response => {
    if (!response || !response.isSuccess) {
      throw new Error((response && response.message) || '案件状态同步失败');
    }
    return { success: true };
  });
}
