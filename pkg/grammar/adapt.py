#!/usr/bin/env python3
"""Adapt only target-language actions; syntax remains sourced from upstream."""
from pathlib import Path
import re
root = Path(__file__).resolve().parents[2]
s = (root / 'quint/src/generated/Quint.g4').read_text()
s, n = re.subn(r'@header \{.*?\n\}', '@parser::header {\nimport "strings"\n}', s, count=1, flags=re.S)
assert n == 1
s = s.replace('simpleId[context: string]', 'simpleId[string context]')
patterns = [
 (r"(wrongTypeApplication :.*?\{).*?(\};)", '''\\1
 p.NotifyErrorListeners("[QNT009] Use square brackets instead of parenthesis for type application: " + $qualId.text + "[" + strings.ReplaceAll($typeArgs.text, ",", ", ") + "]", nil, nil)
 \\2'''),
 (r"(expr '=' expr \{).*?(\}\s*# errorEq)", '''\\1
 p.NotifyErrorListeners("[QNT006] unexpected '=', did you mean '=='?", nil, nil)
 \\2'''),
 (r'(simpleId\[string context\].*?\| qualId \{).*?(\}\s*;)', '''\\1
 p.NotifyErrorListeners("[QNT008] Identifiers in a " + $context + " cannot be qualified with '::'. Found " + $qualId.text + ".", nil, nil)
 \\2'''),
 (r'(\| reserved \{).*?(\};)', '''\\1
 p.NotifyErrorListeners("[QNT008] Reserved keyword '" + $reserved.text + "' cannot be used as an identifier.", nil, nil)
 \\2'''),
]
for pattern, replacement in patterns:
 s, n = re.subn(pattern, replacement, s, count=1, flags=re.S)
 assert n == 1, pattern
assert 'this.notify' not in s and 'const err' not in s
(root / 'pkg/grammar/Quint.g4').write_text(s)
