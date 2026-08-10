#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');
const { getSyncConfig } = require('./sync-config');

const { manifest, syncEnv, reason, sourceRoot, targetRoot } = getSyncConfig();
const printConfigOnly = process.argv.includes('--print-config');

function ensureDir(dirPath) {
  if (!fs.existsSync(dirPath)) {
    fs.mkdirSync(dirPath, { recursive: true });
  }
}

function copyFileSync(src, dest) {
  ensureDir(path.dirname(dest));
  fs.copyFileSync(src, dest);
}

function copyDirSync(src, dest) {
  if (!fs.existsSync(src)) {
    console.warn(`  [WARN] 源目录不存在: ${src}`);
    return;
  }
  ensureDir(dest);
  const entries = fs.readdirSync(src, { withFileTypes: true });
  for (const entry of entries) {
    const srcPath = path.join(src, entry.name);
    const destPath = path.join(dest, entry.name);
    if (entry.isDirectory()) {
      copyDirSync(srcPath, destPath);
    } else {
      fs.copyFileSync(srcPath, destPath);
    }
  }
}

function readFileIfExists(filePath) {
  if (!fs.existsSync(filePath)) {
    return '';
  }
  return fs.readFileSync(filePath, 'utf8');
}

function extractImports(content) {
  const imports = new Map();
  const importRegExp = /^import\s+([A-Za-z_$][\w$]*)\s+from\s+['"]([^'"]+)['"];?/gm;
  let match;

  while ((match = importRegExp.exec(content))) {
    imports.set(match[1], {
      localName: match[1],
      source: match[2],
      statement: match[0]
    });
  }

  return imports;
}

function extractFormMakingRegistrations(content) {
  const registrations = [];
  const registrationRegExp = /name:\s*['"]([^'"]+)['"]\s*,\s*component:\s*([A-Za-z_$][\w$]*)/g;
  let match;
  const seen = new Set();

  while ((match = registrationRegExp.exec(content))) {
    const key = match[1];
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    registrations.push({
      name: match[1],
      component: match[2]
    });
  }

  return registrations;
}

function insertMissingImports(content, missingImports) {
  if (!missingImports.length) {
    return content;
  }

  const lines = missingImports.map(item => item.statement.replace(/;?$/, ';')).join('\n');
  const importMatches = [...content.matchAll(/^import\s+.*?;?$/gm)];
  let insertAt = -1;

  for (const match of importMatches) {
    if (match[0].includes('@/components/Custom/components/')) {
      insertAt = match.index + match[0].length;
    }
  }

  if (insertAt < 0) {
    const formMakingImport = content.match(/^import\s+FormMaking\s+from\s+['"][^'"]+['"];?/m);
    insertAt = formMakingImport
      ? formMakingImport.index + formMakingImport[0].length
      : importMatches.length
        ? importMatches[importMatches.length - 1].index + importMatches[importMatches.length - 1][0].length
        : 0;
  }

  return `${content.slice(0, insertAt)}\n${lines}${content.slice(insertAt)}`;
}

function insertMissingRegistrations(content, missingRegistrations) {
  if (!missingRegistrations.length) {
    return content;
  }

  const vueUseIndex = content.indexOf('Vue.use(FormMaking');
  if (vueUseIndex < 0) {
    console.warn('[WARN] src/main.js 未找到 Vue.use(FormMaking)，跳过自动注册');
    return content;
  }

  const componentsIndex = content.indexOf('components:', vueUseIndex);
  const arrayStart = content.indexOf('[', componentsIndex);
  const arrayEnd = content.indexOf('\n  ]', arrayStart);
  if (componentsIndex < 0 || arrayStart < 0 || arrayEnd < 0) {
    console.warn('[WARN] src/main.js 未找到 FormMaking components 数组，跳过自动注册');
    return content;
  }

  const insertText = missingRegistrations
    .map(item => `    { name: '${item.name}', component: ${item.component} }`)
    .join(',\n');
  const beforeEnd = content.slice(0, arrayEnd).trimEnd();
  const needsComma = !beforeEnd.endsWith('[') && !beforeEnd.endsWith(',');
  const separator = needsComma ? ',\n' : '\n';

  return `${beforeEnd}${separator}${insertText}${content.slice(arrayEnd)}`;
}

function syncFormMakingMainRegistrations() {
  const sourceMainPath = path.join(sourceRoot, 'src/main.js');
  const targetMainPath = path.join(targetRoot, 'src/main.js');
  const sourceMain = readFileIfExists(sourceMainPath);
  let targetMain = readFileIfExists(targetMainPath);

  if (!sourceMain || !targetMain) {
    console.warn('[WARN] src/main.js 不完整，跳过 FormMaking 自定义组件注册同步');
    return;
  }

  const sourceImports = extractImports(sourceMain);
  const targetImports = extractImports(targetMain);
  const targetRegistrations = new Set(extractFormMakingRegistrations(targetMain).map(item => item.name));
  const missingRegistrations = [];
  const missingImports = [];
  const missingImportKeys = new Set();

  for (const registration of extractFormMakingRegistrations(sourceMain)) {
    if (targetRegistrations.has(registration.name)) {
      continue;
    }

    const importInfo = sourceImports.get(registration.component);
    if (!importInfo) {
      console.warn(`[WARN] 源 main.js 未找到 ${registration.component} 的 import，跳过 ${registration.name}`);
      continue;
    }
    if (!importInfo.source.startsWith('@/components/Custom/components/')) {
      continue;
    }

    missingRegistrations.push(registration);
    if (!targetImports.has(registration.component) && !missingImportKeys.has(registration.component)) {
      missingImports.push(importInfo);
      missingImportKeys.add(registration.component);
    }
  }

  if (!missingRegistrations.length) {
    console.log('[FORM] src/main.js - FormMaking 自定义组件注册已齐全');
    return;
  }

  targetMain = insertMissingImports(targetMain, missingImports);
  targetMain = insertMissingRegistrations(targetMain, missingRegistrations);
  fs.writeFileSync(targetMainPath, targetMain, 'utf8');

  console.log(`[FORM] src/main.js - 自动补齐 ${missingRegistrations.length} 个 FormMaking 自定义组件注册: ${missingRegistrations.map(item => item.name).join(', ')}`);
}

