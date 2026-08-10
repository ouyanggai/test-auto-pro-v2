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
  /this\.startDialogStyleObserver\(\);\s*this\.patchNestedFlowDetailDialogOpen\(\);/,
  'FlowDetail should patch nested flow detail opening before users click associated flows'
);

assert.match(
  iframeEntrySource,
  /const flowManager = this\.\$fm2 \|\| window\.\$vue\?\.\$fm2;/,
  'nested associated flow details should be detected through the $fm2 dialog manager'
);

assert.match(
  iframeEntrySource,
  /flowManager\.show = async function\(component, propData\) \{\s*const instance = await originalShow\.call\(this, component, propData\);\s*if \(component === 'flowDetail'\) \{\s*markNestedFlowDetail\(instance\);\s*\}\s*return instance;\s*\};/,
  'only $fm2.show("flowDetail") should be marked as nested associated flow detail'
);

assert.match(
  iframeEntrySource,
  /wrapper\.classList\.add\('flow-detail-keep-header'\);/,
  'nested associated flow detail wrappers should receive the keep-header marker'
);

assert.match(
  iframeEntrySource,
  /if \(wrapper\.classList\.contains\('flow-detail-keep-header'\)\) \{\s*return true;\s*\}/,
  'keep-header marker should preserve the Element UI close button even when it is the only main flow dialog wrapper'
);

assert.match(
  iframeEntrySource,
  /if \(this\.shouldKeepHeaderForNestedFlowDetail\(wrapper, mainFlowDialogWrappers\)\) \{\s*wrapper\.classList\.remove\('flow-detail-hide-header', 'flow-detail-countersign-dialog'\);\s*this\.removeCounterSignBackButton\(wrapper\);\s*return;\s*\}/,
  'nested flow detail dialogs should keep the default header and close button'
);

assert.match(
  iframeEntrySource,
  /getMainFlowDialogWrappers\(\) \{\s*return Array\.from\(document\.querySelectorAll\('\.el-dialog__wrapper'\)\)\s*\.filter\(wrapper => this\.isMainFlowDialog\(wrapper\) && !this\.isCounterSignDialog\(wrapper\)\);/,
  'main flow dialog ordering should remain as a fallback for nested flow detail detection'
);

assert.match(
  iframeEntrySource,
  /if \(!this\.isMainFlowDialog\(wrapper\) \|\| this\.isCounterSignDialog\(wrapper\)\) \{\s*return false;\s*\}\s*return mainFlowDialogWrappers\.indexOf\(wrapper\) > 0;/,
  'countersign dialogs should not be treated as nested flow detail by the ordering fallback'
);

assert.match(
  iframeEntrySource,
  /shouldHideDialogHeader\(wrapper\) \{\s*return this\.isMainFlowDialog\(wrapper\) \|\| this\.isCounterSignDialog\(wrapper\);/,
  'main flow and countersign dialogs should still be selected by the existing hide-header rule'
);

assert.match(
  iframeEntrySource,
  /if \(this\.isCounterSignDialog\(wrapper\)\) \{\s*wrapper\.classList\.add\('flow-detail-countersign-dialog'\);\s*this\.ensureCounterSignBackButton\(wrapper\);/,
  'countersign dialogs should still keep the dedicated back-button behavior'
);

assert.match(
  syncedFlowDetailSource,
  /let newData = this\.propData\.data\?\.row \|\| data;/,
  'synced target flowDetail should remain source-compatible; close-button compatibility lives in src/views/FlowDetail.vue'
);

function shouldKeepHeaderForNestedFlowDetail(wrapper, mainFlowDialogWrappers) {
  if (wrapper.keepHeader) {
    return true;
  }
  if (!wrapper.isMainFlowDialog || wrapper.isCounterSignDialog) {
    return false;
  }
  return mainFlowDialogWrappers.indexOf(wrapper) > 0;
}

const firstMain = { isMainFlowDialog: true, isCounterSignDialog: false };
const nestedMain = { isMainFlowDialog: true, isCounterSignDialog: false };
const markedNestedMain = { isMainFlowDialog: true, isCounterSignDialog: false, keepHeader: true };
const countersign = { isMainFlowDialog: false, isCounterSignDialog: true };
const mainFlowDialogWrappers = [firstMain, nestedMain];

assert.strictEqual(
  shouldKeepHeaderForNestedFlowDetail(firstMain, mainFlowDialogWrappers),
  false,
  'first main flow dialog should keep the old hidden close button behavior'
);

assert.strictEqual(
  shouldKeepHeaderForNestedFlowDetail(nestedMain, mainFlowDialogWrappers),
  true,
  'nested associated flow detail should keep the Element UI close button when ordering is available'
);

assert.strictEqual(
  shouldKeepHeaderForNestedFlowDetail(markedNestedMain, [markedNestedMain]),
  true,
  'marked associated flow detail should keep the close button even when it is the only main flow wrapper'
);

assert.strictEqual(
  shouldKeepHeaderForNestedFlowDetail(countersign, mainFlowDialogWrappers),
  false,
  'countersign dialog should not be treated as associated flow detail'
);

console.log('nested flow detail close button test passed');
