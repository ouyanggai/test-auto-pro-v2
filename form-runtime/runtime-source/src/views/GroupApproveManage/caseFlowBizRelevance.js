const CASE_FLOW_TYPES = [
  'lawsuit_legal',
  'lawsuit_biz',
  'non_lawsuit',
  'lawsuit_plan'
];

export function isCaseFlowType(flowType) {
  return CASE_FLOW_TYPES.includes(flowType);
}

// 后端按 company 关联筛选流程；案件流程要求保存一个公司 ID 为空的关联。
// 审核和重新提交会全量覆盖旧关联，因此每次提交前都统一替换，避免审核后查询不到案件流程。
export function normalizeCaseCompanyRelevance(flowType, relevanceList) {
  const list = Array.isArray(relevanceList)
    ? relevanceList.filter(Boolean).map(item => ({ ...item }))
    : [];

  if (!isCaseFlowType(flowType)) {
    return list;
  }

  const normalized = [
    { otherBiz: 'company', otherBizId: '' },
    ...list.filter(item => item.otherBiz !== 'company')
  ];
  const seen = new Set();
  return normalized.filter(item => {
    const key = `${item.otherBiz || ''}\u0000${item.otherBizId || ''}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}
