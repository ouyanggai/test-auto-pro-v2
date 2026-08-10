const FLOW_DETAIL_FIELDS = [
  'id',
  'flowInstanceId',
  'flowProxyId',
  'flowNodeProxyId',
  'currentNodeProxyId',
  'formProxyId',
  'formExist',
  'jobTaskId',
  'batchNo',
  'flowNextNodeAuditType',
  'nextNodeProxyId',
  'nextNodeName',
  'auditWay',
  'flowName',
  'formName',
  'currentPendingNodeName',
  'lastCountersignFlag',
  'initiatorId',
  'createrId'
];

function hasValue(value) {
  return value !== undefined && value !== null && value !== '';
}

function findFallbackBizItem(sourceItem, fallbackList = [], index = 0) {
  if (!Array.isArray(fallbackList) || !fallbackList.length) {
    return {};
  }

  if (sourceItem) {
    const matchedItem = fallbackList.find((item) => item.otherBiz === sourceItem.otherBiz && item.otherBizId === sourceItem.otherBizId);
    if (matchedItem) {
      return matchedItem;
    }
  }

  return fallbackList[index] || {};
}

function mergeFlowInstanceBizRelevanceList(preferredRow, fallbackRow) {
  const preferredList = Array.isArray(preferredRow?.flowInstanceBizRelevanceList) ? preferredRow.flowInstanceBizRelevanceList : [];
  const fallbackList = Array.isArray(fallbackRow?.flowInstanceBizRelevanceList) ? fallbackRow.flowInstanceBizRelevanceList : [];
  const baseList = preferredList.length ? preferredList : fallbackList;
  const fallbackFlowInstanceId =
    preferredRow?.flowInstanceId ||
    fallbackRow?.flowInstanceId ||
    preferredList[0]?.flowInstanceId ||
    fallbackList[0]?.flowInstanceId ||
    preferredRow?.id ||
    fallbackRow?.id ||
    '';

  return baseList.map((item, index) => {
    const fallbackItem = findFallbackBizItem(item, fallbackList, index);
    return {
      ...fallbackItem,
      ...item,
      flowInstanceId: hasValue(item?.flowInstanceId) ? item.flowInstanceId : (fallbackItem.flowInstanceId || fallbackFlowInstanceId),
    };
  });
}

function mergeFlowDetailRow(preferredRow, fallbackRow) {
  if (!preferredRow && !fallbackRow) {
    return null;
  }

  if (!preferredRow) {
    return fallbackRow;
  }

  if (!fallbackRow) {
    const row = { ...preferredRow };
    row.flowInstanceBizRelevanceList = mergeFlowInstanceBizRelevanceList(preferredRow, null);
    if (!hasValue(row.flowInstanceId)) {
      row.flowInstanceId = row.id || '';
    }
    return row;
  }

  const mergedRow = {
    ...fallbackRow,
    ...preferredRow,
  };

  FLOW_DETAIL_FIELDS.forEach((field) => {
    if (!hasValue(mergedRow[field]) && hasValue(fallbackRow[field])) {
      mergedRow[field] = fallbackRow[field];
    }
    if (!hasValue(mergedRow[field]) && hasValue(preferredRow[field])) {
      mergedRow[field] = preferredRow[field];
    }
  });

  mergedRow.flowInstanceBizRelevanceList = mergeFlowInstanceBizRelevanceList(preferredRow, fallbackRow);
  if (!hasValue(mergedRow.flowInstanceId)) {
    mergedRow.flowInstanceId = mergedRow.id || '';
  }
  if (!hasValue(mergedRow.flowProxyId) && hasValue(mergedRow.flowId)) {
    mergedRow.flowProxyId = mergedRow.flowId;
  }

  return mergedRow;
}

module.exports = {
  mergeFlowDetailRow,
};
