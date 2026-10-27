#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { execFileSync } = require('child_process');
const { openSource } = require('./source-git');

const root = path.resolve(__dirname, '..');
const args = process.argv.slice(2);
const option = name => args.includes(name) ? args[args.indexOf(name) + 1] : '';
const hash = data => crypto.createHash('sha256').update(data).digest('hex');
const git = (repo, argv, options = {}) => execFileSync('git', ['-C', repo, ...argv], {
  encoding: 'utf8', maxBuffer: 128 * 1024 * 1024, ...options
});
const manifest = JSON.parse(fs.readFileSync(path.join(root, 'sync-manifest.json'), 'utf8'));
const recordPath = path.join(root, 'sync-source-lock.json');

function safePath(relative) {
  const absolute = path.resolve(root, relative);
  if (!absolute.startsWith(root + path.sep)) throw new Error('Sync path escapes project: ' + relative);
  for (let current = absolute; current !== root; current = path.dirname(current)) {
    if (fs.existsSync(current) && fs.lstatSync(current).isSymbolicLink()) {
      throw new Error('Sync path contains a symbolic link: ' + relative);
    }
  }
  return absolute;
}

function walk(relative) {
  const absolute = safePath(relative);
  if (!fs.existsSync(absolute)) return [];
  if (fs.lstatSync(absolute).isFile()) return [relative];
  return fs.readdirSync(absolute).flatMap(name => walk(relative.replace(/\/$/, '') + '/' + name));
}

function registrations(main) {
  const block = main.match(/^Vue\.use\(FormMaking, \{[\s\S]*?^\}\);/m);
  if (!block) throw new Error('FormMaking registration block missing');
  const imports = [...main.matchAll(/^import\s+([A-Za-z_$][\w$]*)\s+from\s+['"]([^'"]+)['"];?/gm)];
  const names = [...new Set([...block[0].matchAll(/component:\s*([A-Za-z_$][\w$]*)/g)].map(match => match[1]))];
  const resolved = names.map(name => {
    const found = imports.find(match => match[1] === name);
    if (!found) throw new Error('Missing custom component import: ' + name);
    return { name, from: found[2] };
  });
  return { block: block[0], imports: resolved };
}

function snapshotIssues(record) {
  const issues = [];
  if (record.manifestSha256 !== hash(fs.readFileSync(path.join(root, 'sync-manifest.json')))) {
    issues.push('Sync manifest differs from recorded platform snapshot');
  }
  for (const [relative, expected] of Object.entries(record.files)) {
    const file = safePath(relative);
    if (!fs.existsSync(file)) issues.push('Missing: ' + relative);
    else if (hash(fs.readFileSync(file)) !== expected) issues.push('Content differs: ' + relative);
  }
  for (const mapping of manifest.mappings) {
    for (const relative of walk(mapping.dest.replace(/\/$/, ''))) {
      if (!(relative in record.files)) issues.push('Untracked platform-owned file: ' + relative);
    }
  }
  const current = registrations(fs.readFileSync(path.join(root, 'src/main.js'), 'utf8'));
  if (JSON.stringify(current) !== JSON.stringify(record.registrations)) issues.push('Custom component registrations differ');
  return [...new Set(issues)];
}

function run() {
  if (args.includes('--snapshot')) {
    const record = JSON.parse(fs.readFileSync(recordPath, 'utf8'));
    if (option('--env') && record.environment !== option('--env')) {
      throw new Error('Platform snapshot does not match the selected build environment');
    }
    const issues = snapshotIssues(record);
    if (issues.length) throw new Error(issues.join('\n'));
    console.log(`Platform snapshot verified: ${record.sourceBranch}@${record.sourceCommit}; FormMaking ${record.formMakingTree}`);
    return;
  }

  const syncEnv = option('--env');
  if (!['prod', 'test'].includes(syncEnv)) throw new Error('Specify --env prod or --env test');
  const branch = syncEnv === 'prod' ? 'master' : 'test';
  const currentBranch = git(root, ['branch', '--show-current']).trim();
  const targetBranch = currentBranch || option('--target-branch');
  if (targetBranch !== branch) {
    throw new Error(`Target branch must be ${branch}, got ${currentBranch}`);
  }
  if (manifest.sourceBranches[syncEnv] !== branch || manifest.targetBranches[syncEnv] !== branch) {
    throw new Error('Platform and component branch mapping must match the selected environment');
  }
  protectLocalChanges();
  const source = openSource({ projectRoot: root, branch, repository: manifest.sourceRepository,
    sourceRoot: option('--source-root'), sourceRef: option('--source-ref'), sourceUrl: option('--source-url') });
  try {
    synchronize(source, syncEnv);
  } finally {
    source.dispose();
  }
}

function protectLocalChanges() {
  if (!fs.existsSync(recordPath)) return;
  const record = JSON.parse(fs.readFileSync(recordPath, 'utf8'));
  const dirty = [...new Set([
    ...git(root, ['diff', '--name-only', '-z', 'HEAD']).split('\0'),
    ...git(root, ['ls-files', '--others', '--exclude-standard', '-z']).split('\0')
  ].filter(Boolean))];
  for (const relative of dirty) {
    if (!(relative in record.files)) {
      if (manifest.mappings.some(mapping => mapping.type === 'dir' ? relative.startsWith(mapping.dest) : relative === mapping.dest)) {
        throw new Error('Preserve untracked platform-owned files before syncing: ' + relative);
      }
      continue;
    }
    const file = safePath(relative);
    if (!fs.existsSync(file) || hash(fs.readFileSync(file)) !== record.files[relative]) {
      throw new Error('Commit or preserve local platform-owned edits before syncing: ' + relative);
    }
  }
  if (dirty.includes('src/main.js')) {
    const current = registrations(fs.readFileSync(path.join(root, 'src/main.js'), 'utf8'));
    if (JSON.stringify(current) !== JSON.stringify(record.registrations)) {
      throw new Error('Commit or preserve local custom registrations before syncing');
    }
  }
}