if (printConfigOnly) {
  console.log(JSON.stringify({ syncEnv, reason, sourceRoot, targetRoot }, null, 2));
  process.exit(0);
}

// 执行同步
console.log('=== 开始同步流程组件 ===\n');
console.log(`同步环境: ${syncEnv} (${reason})`);
console.log(`源目录: ${sourceRoot}`);
console.log(`目标目录: ${targetRoot}\n`);
let syncCount = 0;

for (const mapping of manifest.mappings) {
  const src = path.join(sourceRoot, mapping.src);
  const dest = path.join(targetRoot, mapping.dest);

  if (!fs.existsSync(src)) {
    console.warn(`[SKIP] 源不存在: ${mapping.src}`);
    continue;
  }

  if (mapping.type === 'dir') {
    console.log(`[DIR]  ${mapping.src} → ${mapping.dest}`);
    copyDirSync(src, dest);
  } else {
    console.log(`[FILE] ${mapping.src} → ${mapping.dest}`);
    copyFileSync(src, dest);
  }
  syncCount++;
}

console.log(`\n同步完成: ${syncCount}/${manifest.mappings.length} 个映射\n`);

// === 后处理：stub globalDialog/components.js ===
console.log('=== 后处理 ===\n');

syncFormMakingMainRegistrations();

const componentsJsPath = path.join(targetRoot, 'src/layout/components/globalDialog/components.js');
if (fs.existsSync(componentsJsPath)) {
  const stubContent = `// 自动生成 - 由 sync.js 后处理
// stub 掉非流程组件，保留流程相关组件

import orgTree from './components/orgTree.vue';
import flowDetail from './components/flowDetail.vue';
import loanMoney from './components/loanMoney.vue';

// stub: commonAccounts (导航栏组件，流程不需要)
const commonAccounts = { render: h => h('div') };
// stub: invoiceCommonInfo (合同管理组件，流程不需要)
const invoiceCommonInfo = { render: h => h('div') };

export default { orgTree, commonAccounts, flowDetail, invoiceCommonInfo, loanMoney };
`;
  fs.writeFileSync(componentsJsPath, stubContent, 'utf8');
  console.log('[STUB] globalDialog/components.js - stub commonAccounts, invoiceCommonInfo');
}

// === 后处理：写入运行时 config/env.js ===
const envJsPath = path.join(targetRoot, 'src/config/env.js');
const envContent = `// 运行时可配置环境变量
// 可通过 window.__RSH_FLOW_CONFIG__ 在运行时覆盖

const defaultConfig = (() => {
  const isDev = process.env.NODE_ENV === 'development';
  const flag = process.env.VUE_APP_FLAG;

  if (isDev) {
    return {
      baseUrl: 'http://192.168.1.220:28081/api',
      viewFileUrl: 'http://192.168.1.220:28081',
      onlyOfficeUrl: 'http://192.168.1.218:8085'
    };
  }

  if (flag === 'dev') {
    return {
      baseUrl: 'http://192.168.1.218:8077/api',
      viewFileUrl: 'http://192.168.1.218:8077',
      onlyOfficeUrl: 'http://192.168.1.218:8085'
    };
  }

  if (flag === 'test') {
    return {
      baseUrl: 'http://192.168.1.220:38081/api',
      viewFileUrl: 'http://192.168.1.220:38081',
      onlyOfficeUrl: 'http://192.168.1.195:1080'
    };
  }

  return {
    baseUrl: 'https://iserver.runshihua.com/api',
    viewFileUrl: 'https://reader.runshihua.com',
    onlyOfficeUrl: 'https://ioffice.runshihua.com'
  };
})();

const runtimeConfig = window.__RSH_FLOW_CONFIG__ || {};

export let baseUrl = runtimeConfig.baseUrl || defaultConfig.baseUrl;
export let viewFileUrl = runtimeConfig.viewFileUrl || defaultConfig.viewFileUrl;
export let onlyOfficeUrl = runtimeConfig.onlyOfficeUrl || defaultConfig.onlyOfficeUrl;
export let wsUrl = '';

export function setConfig(config) {
  if (config.baseUrl) baseUrl = config.baseUrl;
  if (config.viewFileUrl) viewFileUrl = config.viewFileUrl;
  if (config.onlyOfficeUrl) onlyOfficeUrl = config.onlyOfficeUrl;
}
`;
ensureDir(path.dirname(envJsPath));
fs.writeFileSync(envJsPath, envContent, 'utf8');
console.log('[ENV]  config/env.js - 写入运行时可配置版本');

console.log('\n=== 同步 + 后处理完成 ===');
