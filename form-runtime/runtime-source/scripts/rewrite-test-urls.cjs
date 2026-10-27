'use strict';

const fs = require('fs');
const path = require('path');
const zlib = require('zlib');

const replacements = [
  ['https://cloudreve.runshihua.com', 'http://cloudreve.rshxnk.com:18080'],
  ['https://portal.runshihua.com:28443', 'http://cloudreve.rshxnk.com:18080'],
  ['https://iserver.runshihua.com', 'http://iserver.rshxnk.com:18080'],
  ['wss://iserver.runshihua.com', 'ws://iserver.rshxnk.com:18080'],
  ['https://bserver.runshihua.com', 'http://bserver.rshxnk.com:18080'],
  ['https://reader.runshihua.com', 'http://reader.rshxnk.com:18080'],
  ['https://office.runshihua.com', 'http://office.rshxnk.com:18080'],
  ['https://portal.runshihua.com', 'http://portal.rshxnk.com:18080'],
  ['http://192.168.1.220:38081', 'http://iserver.rshxnk.com:18080'],
  ['http://192.168.1.220:8081', 'http://iserver.rshxnk.com:18080'],
  ['ws://192.168.1.220:9187', 'ws://iserver.rshxnk.com:18080/sk'],
  ['http://192.168.1.220:8012', 'http://reader.rshxnk.com:18080'],
  ['http://192.168.1.195:1080', 'http://office.rshxnk.com:18080'],
  ['http://192.168.1.203:3000', 'http://portal.rshxnk.com:18080'],
  ['http://192.168.1.26:5212', 'http://cloudreve.rshxnk.com:18080'],
];

function rewrite(directory, mode) {
  if (mode !== 'test') throw new Error('URL replacement is restricted to test builds');
  const root = fs.realpathSync(directory);
  let changed = 0;
  function walk(dir) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name);
      if (entry.isSymbolicLink()) throw new Error('Build output contains a symbolic link');
      if (entry.isDirectory()) { walk(file); continue; }
      if (!/\.(js|css|json|html)$/.test(file)) continue;
      let content = fs.readFileSync(file, 'utf8');
      const original = content;
      for (const [from, to] of replacements) {
        content = content.split(from).join(to);
        content = content.split(from.replaceAll('/', '\\/')).join(to.replaceAll('/', '\\/'));
      }
      if (content === original) continue;
      fs.writeFileSync(file, content);
      if (fs.existsSync(file + '.gz')) fs.writeFileSync(file + '.gz', zlib.gzipSync(content));
      if (fs.existsSync(file + '.br')) fs.writeFileSync(file + '.br', zlib.brotliCompressSync(content));
      changed++;
    }
  }
  walk(root);
  return changed;
}

if (require.main === module) {
  console.log('Updated test deployment addresses in ' + rewrite(path.resolve(__dirname, '../dist'), process.argv[2]) + ' files');
}
module.exports = { rewrite };