function synchronize(source, syncEnv) {
  const { repo: sourceRoot, commit: sourceCommit, remote: sourceRemote } = source;
  const branch = syncEnv === 'prod' ? 'master' : 'test';
  const sourceTree = git(sourceRoot, ['ls-tree', '-r', '-z', sourceCommit]).split('\0').filter(Boolean);
  const sourceFiles = new Map(sourceTree.map(line => {
    const match = line.match(/^([0-7]+) blob ([a-f0-9]+)\t([\s\S]+)$/);
    if (!match) throw new Error('Unsupported source entry: ' + line);
    return [match[3], { mode: match[1], blob: match[2] }];
  }));
  const projected = new Map();
  for (const mapping of manifest.mappings) {
    safePath(mapping.dest);
    let count = 0;
    for (const [file, entry] of sourceFiles) {
      if (mapping.type === 'file' ? file === mapping.src : file.startsWith(mapping.src)) {
        if (entry.mode !== '100644' && entry.mode !== '100755') throw new Error('Unsupported source mode');
        projected.set(mapping.dest + (mapping.type === 'dir' ? file.slice(mapping.src.length) : ''), entry);
        count++;
      }
    }
    if (!count) throw new Error('Source mapping missing: ' + mapping.src);
  }
  const mainEntry = sourceFiles.get('src/main.js');
  const entries = [...new Set([...projected.values()].map(entry => entry.blob).concat(mainEntry.blob))];
  const output = git(sourceRoot, ['cat-file', '--batch'], { input: entries.join('\n') + '\n', encoding: null });
  const blobs = new Map();
  let offset = 0;
  for (const blob of entries) {
    const end = output.indexOf(10, offset);
    const [actual, type, sizeText] = output.subarray(offset, end).toString().split(' ');
    if (actual !== blob || type !== 'blob') throw new Error('Invalid Git blob response');
    const size = Number(sizeText);
    blobs.set(blob, output.subarray(end + 1, end + 1 + size));
    offset = end + 2 + size;
  }

  const sourceMain = registrations(blobs.get(mainEntry.blob).toString('utf8'));
  const files = {};
  const changes = [];
  const writes = new Map();
  for (const [relative, entry] of projected) {
    const override = manifest.syncOverrides && manifest.syncOverrides[relative];
    const data = override ? fs.readFileSync(safePath(override)) : blobs.get(entry.blob);
    if (override && relative.startsWith('src/lib/vue-form-making/')) throw new Error('FormMaking cannot be overridden');
    files[relative] = hash(data);
    writes.set(relative, data);
    const target = safePath(relative);
    if (!fs.existsSync(target) || hash(fs.readFileSync(target)) !== files[relative]) changes.push(relative);
  }
  for (const relative of manifest.targetOnly || []) {
    const file = safePath(relative);
    if (!fs.existsSync(file)) throw new Error('Missing declared host integration: ' + relative);
    files[relative] = hash(fs.readFileSync(file));
  }
  const removals = new Set();
  for (const mapping of manifest.mappings) for (const relative of walk(mapping.dest.replace(/\/$/, ''))) {
    if (!(relative in files)) removals.add(relative);
  }
  console.log(JSON.stringify({ environment: syncEnv, sourceCommit, changes, removals: [...removals] }, null, 2));
  if (!args.includes('--write')) return;

  for (const [relative, data] of writes) {
    const file = safePath(relative);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, data);
  }
  for (const relative of removals) fs.unlinkSync(safePath(relative));
  const mainPath = path.join(root, 'src/main.js');
  let main = fs.readFileSync(mainPath, 'utf8');
  main = main.replace(/^import\s+\w+\s+from\s+['"]@\/components\/Custom\/components\/[^'"\n]+['"];?\r?\n/gm, '');
  main = main.replace(/(?:\r?\n)+(?=^Vue\.use\(FormMaking, \{)/m, '\n\n');
  main = main.replace(/^(?:\r?\n)+(?=Vue\.use\(FormMaking, \{)/, '');
  main = main.replace(/^Vue\.use\(FormMaking, \{[\s\S]*?^\}\);/m,
    sourceMain.imports.map(item => `import ${item.name} from '${item.from}';`).join('\n') + '\n\n' + sourceMain.block);
  fs.writeFileSync(mainPath, main);
  const record = { environment: syncEnv, sourceRemote, sourceBranch: branch, sourceCommit,
    formMakingTree: git(sourceRoot, ['rev-parse', sourceCommit + ':src/lib/vue-form-making']).trim(),
    manifestSha256: hash(fs.readFileSync(path.join(root, 'sync-manifest.json'))),
    registrations: sourceMain, files };
  fs.writeFileSync(recordPath, JSON.stringify(record, null, 2) + '\n');
  const issues = snapshotIssues(record);
  if (issues.length) throw new Error(issues.join('\n'));
}

if (require.main === module) {
  try { run(); } catch (error) { console.error(error.message); process.exitCode = 1; }
}
module.exports = { hash, registrations, safePath, snapshotIssues, run };
