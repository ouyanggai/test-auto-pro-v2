#!/usr/bin/env python3
"""从本机发布当前受 Git 管理的应用代码，保留服务器运行配置与数据。"""

import argparse
import hashlib
import io
import json
import os
import pathlib
import select
import shlex
import socket
import subprocess
import sys
import tarfile
import tempfile
import threading
import time
import uuid

ROOT = pathlib.Path(__file__).resolve().parent.parent
NOISE = {'.npmrc', '.claude', '.omc'}
APP_DIRS = {'cmd', 'internal', 'web', 'form-runtime', 'executor-service', 'deploy'}


def configure_output():
    """把本机标准输出固定为 UTF-8，且不可编码字符按替换处理。

    Windows 控制台默认 GBK：服务器构建输出里的勾选等 Unicode 符号无法用 GBK 表示，
    直接 print 会抛 UnicodeEncodeError，把整个发布从本机侧打断（服务器其实构建正常），
    结果发布失败、现场留在服务器上。发布只能因服务器失败而失败，不能因本机输出编码失败。
    """
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, 'reconfigure', None)
        if reconfigure is None:
            continue
        try:
            reconfigure(encoding='utf-8', errors='replace')
        except (ValueError, OSError):
            # 输出被重定向到不支持重配的对象时保持原样，由 emit 兜底。
            pass


def emit(text):
    """安全输出一段服务器日志：任何编码问题都不允许中断发布。"""
    try:
        sys.stdout.write(text)
        sys.stdout.flush()
    except UnicodeEncodeError:
        sys.stdout.write(text.encode('utf-8', 'replace').decode('ascii', 'replace'))
        sys.stdout.flush()


def git(root, *args):
    """只读访问 Git；保留 porcelain 的前导空格，防止误判暂存改动。"""
    return subprocess.check_output(['git', '-C', str(root), *args])


def allowed(relative):
    """发布只消费应用和 F-023 验活文件；敏感文件即使误入 Git 也拒绝打包。"""
    path = pathlib.PurePosixPath(relative)
    if path.is_absolute() or '..' in path.parts:
        return False
    if any(part in {'.git', 'node_modules', '.runtime', 'logs'} for part in path.parts):
        return False
    if path.name.startswith('.env') or path.name == '.npmrc' or path.suffix in {'.key', '.pem', '.log'}:
        return False
    if relative.startswith(('web/dist/', 'executor-service/flow-source/', 'executor-service/page/dist/', 'executor-service/dist/')):
        return False
    return path.parts[0] in APP_DIRS or relative in {
        '.dockerignore', 'go.mod', 'go.sum', 'package.json', 'pnpm-lock.yaml',
        'pnpm-workspace.yaml', 'test/integration/f023_deployment_smoke.sh',
    }


def clean_upstream(relative):
    """参考源码的构建对象同样排除凭证和仓库内自带的本机噪声。"""
    path = pathlib.PurePosixPath(relative)
    return not any(part in NOISE or part.startswith('.env') for part in path.parts) and path.suffix not in {'.key', '.pem'}


def prepare_source(root):
    """缺少清单快照时调用现有同步入口，真实脏改动必须先拒绝而不能被同步覆盖。"""
    manifest = json.loads((root / 'executor-service/flow-source-manifest.json').read_text(encoding='utf-8'))
    source = root / 'executor-service/flow-source'
    if (source / '.git').exists():
        for line in git(source, 'status', '--porcelain', '--untracked-files=no').decode().splitlines():
            if not (line.startswith(' D ') and line[3:].split('/')[0] in NOISE):
                raise RuntimeError('执行源码有未批准修改，拒绝同步或发布脏工作树')
        if (git(source, 'rev-parse', 'HEAD').decode().strip() == manifest['head'] and
                git(source, 'remote', 'get-url', 'origin').decode().strip() == manifest['remote']):
            return
    elif source.exists():
        raise RuntimeError('执行源码目录不是受控 Git 检出，拒绝覆盖')
    print('同步清单锁定的执行源码快照……', flush=True)
    environment = dict(os.environ, EXECUTOR_PULL_LATEST='0')
    subprocess.check_call(['node', str(root / 'executor-service/scripts/sync-flow-source.js')], env=environment)


