"""更新发布包直接消费已校验的 test 业务候选，并拒绝本机敏感文件。"""

import hashlib
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


def candidate_digest(root):
    """按 source-integrity.js 的路径和字节规则计算候选摘要。"""
    skip_names = {
        '.git', 'node_modules', '.native-page-build.json', '.npmrc',
        '.claude', '.omc', '.DS_Store', '__MACOSX',
    }
    files = []
    candidate = root / 'executor-service/flow-source'
    for item in candidate.rglob('*'):
        relative = item.relative_to(candidate).as_posix()
        if any(part in skip_names for part in pathlib.PurePosixPath(relative).parts):
            continue
        if pathlib.PurePosixPath(relative).parts[0] == 'dist':
            continue
        if item.is_file():
            files.append(relative)
    digest = hashlib.sha256()
    for relative in sorted(files):
        digest.update(relative.encode())
        digest.update(b'\0')
        digest.update((candidate / pathlib.PurePosixPath(relative)).read_bytes())
        digest.update(b'\0')
    return digest.hexdigest()


class PackageTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = pathlib.Path(self.temporary.name)
        self.run_git('init', '-q')
        self.run_git('config', 'user.name', '发布测试')
        self.run_git('config', 'user.email', 'release-test@example.invalid')

        self.write('web/src/current.js', '当前应用代码')
        self.write('deploy/update-server.sh', '#!/bin/bash\r\necho test\r\n')
        self.write('form-runtime/runtime-source/src/lib/vue-form-making/dist/FormMaking.common.js', '受控产物')
        self.write('executor-service/flow-source/src/main.js', '执行器适配层')
        self.write('executor-service/flow-source/src/views/GroupApproveManage/contract.js', '全过程 test 合同代码')
        self.write('executor-service/flow-source/src/lib/vue-form-making/dist/FormMaking.common.js', '候选内受控产物')
        self.write('executor-service/flow-source/dist/index.html', '构建产物，不入包')
        self.write('executor-service/flow-source/.env.test', '凭证，不入包')
        self.write('executor-service/flow-source/private.key', '私钥，不入包')
        self.write('executor-service/flow-source/debug.log', '日志，不入包')
        self.write('executor-service/flow-source/node_modules/cache.js', '依赖，不入包')

        manifest = {
            'schemaVersion': 2,
            'mappingVersion': 'f053-package-test',
            'base': {
                'repository': 'rsh-flow-components',
                'remote': 'base-remote',
                'branch': 'test',
            },
            'business': {
                'repository': 'rsh-cloud-invest-power-system',
                'remote': 'business-remote',
                'branch': 'test',
            },
            'mappings': [
                {
                    'source': 'src/views/GroupApproveManage',
                    'destination': 'src/views/GroupApproveManage',
                    'kind': 'directory',
                },
            ],
            'protected': ['src/main.js'],
        }
        self.write('executor-service/flow-source-manifest.json', json.dumps(manifest, ensure_ascii=False))
        mapping_hash = hashlib.sha256(json.dumps({
            'mappingVersion': manifest['mappingVersion'],
            'mappings': manifest['mappings'],
            'protected': manifest['protected'],
        }, ensure_ascii=False, separators=(',', ':')).encode()).hexdigest()
        self.write('executor-service/flow-source.json', json.dumps({
            'schemaVersion': 2,
            'repository': 'composed-flow-source',
            'mappingVersion': manifest['mappingVersion'],
            'mappingHash': mapping_hash,
            'candidateDigest': candidate_digest(self.root),
            'baseRepository': 'rsh-flow-components',
            'baseRemote': 'base-remote',
            'baseBranch': 'test',
            'baseHead': 'a' * 40,
            'baseDigest': 'b' * 64,
            'businessRepository': 'rsh-cloud-invest-power-system',
            'businessRemote': 'business-remote',
            'businessBranch': 'test',
            'businessHead': 'c' * 40,
            'businessDigest': 'd' * 64,
            'businessFileCount': 1,
            'preparedAt': '2026-10-27T00:00:00.000Z',
        }, ensure_ascii=False))
        self.run_git('add', 'web', 'deploy', 'form-runtime', 'executor-service/flow-source-manifest.json')
        self.run_git('commit', '-qm', '应用测试')

    def run_git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.root), *args]).decode('utf-8')

    def write(self, relative, content):
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding='utf-8', newline='')

    def test_package_contains_test_business_and_adapter_with_source_facts(self):
        archive = self.root / 'release.tar.gz'
        digest = release.package(self.root, archive)
        self.assertEqual(len(digest), 64)
        with tarfile.open(archive) as package:
            names = package.getnames()
            self.assertIn('web/src/current.js', names)
            self.assertIn('executor-service/flow-source/src/main.js', names)
            self.assertIn('executor-service/flow-source/src/views/GroupApproveManage/contract.js', names)
            self.assertIn('executor-service/flow-source/src/lib/vue-form-making/dist/FormMaking.common.js', names)
            self.assertIn('executor-service/flow-source.json', names)
            self.assertIn('deploy/release-source.json', names)
            self.assertNotIn('executor-service/flow-source/dist/index.html', names)
            self.assertNotIn('executor-service/flow-source/.env.test', names)
            self.assertNotIn('executor-service/flow-source/private.key', names)
            self.assertNotIn('executor-service/flow-source/debug.log', names)
            self.assertNotIn('executor-service/flow-source/node_modules/cache.js', names)
            self.assertNotIn('executor-service/flow-source/.git/HEAD', names)
            self.assertNotIn(b'\r', package.extractfile('deploy/update-server.sh').read())
            release_source = json.loads(package.extractfile('deploy/release-source.json').read())
            self.assertEqual(release_source['baseRemote'], 'base-remote')
            self.assertEqual(release_source['businessRemote'], 'business-remote')
            self.assertEqual(release_source['businessHead'], 'c' * 40)
            self.assertEqual(release_source['candidateDigest'], candidate_digest(self.root))

    def test_changed_candidate_is_rejected_before_packaging(self):
        self.write('executor-service/flow-source/src/views/GroupApproveManage/contract.js', '未批准改动')
        with self.assertRaises(subprocess.CalledProcessError):
            release.package(self.root, self.root / 'release.tar.gz')

    def test_package_path_rejects_unsupported_secret_suffix(self):
        self.write('executor-service/flow-source/private.p12', '私钥')
        with tarfile.open(self.root / 'release.tar.gz', 'w:gz') as package:
            for relative in ['executor-service/flow-source/private.p12']:
                if release.allowed_candidate(relative):
                    release.add_bytes(package, relative, (self.root / relative).read_bytes())
        with tarfile.open(self.root / 'release.tar.gz') as package:
            self.assertNotIn('executor-service/flow-source/private.p12', package.getnames())


if __name__ == '__main__':
    unittest.main()
