#!/usr/bin/env node
'use strict';

const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const ROOT = path.resolve(__dirname, '..');
const args = process.argv.slice(2);

const isDryRun = args.includes('--dry-run');
const isMinor = args.includes('--minor');
const isMajor = args.includes('--major');

function run(cmd, opts = {}) {
  console.log(`\n> ${cmd}\n`);
  if (!isDryRun || opts.force) {
    return execSync(cmd, { cwd: ROOT, stdio: 'inherit', ...opts });
  } else {
    console.log('  [DRY-RUN] 跳过执行');
  }
}

function runOutput(cmd) {
  return execSync(cmd, { cwd: ROOT, encoding: 'utf8' }).trim();
}

// Step 1: 同步代码
console.log('=== 1/4 同步代码 ===');
run('npm run sync:prod', { force: true });
run('npm run sync:check:prod', { force: true });

// Step 2: 版本号
console.log('\n=== 2/4 更新版本号 ===');
const versionType = isMajor ? 'major' : isMinor ? 'minor' : 'patch';
if (!isDryRun) {
  run(`npm version ${versionType} --no-git-tag-version`);
}
const pkg = JSON.parse(fs.readFileSync(path.join(ROOT, 'package.json'), 'utf8'));
console.log(`版本: ${pkg.version}`);

// Step 3: 构建
console.log('\n=== 3/4 构建 ===');
run('env VUE_APP_FLAG=prod npx vue-cli-service build');

// Step 4: 发布
console.log('\n=== 4/4 发布到 Verdaccio ===');
run('npm publish --registry http://192.168.1.248:4873/');

console.log('\n=== 发布完成 ===');
console.log(`@rsh/flow-components@${pkg.version} 已发布到 http://192.168.1.248:4873/`);

// 验证
if (!isDryRun) {
  try {
    const info = runOutput('npm view @rsh/flow-components version --registry http://192.168.1.248:4873/');
    console.log(`验证: 远程版本 ${info}`);
  } catch (e) {
    console.warn('验证失败，请手动检查: npm view @rsh/flow-components --registry http://192.168.1.248:4873/');
  }
}
