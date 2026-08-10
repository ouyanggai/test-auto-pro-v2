#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');

const MANIFEST_PATH = path.join(__dirname, '..', 'sync-manifest.json');
const VALID_SYNC_ENVS = new Set(['test', 'prod']);

function readManifest() {
  return JSON.parse(fs.readFileSync(MANIFEST_PATH, 'utf8'));
}

function normalizeSyncEnv(value) {
  if (!value) {
    return '';
  }

  const normalized = String(value).trim().toLowerCase();
  if (VALID_SYNC_ENVS.has(normalized)) {
    return normalized;
  }
  if (normalized === 'production') {
    return 'prod';
  }
  if (normalized === 'testing' || normalized === 'uat') {
    return 'test';
  }

  const lifecycleMatch = normalized.match(/(?:^|:)(test|prod)$/);
  return lifecycleMatch ? lifecycleMatch[1] : '';
}

function resolveSyncEnv(env = process.env, manifest = readManifest()) {
  const candidates = [
    { name: 'RSH_FLOW_SYNC_ENV', value: env.RSH_FLOW_SYNC_ENV },
    { name: 'SYNC_ENV', value: env.SYNC_ENV },
    { name: 'BUILD_ENV', value: env.BUILD_ENV },
    { name: 'VUE_APP_FLAG', value: env.VUE_APP_FLAG },
    { name: 'npm_lifecycle_event', value: env.npm_lifecycle_event }
  ];

  for (const candidate of candidates) {
    const syncEnv = normalizeSyncEnv(candidate.value);
    if (syncEnv) {
      return { syncEnv, reason: candidate.name };
    }
  }

  const defaultSyncEnv = normalizeSyncEnv(manifest.defaultSyncEnv) || 'test';
  return { syncEnv: defaultSyncEnv, reason: 'defaultSyncEnv' };
}

function getSyncConfig(env = process.env) {
  const manifest = readManifest();
  const { syncEnv, reason } = resolveSyncEnv(env, manifest);
  const sourceRoot = manifest.sourceRoots && manifest.sourceRoots[syncEnv];

  if (!sourceRoot) {
    throw new Error(`sync-manifest.json 未配置 sourceRoots.${syncEnv}`);
  }

  return {
    manifest,
    syncEnv,
    reason,
    sourceRoot,
    targetRoot: manifest.targetRoot
  };
}

module.exports = {
  getSyncConfig,
  normalizeSyncEnv,
  readManifest,
  resolveSyncEnv
};
