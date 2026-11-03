"""Fast smoke-driver checks; live native binaries are checked separately."""
import argparse
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch


SPEC = importlib.util.spec_from_file_location('native_smoke', Path(__file__).with_name('smoke-go.py'))
SMOKE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SMOKE)


class NativeSmokeTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='quint-smoke-driver-test-')
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name).resolve()

    def test_unicode_and_adjacent_lsp_frames(self):
        messages = [{'id': 1, 'result': 'λ😀'}, {'method': 'notification', 'params': {}}]
        self.assertEqual(SMOKE.unframe(SMOKE.frame(messages)), messages)
        self.assertEqual(SMOKE.unframe(b''), [])
        self.assertEqual(SMOKE.frame([]), b'')
        self.assertIn(b'Content-Length: 26\r\n', SMOKE.frame([messages[0]]))

    def test_malformed_lsp_frames(self):
        for data in [b'Content-Length: 2\r\n', b'bad\r\n\r\n{}',
                     b'Content-Length: nope\r\n\r\n{}',
                     b'Content-Length: -1\r\n\r\n{}',
                     b'Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}',
                     b'Content-Type: json\r\n\r\n{}',
                     b'Content-Length: 3\r\n\r\n{}',
                     b'Content-Length: 2\r\n\r\n[]',
                     b'Content-Length: 2\r\n\r\nxx',
                     b'Content-Length: 1\r\n\r\n\xff']:
            with self.subTest(data=data), self.assertRaises(SMOKE.SmokeError):
                SMOKE.unframe(data)

    def test_argument_validation(self):
        for value in ('0', '-1', 'nan', 'inf'):
            with self.subTest(value=value), self.assertRaises(argparse.ArgumentTypeError):
                SMOKE.positive_seconds(value)
        self.assertEqual(SMOKE.positive_seconds('0.25'), .25)
        nonexecutable = self.directory / 'plain'
        nonexecutable.write_text('plain')
        for path in (nonexecutable, self.directory, self.directory / 'missing'):
            with self.subTest(path=path), self.assertRaises(argparse.ArgumentTypeError):
                SMOKE.executable(str(path))
        self.assertEqual(SMOKE.executable(sys.executable), Path(sys.executable).resolve())

    def test_process_status_environment_and_timeout(self):
        environment = {'PATH': '/nonexistent', 'HOME': str(self.directory)}
        result = SMOKE.invoke(sys.executable,
                              ['-c', 'import os,sys; print(os.getcwd()); print(os.environ["PATH"]); sys.exit(7)'],
                              self.directory, environment, 2, expected=7)
        self.assertEqual(result.stdout.decode().splitlines(), [str(self.directory), '/nonexistent'])
        with self.assertRaisesRegex(SMOKE.SmokeError, 'exit 7, expected 0'):
            SMOKE.invoke(sys.executable, ['-c', 'raise SystemExit(7)'], self.directory, environment, 2)
        with self.assertRaises(SMOKE.SmokeError):
            SMOKE.invoke(sys.executable, ['-c', 'import time; time.sleep(1)'],
                         self.directory, environment, .02)
        with self.assertRaises(SMOKE.SmokeError):
            SMOKE.invoke(self.directory / 'missing', [], self.directory, environment, 2)

    def test_stage_validation(self):
        file = self.directory / 'result.json'
        for text in ('', '[]', '{"errors":[{"message":"bad"}]}', '{"stage":"wrong"}'):
            with self.subTest(text=text):
                file.write_text(text)
                with self.assertRaises(SMOKE.SmokeError):
                    SMOKE.load_stage(self.directory, file.name, 'checking', 'ok')
        file.write_text('{"stage":"checking","status":"ok","errors":[]}')
        self.assertEqual(SMOKE.load_stage(self.directory, file.name, 'checking', 'ok')['status'], 'ok')
        file.unlink()
        with self.assertRaises(SMOKE.SmokeError):
            SMOKE.load_stage(self.directory, file.name)

    def fake_lsp(self, body):
        path = self.directory / 'fake-lsp'
        path.write_text('#!' + sys.executable + '\n' + body)
        path.chmod(0o755)
        return path

    def test_lsp_eof_and_timeout_are_bounded(self):
        model = self.directory / 'Smoke.qnt'
        model.write_text(SMOKE.MODEL)
        for body in ('raise SystemExit(0)\n', 'import time; time.sleep(1)\n'):
            with self.subTest(body=body), self.assertRaises(SMOKE.SmokeError):
                SMOKE.check_lsp(self.fake_lsp(body), self.directory, {'PATH': '/nonexistent'}, .05)

    def test_main_reports_success_and_failures(self):
        arguments = ['--quint', sys.executable, '--lsp', sys.executable]
        output = io.StringIO()
        with patch.object(SMOKE, 'smoke', return_value={'checks': ['native']}) as run:
            with contextlib.redirect_stdout(output):
                self.assertEqual(SMOKE.main(arguments), 0)
            self.assertEqual(json.loads(output.getvalue()), {'checks': ['native']})
            run.assert_called_once_with(Path(sys.executable).resolve(), Path(sys.executable).resolve(), 30)
        errors = io.StringIO()
        with patch.object(SMOKE, 'smoke', side_effect=SMOKE.SmokeError('test failure')):
            with contextlib.redirect_stderr(errors), self.assertRaises(SystemExit) as caught:
                SMOKE.main(arguments)
        self.assertEqual(caught.exception.code, 1)
        self.assertIn('native smoke: test failure', errors.getvalue())


if __name__ == '__main__':
    unittest.main()
