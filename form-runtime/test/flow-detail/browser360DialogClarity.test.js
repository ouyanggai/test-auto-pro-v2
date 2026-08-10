#!/usr/bin/env node
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const projectRoot = path.resolve(__dirname, '../..');
const iframeEntryPath = path.join(projectRoot, 'src/views/FlowDetail.vue');
const examineDialogPath = path.join(projectRoot, 'src/views/GroupApproveManage/components/ExamineDialog.vue');
const enterpriseDialogPath = path.join(projectRoot, 'src/views/GroupApproveManage/components/EnterpriseExamineDialog.vue');

const iframeEntrySource = fs.readFileSync(iframeEntryPath, 'utf8');
const examineDialogSource = fs.readFileSync(examineDialogPath, 'utf8');
const enterpriseDialogSource = fs.readFileSync(enterpriseDialogPath, 'utf8');

assert.match(
  iframeEntrySource,
  /this\.applyBrowser360DialogClarityFix\(wrapper\);/,
  'dialog style sync should apply the 360 browser clarity compatibility pass'
);

assert.match(
  iframeEntrySource,
  /return \/360SE\|360EE\|QihooBrowser\|QHBrowser\|QIHU 360\|360 Aphone Browser\/i\.test\(ua\);/,
  '360 browser detection should be explicit and limited to known 360 user agents'
);

assert.match(
  iframeEntrySource,
  /if \(!this\.isBrowser360\(\) \|\| !this\.isMainFlowDialog\(wrapper\)\) \{\s*return;\s*\}/,
  '360 clarity fix should only touch main flow dialogs'
);

assert.match(
  iframeEntrySource,
  /if \(!dialog \|\| !this\.hasFractionalZoom\(dialog\.style\.zoom\)\) \{\s*return;\s*\}/,
  '360 clarity fix should only touch dialogs with fractional inline zoom'
);

assert.match(
  iframeEntrySource,
  /dialog\.style\.zoom = '1';\s*dialog\.classList\.add\('flow-detail-browser360-clear'\);/,
  '360 clarity fix should reset fractional zoom to 1 and mark the dialog'
);

assert.match(
  iframeEntrySource,
  /const normalizedZoom = String\(zoomValue\)\.includes\('%'\) \? zoomNumber \/ 100 : zoomNumber;\s*return normalizedZoom > 0 && normalizedZoom !== 1 && normalizedZoom % 1 !== 0;/,
  'fractional zoom detection should include decimal and percent zoom values'
);

assert.match(
  examineDialogSource,
  /:style="\{zoom: detectZoom\(\) > 140 \? 0\.88 : 1\}"/,
  'synced fixed-form dialog should keep the source zoom expression unchanged'
);

assert.match(
  enterpriseDialogSource,
  /:style="\{zoom: detectZoom\(\) > 140 \? 0\.88 : 1\}"/,
  'synced formMaking dialog should keep the source zoom expression unchanged'
);

function hasFractionalZoom(zoomValue) {
  if (!zoomValue) return false;
  const zoomNumber = Number(String(zoomValue).replace('%', ''));
  if (!Number.isFinite(zoomNumber)) return false;
  const normalizedZoom = String(zoomValue).includes('%') ? zoomNumber / 100 : zoomNumber;
  return normalizedZoom > 0 && normalizedZoom !== 1 && normalizedZoom % 1 !== 0;
}

assert.strictEqual(hasFractionalZoom('0.88'), true, 'decimal zoom should be treated as blurry risk');
assert.strictEqual(hasFractionalZoom('88%'), true, 'percentage fractional zoom should be treated as blurry risk');
assert.strictEqual(hasFractionalZoom('1'), false, 'zoom 1 should not be changed');
assert.strictEqual(hasFractionalZoom('100%'), false, 'zoom 100% should not be changed');
assert.strictEqual(hasFractionalZoom('2'), false, 'integer zoom should not be changed');
assert.strictEqual(hasFractionalZoom(''), false, 'empty zoom should not be changed');

console.log('360 browser dialog clarity test passed');
