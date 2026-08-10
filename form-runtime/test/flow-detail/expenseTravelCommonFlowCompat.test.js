#!/usr/bin/env node
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const projectRoot = path.resolve(__dirname, '../..');
const iframeEntryPath = path.join(projectRoot, 'src/views/FlowDetail.vue');
const syncedFlowDetailPath = path.join(projectRoot, 'src/layout/components/globalDialog/components/flowDetail.vue');

const iframeEntrySource = fs.readFileSync(iframeEntryPath, 'utf8');
const syncedFlowDetailSource = fs.readFileSync(syncedFlowDetailPath, 'utf8');

assert.match(
  iframeEntrySource,
  /return \['expense_budget', 'travel_expense'\]\.includes\(this\.getExpenseCommonFlowAuditWay\(row\)\);/,
  'compatibility should be limited to expense_budget and travel_expense'
);

assert.match(
  iframeEntrySource,
  /return mode === 'edit' && this\.isExpenseCommonFlowCompat\(taskContext\);/,
  'compatibility should only run for edit mode'
);

assert.match(
  iframeEntrySource,
  /if \(taskContext && !isExpenseCommonFlowEdit\) \{\s*data\.row = taskContext;\s*\}/,
  'expense/travel edit should not pass iframe row that overrides the full task row'
);

assert.match(
  iframeEntrySource,
  /if \(isExpenseCommonFlowEdit\) \{\s*this\.applyExpenseCommonFlowEditContext\(dialog\);\s*\}/,
  'expense/travel edit should install the common flow edit compatibility patch'
);

assert.match(
  iframeEntrySource,
  /root\.clickReInitiate = function\(row, type\) \{\s*const result = originalClickReInitiate\.call\(this, row, type\);\s*apply\(row\);\s*return result;\s*\};/,
  'compatibility patch should run after target clickReInitiate assigns its normal state'
);

assert.match(
  iframeEntrySource,
  /root\.isExamine = false;\s*root\.isReInitiate = true;/,
  'compatibility patch should enable common flow editing for whitelisted edit entries'
);

assert.match(
  iframeEntrySource,
  /root\.flowInstanceBizRelevanceList = row\.flowInstanceBizRelevanceList\.map\(item => \(\{ \.\.\.item \}\)\);/,
  'compatibility patch should preserve full relation list for associated flow echo'
);

assert.match(
  syncedFlowDetailSource,
  /let newData = this\.propData\.data\?\.row \|\| data;/,
  'synced target flowDetail should remain source-compatible; iframe compatibility lives in src/views/FlowDetail.vue'
);

function getExpenseCommonFlowAuditWay(row) {
  const relation = Array.isArray(row?.flowInstanceBizRelevanceList)
    ? row.flowInstanceBizRelevanceList.find(item => ['expense_budget', 'travel_expense'].includes(item.otherBiz))
    : null;
  return row?.auditWay || relation?.otherBiz || '';
}

function isExpenseCommonFlowCompat(row) {
  return ['expense_budget', 'travel_expense'].includes(getExpenseCommonFlowAuditWay(row));
}

function shouldPassRow(mode, taskContext) {
  const isExpenseCommonFlowEdit = mode === 'edit' && isExpenseCommonFlowCompat(taskContext);
  return Boolean(taskContext && !isExpenseCommonFlowEdit);
}

assert.strictEqual(
  shouldPassRow('edit', { auditWay: 'expense_budget', flowInstanceBizRelevanceList: [] }),
  false,
  'expense_budget edit should avoid passing incomplete iframe row'
);

assert.strictEqual(
  shouldPassRow('edit', { auditWay: 'travel_expense', flowInstanceBizRelevanceList: [] }),
  false,
  'travel_expense edit should avoid passing incomplete iframe row'
);

assert.strictEqual(
  shouldPassRow('edit', {
    flowInstanceBizRelevanceList: [
      { otherBiz: 'travel_expense', otherBizId: 'biz-1' },
      { otherBiz: 'commonFlow', otherBizId: 'flow-1' },
    ],
  }),
  false,
  'travel_expense edit should be detected from relation list when auditWay is missing'
);

assert.strictEqual(
  shouldPassRow('edit', { auditWay: 'request_funds', flowInstanceBizRelevanceList: [] }),
  true,
  'non-whitelisted edit should keep the existing row passthrough behavior'
);

assert.strictEqual(
  shouldPassRow('view', { auditWay: 'expense_budget', flowInstanceBizRelevanceList: [] }),
  true,
  'expense_budget view should keep the existing row passthrough behavior'
);

console.log('expense/travel common flow compatibility test passed');
