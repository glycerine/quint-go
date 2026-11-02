import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from adapt import adapt
from generate import ROOT, SHA256, fix_go_parser, verify_jar


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

    def test_parser_dummy_label_precedes_return(self):
        tail = '\treturn localctx\n\tgoto errorExit // Trick to prevent compiler error if the label is not used\n'
        expected = '\tif false {\n\t\tgoto errorExit // Keep the generated error label in use.\n\t}\n\treturn localctx\n'
        source = 'errorExit:\n\tp.ExitRule()\n' + tail
        self.assertEqual(fix_go_parser(source), 'errorExit:\n\tp.ExitRule()\n' + expected)
        self.assertEqual(fix_go_parser(source + source), ('errorExit:\n\tp.ExitRule()\n' + expected) * 2)
        self.assertEqual(fix_go_parser(source.replace('\n', '\r\n')), 'errorExit:\r\n\tp.ExitRule()\r\n' + expected)

    def test_changed_parser_tail_fails_loudly(self):
        tail = '\treturn localctx\n\tgoto errorExit // Trick to prevent compiler error if the label is not used\n'
        for source in ['', tail.replace('return localctx', 'return context'),
                       tail + tail.replace('return localctx', 'return context'),
                       tail + tail.replace('Trick to prevent', 'Changed comment to prevent')]:
            with self.subTest(source=source), self.assertRaisesRegex(ValueError, 'parser rule tail changed'):
                fix_go_parser(source)


if __name__ == '__main__':
    unittest.main()
