#!/usr/bin/env python3
# 使用临时仓库验证更名，不修改当前项目或依赖本机 Go 环境。
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('rename-project.sh').resolve()


class RenameProjectTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='rename project ')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / 'scripts').mkdir()
        shutil.copy2(SCRIPT, self.root / 'scripts/rename-project.sh')
        self.write('project.yaml', 'module_prefix: github.com/templatekit\nenv_prefix: TEMPLATEKIT\nmetric_prefix: templatekit\nimage_prefix: templatekit\ndisplay_name: TemplateKit\n')
        self.write('sample.txt', 'github.com/templatekit/platform TEMPLATEKIT_PORT templatekit_requests templatekit.user templatekit/user TemplateKit templatekit\n')
        self.write('charts/templatekit/nested/templatekit file\nname.yaml', 'templatekit/user\n')
        self.write('user-service/internal/shared/rbacbaseline/ids.go', 'TemplateKit immutable UUID\n')
        self.write('.github/workflows/ci.yml', 'name: TemplateKit\n')
        self.write('renovate.json', '{}\n')
        self.write('coverage/platform.txt', 'coverage report\n')
        self.write('.gitignore', 'cache/\n')
        self.write('cache/templatekit.txt', 'templatekit\n')
        self.git('init', '-q')
        self.git('config', 'user.name', 'Rename Test')
        self.git('config', 'user.email', 'rename@example.invalid')
        self.commit()

    def write(self, name, content):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        return path

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.root), *args], text=True)

    def commit(self):
        self.git('add', '.')
        self.git('commit', '-qm', 'fixture')

    def run_script(self, *args, cwd=None, env=None):
        return subprocess.run(['/bin/bash', str(self.root / 'scripts/rename-project.sh'), *args],
                              cwd=cwd or self.root, env=env, text=True,
                              stdout=subprocess.PIPE, stderr=subprocess.STDOUT)

    def assert_clean(self):
        self.assertEqual(self.git('status', '--porcelain'), '')
        self.assertEqual((self.root / '.github/workflows/ci.yml').read_text(), 'name: TemplateKit\n')
        self.assertEqual((self.root / 'renovate.json').read_text(), '{}\n')
        self.assertEqual((self.root / 'cache/templatekit.txt').read_text(), 'templatekit\n')
        self.assertEqual((self.root / 'coverage/platform.txt').read_text(), 'coverage report\n')

    def test_single_pass_from_subdirectory_and_override(self):
        result = self.run_script('--apply', '--skip-generate', '--module',
                                 'github.com/example/templatekit-app', '--display-name', 'TemplateKitApp',
                                 cwd=self.root / 'scripts')
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertFalse((self.root / 'project.yaml').exists())
        self.assertEqual((self.root / 'sample.txt').read_text(),
                         'github.com/example/templatekit-app/platform TEMPLATEKIT_APP_PORT templatekit_app_requests templatekit_app.user templatekit-app/user TemplateKitApp templatekit_app\n')
        self.assertTrue((self.root / 'charts/templatekit-app/nested/templatekit-app file\nname.yaml').is_file())
        self.assertEqual((self.root / 'cache/templatekit.txt').read_text(), 'templatekit\n')
        self.assertEqual((self.root / 'user-service/internal/shared/rbacbaseline/ids.go').read_text(), 'TemplateKit immutable UUID\n')

    def test_success_removes_template_metadata_after_generation(self):
        with tempfile.TemporaryDirectory() as mock_dir:
            go = Path(mock_dir) / 'go'
            go.write_text('#!/bin/sh\ntest -d .git && test -f .github/workflows/ci.yml && test -f renovate.json || exit 42\necho go-test\n')
            make = Path(mock_dir) / 'make'
            shutil.copy2(go, make)
            go.chmod(0o755)
            make.chmod(0o755)
            env = dict(os.environ, PATH=mock_dir + os.pathsep + os.environ['PATH'])
            result = self.run_script('--apply', '--module', 'github.com/example/new-app', env=env)
        self.assertEqual(result.returncode, 0, result.stdout)
        for name in ['project.yaml', '.github', '.git', 'renovate.json', 'coverage']:
            self.assertFalse((self.root / name).exists(), name)

    def test_missing_optional_metadata_does_not_block_cleanup(self):
        shutil.rmtree(self.root / '.github')
        (self.root / 'renovate.json').unlink()
        shutil.rmtree(self.root / 'coverage')
        self.commit()
        result = self.run_script('--apply', '--skip-generate', '--module', 'github.com/example/new-app')
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertFalse((self.root / '.git').exists())

    def test_explicitly_preserved_module_and_binary(self):
        binary = self.root / 'binary.dat'
        binary.write_bytes(b'\x00templatekit\xff')
        external = Path(self.temp.name).parent / (self.root.name + '-outside')
        external.write_text('templatekit')
        self.addCleanup(external.unlink)
        (self.root / 'link.txt').symlink_to(external)
        self.commit()
        result = self.run_script('--apply', '--skip-generate', '--module',
                                 'github.com/templatekit', '--name', 'new-app')
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertTrue((self.root / 'sample.txt').read_text().startswith('github.com/templatekit/platform '))
        self.assertEqual(binary.read_bytes(), b'\x00templatekit\xff')
        self.assertEqual(external.read_text(), 'templatekit')

    def test_dry_run_and_invalid_arguments_never_write(self):
        for args, success in [
            (['--module', 'github.com/example/new-app', '--skip-go-check'], True),
            (['--module'], False),
            (['--module', '--apply'], False),
            (['--dry-run', '--apply'], False),
            (['--apply', '--dry-run'], False),
        ]:
            with self.subTest(args=args):
                result = self.run_script(*args)
                self.assertEqual(result.returncode == 0, success, result.stdout)
                self.assert_clean()
        for flag in ['--drop-git', '--drop-github', '--reset-git']:
            removed = self.run_script('--apply', '--skip-generate', '--module', 'github.com/example/new-app', flag)
            self.assertEqual(removed.returncode, 2, removed.stdout)
            self.assertIn('未知命令行参数: ' + flag, removed.stdout)
            self.assert_clean()
        preview = self.run_script('--module', 'github.com/example/new-app', '--skip-go-check')
        self.assertIn('将删除 project.yaml、.github、.git、renovate.json 和 coverage/', preview.stdout)

    def test_conflict_rolls_back_explicit_exit(self):
        self.write('charts/new-app/keep.txt', 'keep\n')
        self.commit()
        result = self.run_script('--apply', '--skip-generate', '--module', 'github.com/example/new-app')
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn('目标已存在', result.stdout)
        self.assert_clean()
        self.assertTrue((self.root / 'charts/templatekit/nested/templatekit file\nname.yaml').is_file())

    def test_generation_failure_and_signal_roll_back(self):
        for failure in ['exit 42', 'kill -TERM "$PPID"; exit 143']:
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as mock_dir:
                go = Path(mock_dir) / 'go'
                go.write_text('#!/bin/sh\nif [ "$1" = version ]; then echo go-test; exit 0; fi\nprintf generated > generated.tmp\n' + failure + '\n')
                go.chmod(0o755)
                env = dict(os.environ, PATH=mock_dir + os.pathsep + os.environ['PATH'])
                result = self.run_script('--apply', '--module', 'github.com/example/new-app', env=env)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assert_clean()
                self.assertFalse((self.root / 'generated.tmp').exists())

    def test_linked_worktree_rejects_git_removal(self):
        linked = self.root / 'linked'
        self.git('worktree', 'add', '--detach', str(linked))
        result = subprocess.run(['/bin/bash', str(linked / 'scripts/rename-project.sh'), '--apply',
                                 '--skip-generate', '--module', 'github.com/example/new-app'],
                                text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn('linked worktree', result.stdout)
        self.assertTrue((linked / '.git').is_file())


if __name__ == '__main__':
    unittest.main()
