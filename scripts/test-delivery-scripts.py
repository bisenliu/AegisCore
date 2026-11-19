#!/usr/bin/env python3
# 在临时目录验证无 Git 的派生项目，以及保留 Git 后的差异门禁。
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class DeliveryScriptsTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='delivery scripts ')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        for name in ['Makefile', 'common/Makefile', 'user-service/Makefile',
                     'scripts/check-github-actions-pins.sh']:
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT / name, target)

    def run_command(self, *args, cwd=None):
        return subprocess.run(args, cwd=cwd or self.root, text=True,
                              stdout=subprocess.PIPE, stderr=subprocess.STDOUT)

    def test_actions_without_git_and_from_subdirectory(self):
        script = str(self.root / 'scripts/check-github-actions-pins.sh')
        result = self.run_command('bash', script, cwd=self.root / 'common')
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn('skipped', result.stdout)
        workflow = self.root / '.github/workflows/ci.yml'
        workflow.parent.mkdir(parents=True)
        for ref, valid in [('main', False), ('a' * 40, True), ('', False)]:
            workflow.write_text('steps:\n  - uses: actions/checkout' + ('@' + ref if ref else '') + '\n')
            result = self.run_command('bash', script, cwd=self.root / 'common')
            self.assertEqual(result.returncode == 0, valid, result.stdout)

    def test_make_git_checks_skip_without_git_and_reject_drift_with_git(self):
        checks = [
            ('.', 'verify', ['lint', 'user-service-architecture-lint', 'github-actions-pins-check',
                             'delivery-verify', 'common-generate', 'user-service-generate',
                             'test', 'user-service-openapi-generate']),
            ('common', 'verify', ['lint', 'generate', 'test']),
            ('user-service', 'verify', ['lint', 'architecture-lint', 'generate', 'test', 'openapi-generate']),
            ('user-service', 'fxgraph-check', ['fxgraph-generate']),
        ]

        def check(expected_success):
            for directory, target, dependencies in checks:
                with self.subTest(directory=directory, target=target):
                    args = ['make', '-C', directory]
                    for dependency in dependencies:
                        args.extend(['-o', dependency])
                    result = self.run_command(*args, target)
                    self.assertEqual(result.returncode == 0, expected_success, result.stdout)
                    if expected_success:
                        self.assertIn('skipped', result.stdout)

        check(True)
        for args in [('init', '-q'), ('config', 'user.name', 'Test'),
                     ('config', 'user.email', 'test@example.invalid'),
                     ('add', '.'), ('commit', '-qm', 'fixture')]:
            result = self.run_command('git', *args)
            self.assertEqual(result.returncode, 0, result.stdout)
        # 所有检查都应命中各自范围内的差异，包括 Fx 图生成物。
        for name in ['common/Makefile', 'user-service/Makefile', 'user-service/docs/fx-dependency-graph.dot']:
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.touch()
        self.run_command('git', 'add', '.')
        self.run_command('git', 'commit', '-qm', 'graph fixture')
        for name in ['common/Makefile', 'user-service/Makefile', 'user-service/docs/fx-dependency-graph.dot']:
            with (self.root / name).open('a') as output:
                output.write('\n# drift\n')
        check(False)


if __name__ == '__main__':
    unittest.main()
