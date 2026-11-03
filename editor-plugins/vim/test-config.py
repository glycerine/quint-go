#!/usr/bin/env python3
"""Execute the README's Vim LSP configuration against an owned registration stub.

Requires Vim and Python only; no language-server or editor-plugin download.
Run: python3 editor-plugins/vim/test-config.py
"""

import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile


def quoted(value):
    return "'" + str(value).replace("'", "''") + "'"


def run():
    vim = shutil.which("vim")
    if vim is None:
        raise SystemExit("Vim is required for this optional editor configuration test")
    readme = Path(__file__).with_name("README.md").read_text()
    blocks = re.findall(r"```vim-script\n(.*?)\n```", readme, re.DOTALL)
    config = next(block for block in blocks if "lsp#register_server" in block)
    with tempfile.TemporaryDirectory(prefix="quint-vim-config-") as directory:
        root = Path(directory)
        (root / "autoload").mkdir()
        (root / "autoload/lsp.vim").write_text(
            "function! lsp#register_server(info) abort\n"
            "  let g:registered = copy(a:info)\nendfunction\n"
        )
        binary_dir = root / "bin"
        binary_dir.mkdir()
        for name in ("quint-lsp", "quint-language-server", "server with spaces"):
            binary = binary_dir / name
            binary.write_text("#!/bin/sh\nexit 0\n")
            binary.chmod(0o755)
        (root / "main.qnt").write_text("module M {}\n")
        env = dict(os.environ, PATH=str(binary_dir) + os.pathsep + os.environ.get("PATH", ""))
        scenarios = [
            ("default-go", None, ["quint-lsp"]),
            ("configured-go", [str(binary_dir / "server with spaces")], [str(binary_dir / "server with spaces")]),
            ("explicit-typescript", ["quint-language-server", "--stdio"], ["quint-language-server", "--stdio"]),
            ("missing-go", [str(root / "missing-go")], None),
        ]
        for name, command, expected in scenarios:
            script = "set nocompatible\nset runtimepath^=" + str(root) + "\n"
            if command is not None:
                script += "let g:quint_lsp_command = [" + ",".join(map(quoted, command)) + "]\n"
            script += config + "\ndoautocmd User lsp_setup\n"
            if expected is None:
                script += "call assert_false(exists('g:registered'))\n"
            else:
                script += "call assert_equal(['quint'], g:registered.allowlist)\n"
                script += "call assert_equal([" + ",".join(map(quoted, expected)) + "], call(g:registered.cmd, [{}]))\n"
            script += "execute 'edit ' . fnameescape(" + quoted(root / "main.qnt") + ")\n"
            script += "call assert_equal('quint', &filetype)\n"
            script += "doautocmd User lsp_buffer_enabled\ncall assert_equal('lsp#complete', &omnifunc)\n"
            script += "if !empty(v:errors)\ncall writefile(v:errors," + quoted(root / "errors") + ")\ncquit\nendif\nqa!\n"
            script_path = root / "test.vim"
            script_path.write_text(script)
            result = subprocess.run([vim, "-Nu", "NONE", "-n", "-es", "-S", str(script_path)], env=env, capture_output=True, text=True)
            if result.returncode != 0:
                errors = (root / "errors").read_text() if (root / "errors").exists() else result.stderr
                raise SystemExit(f"{name}: Vim configuration failed\n{errors}")
        print(f"Passed {len(scenarios)} actual Vim configuration scenarios")


if __name__ == "__main__":
    run()
