#!/usr/bin/env node
'use strict';

const path = require('path');
const fs = require('fs');
const { spawnSync } = require('child_process');

const ROOT = path.resolve(__dirname, '..');
const flag = String(process.argv[2] || 'test').trim().toLowerCase();
if (!['test', 'prod'].includes(flag)) {
  console.error(`不支持的构建环境: ${flag}，可选 test / prod`);
  process.exit(1);
}

// 不调用 node_modules/.bin/vue-cli-service：该目录可能是从 macOS 拷贝来的 Unix shim，
// Windows 上没有 .cmd 文件。直接启动 CLI 的 JS 入口，避免平台相关的 shim。
const cli = path.join(ROOT, 'node_modules', '@vue', 'cli-service', 'bin', 'vue-cli-service.js');
if (!fs.existsSync(cli)) {
  console.error(`找不到 Vue CLI 构建入口: ${cli}`);
  console.error('请先在当前系统执行 npm install，不要直接使用从 macOS 拷贝来的 node_modules。');
  process.exit(1);
}
const command = process.execPath;
const args = [cli, 'build'];
const result = spawnSync(command, args, {
  cwd: ROOT,
  env: {
    ...process.env,
    VUE_APP_FLAG: flag,
    PATH: `${path.join(ROOT, 'node_modules', '.bin')}${path.delimiter}${process.env.PATH || ''}`
  },
  stdio: 'inherit'
});

if (result.error) {
  console.error(`无法执行 vue-cli-service: ${result.error.message}`);
  process.exit(1);
}
if (result.status === null) {
  console.error('vue-cli-service 未正常退出。');
  process.exit(1);
}
if (result.status !== 0) {
  console.error(`vue-cli-service 构建失败，退出码: ${result.status}`);
}
process.exit(result.status);
