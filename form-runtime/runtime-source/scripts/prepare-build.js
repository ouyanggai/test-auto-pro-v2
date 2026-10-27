'use strict';

const { execFileSync } = require('child_process');
const path = require('path');

const args = process.argv.slice(2);
const syncEnv = args[args.indexOf('--env') + 1];
if (!['prod', 'test'].includes(syncEnv)) throw new Error('Build environment must be prod or test');
const prepared = process.env.RSH_FLOW_SYNC_PREPARED === '1';
execFileSync(process.execPath, [path.join(__dirname, prepared ? 'sync-check.js' : 'sync.js'),
  '--env', syncEnv, ...(prepared ? [] : ['--write'])], { stdio: 'inherit' });