def add_bytes(output, relative, data, mode=0o644):
    """统一 Linux 脚本换行，使用固定文件模式和合理时间，避免 Windows 打包差异。"""
    if relative.endswith('.sh'):
        data = data.replace(b'\r\n', b'\n')
        mode = 0o755
    item = tarfile.TarInfo(relative)
    item.size, item.mode, item.mtime = len(data), mode, int(time.time())
    output.addfile(item, io.BytesIO(data))


def package(root, destination):
    """从跟踪文件与清单 Git 对象构建包，绝不复制执行源码的脏工作树。"""
    manifest = json.loads((root / 'executor-service/flow-source-manifest.json').read_text(encoding='utf-8'))
    source = root / 'executor-service/flow-source'
    if git(source, 'rev-parse', 'HEAD').decode().strip() != manifest['head']:
        raise RuntimeError('执行源码不在清单版本，请先运行 pnpm --dir executor-service sync-source')
    if git(source, 'remote', 'get-url', 'origin').decode().strip() != manifest['remote']:
        raise RuntimeError('执行源码来源与清单不一致')
    for line in git(source, 'status', '--porcelain', '--untracked-files=no').decode().splitlines():
        if not (line.startswith(' D ') and line[3:].split('/')[0] in NOISE):
            raise RuntimeError('执行源码有未批准修改，拒绝发布脏工作树')
    paths = set(git(root, 'ls-files', '-z').decode('utf-8').split('\0'))
    # 首次编写入口时也可验证完整发布；后续这些入口均已纳入 Git。
    paths.update({'deploy/update.py', 'deploy/update.cmd', 'deploy/update-server.sh'})
    upstream = git(source, 'archive', manifest['head'])
    lib_prefix = 'form-runtime/runtime-source/src/lib/vue-form-making/dist/'
    if not (root / (lib_prefix + 'FormMaking.common.js')).is_file():
        raise RuntimeError('缺少受控 FormMaking 产物，拒绝发布')
    with tarfile.open(destination, 'w:gz') as output:
        for relative in sorted(paths):
            if relative and allowed(relative) and (root / relative).is_file():
                if (root / relative).is_symlink():
                    raise RuntimeError('发布文件不能是符号链接：' + relative)
                add_bytes(output, relative, (root / relative).read_bytes())
        with tarfile.open(fileobj=io.BytesIO(upstream)) as archive:
            for item in archive:
                if item.isfile() and clean_upstream(item.name):
                    add_bytes(output, 'executor-service/flow-source/' + item.name, archive.extractfile(item).read(), item.mode)
        for relative in sorted(paths):
            if relative.startswith(lib_prefix) and (root / relative).is_file():
                add_bytes(output, 'executor-service/flow-source/src/lib/vue-form-making/dist/' + relative[len(lib_prefix):], (root / relative).read_bytes())
        revision = git(root, 'rev-parse', 'HEAD').decode().strip()
        data = {'revision': revision, 'flowSource': manifest['head'], 'trackedWorktree': True}
        add_bytes(output, 'deploy/release-source.json', json.dumps(data, ensure_ascii=False).encode('utf-8'))
    return hashlib.sha256(destination.read_bytes()).hexdigest()


def forward_proxy(client, port):
    """临时把服务器回环端口转发到本机代理；连接结束后隧道自动消失。"""
    with socket.create_connection(('127.0.0.1', port), timeout=2):
        pass
    transport = client.get_transport()
    remote_port = transport.request_port_forward('127.0.0.1', 0)

    def bridge(channel):
        connection = None
        try:
            connection = socket.create_connection(('127.0.0.1', port), timeout=15)
            connection.settimeout(None)
            while transport.is_active():
                ready, _, _ = select.select([channel, connection], [], [], 1)
                for endpoint in ready:
                    data = endpoint.recv(65536)
                    if not data:
                        return
                    (connection if endpoint is channel else channel).sendall(data)
        finally:
            channel.close()
            if connection:
                connection.close()

    def accept():
        while transport.is_active():
            channel = transport.accept(1)
            if channel:
                threading.Thread(target=bridge, args=(channel,), daemon=True).start()

    threading.Thread(target=accept, daemon=True).start()
    return remote_port


