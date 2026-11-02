import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from adapt import adapt
from generate import ROOT, SHA256, verify_jar


class GenerationTests(unittest.TestCase):
    def test_adaptation_matches_committed_grammar(self):
        source = (ROOT / 'quint/src/generated/Quint.g4').read_text()
        self.assertEqual(adapt(source), (ROOT / 'pkg/grammar/Quint.g4').read_text())

    def test_changed_actions_fail_loudly(self):
        source = (ROOT / 'quint/src/generated/Quint.g4').read_text()
        for fragment in ['@header', 'wrongTypeApplication :', "expr '=' expr", '| reserved {']:
            with self.subTest(fragment=fragment), self.assertRaises(AssertionError):
                adapt(source.replace(fragment, 'changed-upstream-action'))

    def test_corrupt_jar_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            jar = Path(directory) / 'tool.jar'
            jar.write_bytes(b'not the generator')
            with self.assertRaisesRegex(ValueError, SHA256):
                verify_jar(jar)

    def test_matching_hash_is_accepted(self):
        with patch('generate.hashlib.sha256') as digest:
            digest.return_value.hexdigest.return_value = SHA256
            with patch.object(Path, 'read_bytes', return_value=b'jar'):
                verify_jar(Path('tool.jar'))


if __name__ == '__main__':
    unittest.main()
