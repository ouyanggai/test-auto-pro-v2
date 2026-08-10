#!/usr/bin/env node
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const projectRoot = path.resolve(__dirname, '../..');
const flowDetailPath = path.join(projectRoot, 'src/layout/components/globalDialog/components/flowDetail.vue');
const iframeEntryPath = path.join(projectRoot, 'src/views/FlowDetail.vue');

const flowDetailSource = fs.readFileSync(flowDetailPath, 'utf8');
const iframeEntrySource = fs.readFileSync(iframeEntryPath, 'utf8');

const formMakingBranchStart = flowDetailSource.indexOf('if (!this.formExist) { // formMaking');
assert.notStrictEqual(formMakingBranchStart, -1, 'flowDetail formMaking branch should exist');

const formMakingBranch = flowDetailSource.slice(formMakingBranchStart, formMakingBranchStart + 1200);

assert.match(
  formMakingBranch,
  /this\.operaType = ["']check["'];\s*this\.actionType = ["']preview["'];/,
  'synced flowDetail should remain source-compatible; iframe audit compatibility lives in src/views/FlowDetail.vue'
);

assert.match(
  iframeEntrySource,
  /actionType: isExamine \? 'examine' : mode === 'edit' \? 'edit' : 'preview'/,
  'iframe entry should pass an explicit actionType by mode'
);

assert.match(
  iframeEntrySource,
  /applyIframeAuditContext\(dialog\)/,
  'iframe audit mode should apply the compatibility context'
);

assert.match(
  iframeEntrySource,
  /root\.operaType = 'edit';[\s\S]*root\.actionType = 'examine';[\s\S]*root\.isExamine = true;[\s\S]*root\.btnVisible = true;/,
  'iframe audit compatibility should map the root flow dialog to the same state as source pending review'
);

assert.match(
  iframeEntrySource,
  /Object\.prototype\.hasOwnProperty\.call\(data, 'actionType'\)[\s\S]*component\.actionType = 'examine';/,
  'iframe audit compatibility should also update mounted custom form components'
);

assert.match(
  iframeEntrySource,
  /dialog\.instance\.previewHandle = function\(row\)[\s\S]*originalPreviewHandle\.call\(this, row\);[\s\S]*apply\(\);/,
  'iframe audit compatibility should patch previewHandle before formMaking children mount'
);

assert.match(
  iframeEntrySource,
  /case 'audit':[\s\S]*isExamine = true;[\s\S]*taskStatus = 'pending';/,
  'iframe audit mode should still enter pending examine context'
);

console.log('flowDetail actionType mapping test passed');