def remote_run(client, command, log):
    """实时输出并保存服务器日志，以实际 SSH 退出码判断成功。"""
    channel = client.get_transport().open_session()
    channel.set_combine_stderr(True)
    channel.exec_command(command)
    with channel:
        while True:
            if channel.recv_ready():
                data = channel.recv(65536)
                log.write(data)
                log.flush()
                emit(data.decode('utf-8', errors='replace'))
            elif channel.exit_status_ready():
                return channel.recv_exit_status()
            else:
                time.sleep(0.1)


def main():
    """本机入口读取被忽略的凭证配置，上传后执行同一套服务器发布流程。"""
    configure_output()
    parser = argparse.ArgumentParser(description='一键更新 192.168.2.200 当前应用')
    parser.add_argument('--config', type=pathlib.Path, default=ROOT / 'deploy/.env.update.json')
    parser.add_argument('--package-only', type=pathlib.Path, help='仅生成发布包，不连接服务器')
    args = parser.parse_args()
    if args.package_only:
        print('发布包 SHA256：' + package(ROOT, args.package_only))
        return
    import paramiko
    config = json.loads(args.config.read_text(encoding='utf-8-sig'))
    release = time.strftime('%Y%m%d-%H%M%S') + '-' + uuid.uuid4().hex[:8]
    log_dir = ROOT / '.runtime/deploy'
    log_dir.mkdir(parents=True, exist_ok=True)
    log_path = log_dir / (release + '.log')
    remote_root = config.get('directory', '/opt/test-auto-pro-v2')
    remote_temp = '/opt/.test-auto-pro-v2-update-' + release
    client = paramiko.SSHClient()
    known = pathlib.Path.home() / '.ssh/known_hosts'
    known.parent.mkdir(parents=True, exist_ok=True)
    if known.exists():
        client.load_host_keys(str(known))
    # 首次记录指定内网服务器公钥，之后变更会被 Paramiko 拒绝。
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    print('本机日志：' + str(log_path), flush=True)
    try:
        with tempfile.TemporaryDirectory(prefix='app-release-') as temporary:
            archive = pathlib.Path(temporary) / 'source.tar.gz'
            prepare_source(ROOT)
            digest = package(ROOT, archive)
            print(f'发布包已生成：{archive.stat().st_size // 1024} KiB，连接服务器……', flush=True)
            client.connect(config.get('host', '192.168.2.200'), username=config.get('user', 'root'),
                           password=config['password'], timeout=15, look_for_keys=False, allow_agent=False)
            client.save_host_keys(str(known))
            client.get_transport().set_keepalive(15)
            proxy_port = 0
            if config.get('proxy_port', 7897):
                try:
                    proxy_port = forward_proxy(client, int(config.get('proxy_port', 7897)))
                    print('临时下载代理已连接', flush=True)
                except OSError:
                    print('本机代理未启动，使用服务器直连下载', flush=True)
            with client.open_sftp() as sftp:
                sftp.mkdir(remote_temp, mode=0o700)
                sftp.put(str(archive), remote_temp + '/source.tar.gz')
                with sftp.open(remote_temp + '/update-server.sh', 'w') as script:
                    script.write((ROOT / 'deploy/update-server.sh').read_text(encoding='utf-8'))
            command = 'bash ' + shlex.quote(remote_temp + '/update-server.sh')
            command += ' ' + ' '.join(map(shlex.quote, [remote_root, remote_temp, digest, str(proxy_port)]))
            with log_path.open('wb') as log:
                status = remote_run(client, command, log)
            if status:
                raise RuntimeError(f'服务器发布失败（退出码 {status}），日志：{log_path}')
            print('更新发布成功：http://' + config.get('host', '192.168.2.200') + ':19000', flush=True)
    finally:
        client.close()


if __name__ == '__main__':
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        print(f'更新发布失败：{error}', flush=True)
        raise SystemExit(1)
