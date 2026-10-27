#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');

function readManifest() {
  return JSON.parse(fs.readFileSync(path.resolve(__dirname, '../sync-manifest.json'), 'utf8'));
}

function normalizeSyncEnv(value) {
  const normalized = String(value || '').trim().toLowerCase();
  if (normalized === 'production') return 'prod';
  if (normalized === 'testing' || normalized === 'uat') return 'test';
  if (normalized === 'test' || normalized === 'prod') return normalized;
  const match = normalized.match(/(?:^|:)(test|prod)$/);
  return match ? match[1] : '';
}

function resolveSourceGitUrl(componentUrl, repository = 'rsh-cloud-invest-power-system', override = '') {
  const sourceUrl = override || String(componentUrl).replace(/rsh-flow-components(?:\.git)?\/?$/, repository + '.git');
  if (!sourceUrl || sourceUrl === componentUrl || /192\.168\.1\.155/.test(sourceUrl)) {
    throw new Error('Configure the platform repository on the new Git server');
  }
  if (!sourceUrl.replace(/\/$/, '').endsWith('/' + repository + '.git') &&
      !sourceUrl.replace(/\/$/, '').endsWith('/' + repository)) {
    throw new Error('Unexpected platform Git repository');
  }
  return sourceUrl;
}

module.exports = { readManifest, normalizeSyncEnv, resolveSourceGitUrl };
