"""Fast isolated tests for native verification orchestration; no Go builds."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().with_name('verify-go.sh')
REVISION = '4e8d22641447b92cb4f8e404023336fbf8d4b08d'


class VerifyGoTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='quint-verify-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.checker = self.root / 'checker with spaces'
        (self.checker / 'pkg').mkdir(parents=True)
        (self.checker / 'pkg/go.mod').write_text('module checker\ngo 1.27.3\n')
        self.output = self.root / 'output with spaces'
        self.log = self.root / 'commands'
        commands = self.root / 'commands-bin'
        commands.mkdir()
        self.write_command(commands / 'git', '''#!/bin/sh
case "$3" in
  rev-parse)
    [ "${FAKE_GIT_FAIL-}" != rev-parse ] || exit 9
    printf '%s\\n' "$FAKE_CHECKER_REVISION"
    ;;
  status)
    [ "${FAKE_GIT_FAIL-}" != status ] || exit 9
    printf '%s' "${FAKE_CHECKER_STATUS-}"
    ;;
  *) exit 8 ;;
esac
''')
        self.write_command(commands / 'go', '''#!/bin/sh
printf '%s|%s|%s|%s|%s\\n' "$GOWORK" "$PWD" "${GOFLAGS-}" "${CGO_ENABLED-}" "$*" >> "$FAKE_LOG"
[ "${FAKE_GO_FAIL-}" != "$1" ] || exit 9
case "$1" in
  env) printf '%s\\n' "${FAKE_GO_VERSION-go1.27.3}" ;;
  work) printf '%s\\n' 'go 1.27.3' > "$GOWORK" ;;
  build)
    while [ "$#" -gt 0 ]; do
      if [ "$1" = -o ]; then printf '%s\\n' 'native artifact' > "$2"; break; fi
      shift
    done
    ;;
  test|vet) ;;
  *) exit 8 ;;
esac
''')
        self.env = dict(os.environ,
                        PATH=str(commands) + os.pathsep + os.environ['PATH'],
                        FAKE_LOG=str(self.log), FAKE_CHECKER_REVISION=REVISION,
                        TMPDIR=str(self.root), GOWORK='/ambient/invalid/go.work',
                        GOFLAGS='-race', CGO_ENABLED='1')
        for name in ['FAKE_GIT_FAIL', 'FAKE_CHECKER_STATUS', 'FAKE_GO_FAIL', 'FAKE_GO_VERSION']:
            self.env.pop(name, None)

    @staticmethod
    def write_command(path, text):
        path.write_text(text)
        path.chmod(0o755)

    def invoke(self, *args, **env):
        return subprocess.run(['bash', str(SCRIPT), *map(str, args)],
                              text=True, capture_output=True,
                              env=dict(self.env, **env), timeout=5)

    def standard(self, *extra, **env):
        return self.invoke('--checker', self.checker, '--output', self.output, *extra, **env)

    def calls(self):
        if not self.log.exists():
            return []
        return [line.split('|', 4) for line in self.log.read_text().splitlines()]

    def assert_stage_removed(self):
        for call in self.calls():
            if call[0] != 'off':
                self.assertFalse(Path(call[0]).parent.exists(), call[0])

    def test_help_does_not_run_tools(self):
        result = self.invoke('--help')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('Usage:', result.stdout)
        self.assertEqual(self.calls(), [])

    def test_invalid_arguments(self):
        for args in [[], ['--unknown'], ['--checker'], ['--checker', '--output'],
                     ['--checker', self.checker, '--checker-revision', 'main'],
                     ['--checker', self.checker, '--checker-revision', REVISION.upper()]]:
            with self.subTest(args=args):
                self.assertEqual(self.invoke(*args).returncode, 2)
        self.assertEqual(self.calls(), [])

    def test_checker_module_required(self):
        result = self.invoke('--checker', self.root / 'missing')
        self.assertEqual(result.returncode, 1)
        self.assertIn('pkg/go.mod', result.stderr)
        self.assertEqual(self.calls(), [])

    def test_wrong_revision_rejected(self):
        result = self.standard(FAKE_CHECKER_REVISION='a' * 40)
        self.assertEqual(result.returncode, 1)
        self.assertIn(REVISION, result.stderr)
        self.assertEqual(self.calls(), [])

    def test_dirty_checker_rejected(self):
        result = self.standard(FAKE_CHECKER_STATUS='?? local.go\n')
        self.assertEqual(result.returncode, 1)
        self.assertIn('clean', result.stderr)
        self.assertEqual(self.calls(), [])

    def test_git_failures_rejected(self):
        for command in ['rev-parse', 'status']:
            with self.subTest(command=command):
                result = self.standard(FAKE_GIT_FAIL=command)
                self.assertEqual(result.returncode, 1)
        self.assertEqual(self.calls(), [])

    def test_toolchain_required_before_build(self):
        result = self.standard(FAKE_GO_VERSION='go1.26.0')
        self.assertEqual(result.returncode, 1)
        self.assertIn('Go 1.27.3', result.stderr)
        self.assertEqual([call[4] for call in self.calls()], ['env GOVERSION'])

    def test_native_commands_use_isolated_workspace(self):
        result = self.standard()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        self.assertEqual([call[4].split()[0] for call in calls],
                         ['env', 'work', 'build', 'build', 'test', 'vet'])
        self.assertEqual(calls[0][0], 'off')
        self.assertIn(str(self.checker / 'pkg'), calls[1][4])
        for call in calls[1:]:
            self.assertEqual(call[0], calls[1][0])
            self.assertEqual(call[2:4], ['-mod=readonly', '0'])
        for call in calls[2:]:
            self.assertEqual(call[1], str(SCRIPT.parents[1]))
        self.assertIn('-trimpath -buildvcs=false', calls[2][4])
        self.assertEqual(calls[-2][4], 'test ./... -count=1')
        self.assertEqual(calls[-1][4], 'vet ./...')
        self.assertTrue((self.output / 'quint').is_file())
        self.assertTrue((self.output / 'quint-lsp').is_file())
        self.assert_stage_removed()

    def test_explicit_revision(self):
        revision = 'a' * 40
        result = self.standard('--checker-revision', revision, FAKE_CHECKER_REVISION=revision)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(revision, result.stdout)
        self.assert_stage_removed()

    def test_failure_stops_following_commands_and_cleans_workspace(self):
        for command in ['work', 'build', 'test', 'vet']:
            with self.subTest(command=command):
                self.log.unlink(missing_ok=True)
                result = self.standard(FAKE_GO_FAIL=command)
                self.assertEqual(result.returncode, 9, result.stderr)
                self.assertEqual(self.calls()[-1][4].split()[0], command)
                self.assert_stage_removed()

    def test_temporary_artifacts_removed_without_output_option(self):
        result = self.invoke('--checker', self.checker)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_stage_removed()
        self.assertEqual(list(self.root.glob('quint-go-verify.*')), [])


if __name__ == '__main__':
    unittest.main()
