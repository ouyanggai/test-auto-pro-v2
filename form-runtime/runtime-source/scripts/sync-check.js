#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { getSyncConfig } = require('./sync-config');

const { syncEnv, reason, sourceRoot, targetRoot } = getSyncConfig();

function sha1(filePath) {
  return crypto.createHash('sha1').update(fs.readFileSync(filePath)).digest('hex');
}

function listFiles(dirPath) {
  let files = [];
  const entries = fs.readdirSync(dirPath, { withFileTypes: true });
  for (const entry of entries) {
    const absPath = path.join(dirPath, entry.name);
    if (entry.isDirectory()) {
      files = files.concat(listFiles(absPath));
    } else {
      files.push(absPath);
    }
  }
  return files;
}

function checkPathSync(srcRel, destRel, type) {
  const src = path.join(sourceRoot, srcRel);
  const dest = path.join(targetRoot, destRel);
  const issues = [];

  if (!fs.existsSync(src)) {
    issues.push(`[MISSING_SOURCE] ${srcRel}`);
    return issues;
  }
  if (!fs.existsSync(dest)) {
    issues.push(`[MISSING_TARGET] ${destRel}`);
    return issues;
  }

  if (type === 'file') {
    if (sha1(src) !== sha1(dest)) {
      issues.push(`[DIFF_FILE] ${srcRel} -> ${destRel}`);
    }
    return issues;
  }

  const srcFiles = listFiles(src).map(f => path.relative(src, f));
  const destFiles = listFiles(dest).map(f => path.relative(dest, f));
  const srcSet = new Set(srcFiles);
  const destSet = new Set(destFiles);

  for (const rel of srcFiles) {
    if (!destSet.has(rel)) {
      issues.push(`[MISSING_TARGET_FILE] ${path.join(destRel, rel)}`);
      continue;
    }
    const srcFile = path.join(src, rel);
    const destFile = path.join(dest, rel);
    if (sha1(srcFile) !== sha1(destFile)) {
      issues.push(`[DIFF_DIR_FILE] ${path.join(srcRel, rel)} -> ${path.join(destRel, rel)}`);
    }
  }

  return issues;
}

function extractFormMakingComponentNames(filePath) {
  if (!fs.existsSync(filePath)) {
    return [];
  }

  const content = fs.readFileSync(filePath, 'utf8');
  const names = [];
  const nameRegExp = /name:\s*['"]([^'"]+)['"]\s*,\s*component:\s*[A-Za-z_$][\w$]*/g;
  let match;

  while ((match = nameRegExp.exec(content))) {
    names.push(match[1]);
  }

  return names;
}

function checkFormMakingRegistrations() {
  const srcMain = path.join(sourceRoot, 'src/main.js');
  const destMain = path.join(targetRoot, 'src/main.js');
  const issues = [];

  if (!fs.existsSync(srcMain)) {
    issues.push('[MISSING_SOURCE] src/main.js');
    return issues;
  }
  if (!fs.existsSync(destMain)) {
    issues.push('[MISSING_TARGET] src/main.js');
    return issues;
  }

  const srcNames = extractFormMakingComponentNames(srcMain);
  const destNames = new Set(extractFormMakingComponentNames(destMain));

  for (const name of srcNames) {
    if (!destNames.has(name)) {
      issues.push(`[MISSING_FORMMAKING_COMPONENT] src/main.js 注册缺失: ${name}`);
    }
  }

  return issues;
}

const criticalMappings = [
  // 费用报销审核主链路
  { src: 'src/views/GroupApproveManage/', dest: 'src/views/GroupApproveManage/', type: 'dir' },
  { src: 'src/views/BudgetManage/', dest: 'src/views/BudgetManage/', type: 'dir' },
  { src: 'src/components/', dest: 'src/components/', type: 'dir' },
  // alias 实际运行路径
  { src: 'src/views/PerformanceManage/QuarterPerfAssess/', dest: 'src/cross-modules/PerformanceManage/QuarterPerfAssess/', type: 'dir' },
  { src: 'src/views/TaskManage/TaskArrange/components/', dest: 'src/cross-modules/TaskManage/TaskArrange/components/', type: 'dir' },
  { src: 'src/views/BacklogManage/components/NoFormMulBranch/', dest: 'src/cross-modules/BacklogManage/components/NoFormMulBranch/', type: 'dir' },
  { src: 'src/views/flowLibrary/components/', dest: 'src/cross-modules/flowLibrary/components/', type: 'dir' },
  { src: 'src/views/QuestionnaireManage/components/fill.vue', dest: 'src/cross-modules/QuestionnaireManage/components/fill.vue', type: 'file' }
];

let allIssues = [];
for (const item of criticalMappings) {
  allIssues = allIssues.concat(checkPathSync(item.src, item.dest, item.type));
}
allIssues = allIssues.concat(checkFormMakingRegistrations());

if (allIssues.length) {
  console.error('\n同步校验失败：\n');
  for (const line of allIssues) {
    console.error(`- ${line}`);
  }
  process.exit(1);
}

console.log(`sync-check 通过：关键同步路径与源平台一致。环境=${syncEnv} (${reason})`);
