#!/usr/bin/env node
'use strict';

const { execFileSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const ROOT = path.resolve(__dirname, '..');
const args = process.argv.slice(2);

const isDryRun = args.includes('--dry-run');
const isMinor = args.includes('--minor');
const isMajor = args.includes('--major');
const envArg = args.find(arg => arg.startsWith('--env='));
const publishEnv = envArg ? envArg.slice('--env='.length).trim().toLowerCase() : 'prod';

if (!['test', 'prod'].includes(publishEnv)) {
  console.error(`不支持的发布环境: ${publishEnv}，可选 test / prod`);
  process.exit(1);
}

function resolveNpmRuntime() {
  if (process.platform !== 'win32') {
    return { command: 'npm', prefix: [], options: {} };
  }

  const candidates = [];
  if (process.env.npm_execpath) {
    candidates.push(process.env.npm_execpath);
  }

  for (const directory of (process.env.PATH || '').split(path.delimiter)) {
    if (directory) {
      candidates.push(path.join(directory, 'npm.cmd'));
    }
  }

  for (const npmCmd of candidates) {
    if (!fs.existsSync(npmCmd)) {
      continue;
    }
    const npmCli = npmCmd.endsWith('.cmd')
      ? path.join(path.dirname(npmCmd), 'node_modules', 'npm', 'bin', 'npm-cli.js')
      : npmCmd;
    if (fs.existsSync(npmCli)) {
      return { command: process.execPath, prefix: [npmCli], options: {} };
    }
  }

  // Last resort for non-standard Node installations.
  return { command: 'npm.cmd', prefix: [], options: { shell: true } };
}

const npmRuntime = resolveNpmRuntime();

function formatCommand(command, commandArgs) {
  return [command, ...commandArgs].join(' ');
}

function run(command, commandArgs = [], options = {}) {
  console.log(`\n> ${formatCommand(command, commandArgs)}\n`);
  const { force = false, ...execOptions } = options;
  if (isDryRun && !force) {
    console.log('  [DRY-RUN] 跳过执行');
    return;
  }

  return execFileSync(command, commandArgs, {
    cwd: ROOT,
    stdio: 'inherit',
    ...execOptions
  });
}

function runOutput(command, commandArgs = []) {
  return execFileSync(command, commandArgs, {
    cwd: ROOT,
    encoding: 'utf8'
  }).trim();
}

function runNpm(commandArgs, options = {}) {
  return run(npmRuntime.command, [...npmRuntime.prefix, ...commandArgs], {
    ...npmRuntime.options,
    ...options
  });
}

function runNpmOutput(commandArgs) {
  return runOutput(npmRuntime.command, [...npmRuntime.prefix, ...commandArgs]);
}

try {
  // Step 1: 同步代码
  console.log(`=== 1/4 同步代码 (${publishEnv}) ===`);
  runNpm(['run', `sync:${publishEnv}`], { force: true });
  runNpm(['run', `sync:check:${publishEnv}`], { force: true });

  // Step 2: 版本号
  console.log('\n=== 2/4 更新版本号 ===');
  const versionType = isMajor ? 'major' : isMinor ? 'minor' : 'patch';
  if (!isDryRun) {
    runNpm(['version', versionType, '--no-git-tag-version']);
  }
  const pkg = JSON.parse(fs.readFileSync(path.join(ROOT, 'package.json'), 'utf8'));
  console.log(`版本: ${pkg.version}`);

  // Step 3: 构建
  console.log('\n=== 3/4 构建 ===');
  run(process.execPath, [path.join('scripts', 'build.js'), publishEnv]);

  // Step 4: 发布
  console.log('\n=== 4/4 发布到 Verdaccio ===');
  runNpm(['publish', '--registry', 'http://192.168.1.248:4873/']);

  console.log('\n=== 发布完成 ===');
  console.log(`@rsh/flow-components@${pkg.version} 已发布到 http://192.168.1.248:4873/`);

  // 验证
  if (!isDryRun) {
    try {
      const info = runNpmOutput([
        'view',
        '@rsh/flow-components',
        'version',
        '--registry',
        'http://192.168.1.248:4873/'
      ]);
      console.log(`验证: 远程版本 ${info}`);
    } catch (error) {
      console.warn('验证失败，请手动检查: npm view @rsh/flow-components --registry http://192.168.1.248:4873/');
    }
  }
} catch (error) {
  const detail = error && error.message ? error.message : String(error);
  console.error(`\n发布失败: ${detail}`);
  console.error(`请确认当前分支与发布环境一致，并使用 npm run publish:flow -- --env=${publishEnv} 重试。`);
  process.exit(1);
}
