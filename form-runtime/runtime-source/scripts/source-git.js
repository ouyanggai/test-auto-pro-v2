'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync, spawnSync } = require('child_process');
const { resolveSourceGitUrl } = require('./sync-config');

function publicUrl(value) {
  return String(value).replace(/(https?:\/\/)[^/@\s]+@/g, '$1');
}

function git(repo, argv) {
  try {
    return execFileSync('git', ['-C', repo, ...argv], {
      encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'],
      env: { ...process.env, GIT_TERMINAL_PROMPT: '0' }
    }).trim();
  } catch (error) {
    throw new Error('Platform Git operation failed: ' + publicUrl(String(error.stderr || error.message)).trim());
  }
}

function openSource({ projectRoot, branch, repository, sourceRoot, sourceRef, sourceUrl }) {
  let temporary = '';
  let disposed = false;
  const dispose = () => {
    if (disposed || !temporary) return;
    const absolute = path.resolve(temporary);
    const allowed = path.resolve(os.tmpdir()) + path.sep + 'rsh-flow-source-';
    if (!absolute.startsWith(allowed)) throw new Error('Unexpected temporary Git cleanup path');
    fs.rmSync(absolute, { recursive: true, force: true });
    disposed = true;
  };

  try {
    let repo = sourceRoot && path.resolve(sourceRoot);
    if (!repo) {
      const componentUrl = git(projectRoot, ['remote', 'get-url', 'origin']);
      const remote = resolveSourceGitUrl(componentUrl, repository, sourceUrl || process.env.RSH_FLOW_SOURCE_GIT_URL);
      temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'rsh-flow-source-'));
      repo = temporary;
      git(repo, ['init', '--bare', '--quiet']);
      git(repo, ['remote', 'add', 'origin', remote]);
      git(repo, ['fetch', '--quiet', '--no-tags', '--depth=1', 'origin',
        'refs/heads/' + branch + ':refs/remotes/origin/' + branch]);
    }
    const remote = git(repo, ['remote', 'get-url', 'origin']);
    if (/192\.168\.1\.155/.test(remote)) throw new Error('Sync source must use the new Git server');
    const ref = sourceRef || 'refs/remotes/origin/' + branch;
    const commit = git(repo, ['rev-parse', '--verify', ref + '^{commit}']);
    git(repo, ['rev-parse', '--verify', 'refs/remotes/origin/' + branch]);
    const belongs = spawnSync('git', ['-C', repo, 'merge-base', '--is-ancestor', commit, 'refs/remotes/origin/' + branch]);
    if (belongs.status !== 0) throw new Error('Source commit does not belong to platform ' + branch);
    return { repo, commit, remote: publicUrl(remote), dispose };
  } catch (error) {
    dispose();
    throw error;
  }
}

module.exports = { openSource, publicUrl };
