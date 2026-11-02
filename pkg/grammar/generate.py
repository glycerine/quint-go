#!/usr/bin/env python3
"""Regenerate both Go parsers offline with a verified ANTLR tool jar."""
import argparse
import hashlib
from pathlib import Path
import re
import subprocess
import tempfile

from adapt import adapt

VERSION = '4.13.2'
SHA256 = 'eae2dfa119a64327444672aff63e9ec35a20180dc5b8090b7a6ab85125df4d76'
ROOT = Path(__file__).resolve().parents[2]


def verify_jar(path):
    if hashlib.sha256(path.read_bytes()).hexdigest() != SHA256:
        raise ValueError(f'ANTLR jar must be version {VERSION}, SHA-256 {SHA256}')


def fix_go_parser(source):
    """Keep ANTLR's dummy label use reachable without executing the jump.

    ANTLR 4.13.2 appends a goto after each rule's return to suppress Go's
    unused-label error. go vet correctly rejects that unreachable statement.
    A constant-false branch before the return serves the same compiler purpose
    and does not change execution. Fail on changed generator templates instead
    of silently publishing a partially transformed parser.
    """
    marker = 'goto errorExit // Trick to prevent compiler error if the label is not used'
    tail = re.compile(r'(?m)^([ \t]*)return localctx\r?\n[ \t]*' + re.escape(marker) + r'\r?\n')
    def replace(match):
        indent = match[1]
        return (indent + 'if false {\n' + indent + '\tgoto errorExit // Keep the generated error label in use.\n'
                + indent + '}\n' + indent + 'return localctx\n')
    result, count = tail.subn(replace, source)
    if count == 0 or count != source.count(marker) or re.search(r'return localctx\s+goto errorExit', result):
        raise ValueError('ANTLR Go parser rule tail changed; update fix_go_parser')
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--jar', type=Path, required=True)
    parser.add_argument('--check', action='store_true', help='compare without changing files')
    args = parser.parse_args()
    jar = args.jar.resolve()
    verify_jar(jar)
    grammar = adapt((ROOT / 'quint/src/generated/Quint.g4').read_text())
    with tempfile.TemporaryDirectory(prefix='quint-parser-') as directory:
        stage = Path(directory)
        inputs = [('pkg/grammar/Quint.g4', 'quintparser'),
                  ('quint/src/generated/Effect.g4', 'effectparser')]
        outputs = {'pkg/grammar/Quint.g4': grammar.encode()}
        for relative, package in inputs:
            source = stage / relative
            source.parent.mkdir(parents=True, exist_ok=True)
            source.write_bytes(grammar.encode() if package == 'quintparser' else (ROOT / relative).read_bytes())
            target = f'pkg/internal/{package}'
            subprocess.run(['java', '-jar', str(jar), '-Dlanguage=Go', '-package', package,
                            '-visitor', '-Xexact-output-dir', '-o', target, relative], cwd=stage, check=True)
            go_files = sorted((stage / target).glob('*.go'))
            for path in go_files:
                if path.name.endswith('_parser.go'):
                    path.write_text(fix_go_parser(path.read_text()))
            subprocess.run(['gofmt', '-w', *map(str, go_files)], check=True)
            for path in sorted((stage / target).iterdir()):
                outputs[str(path.relative_to(stage))] = path.read_bytes()
        changed = [name for name, data in outputs.items()
                   if not (ROOT / name).exists() or (ROOT / name).read_bytes() != data]
        if args.check:
            if changed:
                parser.exit(1, 'Generated files differ:\n' + '\n'.join(changed) + '\n')
            print('Both generated parsers match the committed files.')
        else:
            for name in changed:
                (ROOT / name).write_bytes(outputs[name])
            print(f'Updated {len(changed)} generated files.')


if __name__ == '__main__':
    main()
