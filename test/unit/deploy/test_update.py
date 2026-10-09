"""更新发布的包边界和失败门禁，使用独立临时 Git 仓库避免修改真实源码。"""

import importlib.util
import json
import pathlib
import subprocess
import tarfile
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[3]
spec = importlib.util.spec_from_file_location('release_update', ROOT / 'deploy/update.py')
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class PackageTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = pathlib.Path(self.temporary.name)
        self.source = self.root / 'executor-service/flow-source'
        self.source.mkdir(parents=True)
        self.run_git(self.source, 'init', '-q')
        self.run_git(self.source, 'config', 'user.name', '发布测试')
        self.run_git(self.source, 'config', 'user.email', 'release-test@example.invalid')
        self.write('executor-service/flow-source/src/page.js', '干净源码')
        self.write('executor-service/flow-source/.npmrc', '未知配置')
        self.run_git(self.source, 'add', '.')
        self.run_git(self.source, 'commit', '-qm', '测试快照')
        head = self.run_git(self.source, 'rev-parse', 'HEAD').strip()
        self.run_git(self.source, 'remote', 'add', 'origin', 'git@example.invalid:flow.git')
        (self.source / '.npmrc').unlink()
        self.run_git(self.root, 'init', '-q')
        self.write('executor-service/flow-source-manifest.json', json.dumps({'head': head, 'remote': 'git@example.invalid:flow.git'}))
        self.write('web/src/current.js', '当前应用代码')
        self.write('deploy/.env', '不准入包')
        self.write('deploy/.env.update.json', '不准入包')
        self.write('logs/current.log', '不准入包')
        self.write('web/node_modules/cache.js', '不准入包')
        self.write('deploy/update-server.sh', '#!/bin/bash\r\necho test\r\n')
        self.write('form-runtime/runtime-source/src/lib/vue-form-making/dist/FormMaking.common.js', '受控产物')
        self.run_git(self.root, 'add', 'web', 'deploy', 'logs', 'form-runtime', 'executor-service/flow-source-manifest.json')
        self.run_git(self.root, 'config', 'user.name', '发布测试')
        self.run_git(self.root, 'config', 'user.email', 'release-test@example.invalid')
        self.run_git(self.root, 'commit', '-qm', '应用测试')

    def run_git(self, root, *args):
        return subprocess.check_output(['git', '-C', str(root), *args]).decode('utf-8')

    def write(self, relative, content):
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding='utf-8', newline='')

    def test_package_excludes_secrets_and_uses_git_objects(self):
        archive = self.root / 'release.tar.gz'
        digest = release.package(self.root, archive)
        self.assertEqual(len(digest), 64)
        with tarfile.open(archive) as package:
            names = package.getnames()
            self.assertIn('web/src/current.js', names)
            self.assertIn('executor-service/flow-source/src/lib/vue-form-making/dist/FormMaking.common.js', names)
            self.assertNotIn('deploy/.env', names)
            self.assertNotIn('deploy/.env.update.json', names)
            self.assertNotIn('logs/current.log', names)
            self.assertNotIn('web/node_modules/cache.js', names)
            self.assertNotIn('executor-service/flow-source/.npmrc', names)
            self.assertNotIn(b'\r', package.extractfile('deploy/update-server.sh').read())

    def test_dirty_execution_source_is_rejected(self):
        self.write('executor-service/flow-source/src/page.js', '未批准改动')
        with self.assertRaisesRegex(RuntimeError, '脏工作树'):
            release.package(self.root, self.root / 'release.tar.gz')

    def test_wrong_snapshot_is_rejected(self):
        manifest = self.root / 'executor-service/flow-source-manifest.json'
        manifest.write_text(json.dumps({'head': '0000000', 'remote': 'git@example.invalid:flow.git'}))
        with self.assertRaisesRegex(RuntimeError, '清单版本'):
            release.package(self.root, self.root / 'release.tar.gz')


if __name__ == '__main__':
    unittest.main()
