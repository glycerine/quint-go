#!/usr/bin/env python3
"""Optional native command smoke; Python drives binaries with no tools on PATH."""
import argparse
import json
import os
from pathlib import Path
import queue
import shutil
import subprocess
import tempfile
import threading
import time


MODEL = """module Smoke {
  /// A finite counter used by the native packaging smoke check.
  var x: int
  action init = x' = 0
  action step = x' = if (x == 0) 1 else 0
  val safe = x >= 0 and x <= 1
  val broken = x == 0
  pure val builtin = List(1, 2).length()
  run smokeTest = init.then(step).expect(x == 1)
}
"""


class SmokeError(Exception):
    pass


def require(condition, message):
    if not condition:
        raise SmokeError(message)


def executable(value):
    path = Path(value).expanduser().resolve()
    if not path.is_file() or not os.access(path, os.X_OK):
        raise argparse.ArgumentTypeError(f"not an executable file: {path}")
    return path


def positive_seconds(value):
    seconds = float(value)
    if not 0 < seconds < float('inf'):
        raise argparse.ArgumentTypeError("timeout must be positive and finite")
    return seconds


def frame(messages):
    out = bytearray()
    for message in messages:
        body = json.dumps(message, separators=(',', ':'), ensure_ascii=False).encode('utf-8')
        out.extend(f"Content-Length: {len(body)}\r\n\r\n".encode('ascii'))
        out.extend(body)
    return bytes(out)


def unframe(data):
    messages = []
    while data:
        header, separator, body = data.partition(b'\r\n\r\n')
        require(bool(separator), "LSP response has an incomplete header")
        lengths = []
        for line in header.split(b'\r\n'):
            name, colon, value = line.partition(b':')
            require(bool(colon), "LSP response has a malformed header")
            if name.lower() == b'content-length':
                try:
                    lengths.append(int(value.strip()))
                except ValueError as error:
                    raise SmokeError("LSP response has an invalid length") from error
        require(len(lengths) == 1 and lengths[0] >= 0, "LSP response needs one nonnegative length")
        length = lengths[0]
        require(len(body) >= length, "LSP response has a truncated body")
        try:
            message = json.loads(body[:length])
        except (ValueError, UnicodeError) as error:
            raise SmokeError("LSP response is not UTF-8 JSON") from error
        require(isinstance(message, dict), "LSP response is not an object")
        messages.append(message)
        data = body[length:]
    return messages


def invoke(binary, arguments, cwd, environment, timeout, expected=0, stdin=b''):
    command = [str(binary), *arguments]
    try:
        result = subprocess.run(command, input=stdin, capture_output=True, cwd=cwd,
                                env=environment, timeout=timeout, check=False)
    except (OSError, subprocess.TimeoutExpired) as error:
        raise SmokeError(f"{' '.join(command)}: {error}") from error
    require(result.returncode == expected,
            f"{' '.join(arguments)}: exit {result.returncode}, expected {expected}\n"
            f"{result.stderr.decode('utf-8', errors='replace')[-2000:]}"
            f"{result.stdout.decode('utf-8', errors='replace')[-2000:]}")
    return result


def load_stage(directory, name, stage=None, status=None):
    try:
        result = json.loads((directory / name).read_text(encoding='utf-8'))
    except (OSError, ValueError) as error:
        raise SmokeError(f"invalid or missing {name}: {error}") from error
    require(isinstance(result, dict), f"{name}: expected a JSON object")
    require(not result.get('errors'), f"{name}: unexpected diagnostics")
    if stage is not None:
        require(result.get('stage') == stage, f"{name}: unexpected stage")
    if status is not None:
        require(result.get('status') == status, f"{name}: unexpected status")
    return result


def check_lsp(binary, directory, environment, timeout):
    lines = MODEL.splitlines()
    line = next(index for index, text in enumerate(lines) if '.length()' in text)
    character = lines[line].index('length') + 1
    uri = (directory / 'Smoke.qnt').as_uri()
    messages = [
        {'jsonrpc': '2.0', 'id': 1, 'method': 'initialize',
         'params': {'processId': None, 'rootUri': directory.as_uri(), 'capabilities': {}}},
        {'jsonrpc': '2.0', 'method': 'initialized', 'params': {}},
        {'jsonrpc': '2.0', 'method': 'textDocument/didOpen', 'params': {
            'textDocument': {'uri': uri, 'languageId': 'quint', 'version': 1, 'text': MODEL}}},
        {'jsonrpc': '2.0', 'id': 2, 'method': 'textDocument/hover', 'params': {
            'textDocument': {'uri': uri}, 'position': {'line': line, 'character': character}}},
        {'jsonrpc': '2.0', 'id': 3, 'method': 'textDocument/documentSymbol',
         'params': {'textDocument': {'uri': uri}}},
        {'jsonrpc': '2.0', 'id': 4, 'method': 'shutdown', 'params': None},
        {'jsonrpc': '2.0', 'method': 'exit'},
    ]
    process = subprocess.Popen([str(binary)], cwd=directory, env=environment,
                               stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    incoming = queue.Queue()

    def read_responses():
        try:
            while True:
                header = bytearray()
                while not header.endswith(b'\r\n\r\n'):
                    line_bytes = process.stdout.readline()
                    if not line_bytes:
                        incoming.put(SmokeError('LSP closed its response stream'))
                        return
                    header.extend(line_bytes)
                lengths = [line_bytes.partition(b':')[2].strip()
                           for line_bytes in bytes(header).split(b'\r\n')
                           if line_bytes.lower().startswith(b'content-length:')]
                require(len(lengths) == 1, 'LSP response needs one length')
                length = int(lengths[0])
                require(length >= 0, 'LSP response has a negative length')
                body = process.stdout.read(length)
                for response in unframe(bytes(header) + body):
                    incoming.put(response)
        except Exception as error:
            incoming.put(error)

    reader = threading.Thread(target=read_responses, daemon=True)
    reader.start()

    def send(*requests):
        process.stdin.write(frame(requests))
        process.stdin.flush()

    def receive(predicate):
        deadline = time.monotonic() + timeout
        while True:
            remaining = deadline - time.monotonic()
            require(remaining > 0, 'timed out waiting for an LSP response')
            try:
                response = incoming.get(timeout=remaining)
            except queue.Empty as error:
                raise SmokeError('timed out waiting for an LSP response') from error
            if isinstance(response, Exception):
                raise SmokeError(f'LSP response failed: {response}') from response
            require('error' not in response, f'LSP request failed: {response}')
            if predicate(response):
                return response

    responses = {}
    try:
        send(messages[0])
        responses[1] = receive(lambda response: response.get('id') == 1).get('result')
        send(*messages[1:3])
        # didOpen analysis is asynchronous. Wait for its real protocol event,
        # rather than relying on timing or shutting down while it is pending.
        diagnostic = receive(lambda response: response.get('method') == 'textDocument/publishDiagnostics'
                             and response.get('params', {}).get('uri') == uri)
        require(not diagnostic['params'].get('diagnostics'), 'LSP reported smoke model diagnostics')
        for message in messages[3:6]:
            send(message)
            request_id = message['id']
            responses[request_id] = receive(lambda response: response.get('id') == request_id).get('result')
        send(messages[6])
        process.stdin.close()
        try:
            result = process.wait(timeout=timeout)
        except subprocess.TimeoutExpired as error:
            raise SmokeError('LSP did not exit after shutdown') from error
        require(result == 0, f'LSP exit {result}')
        require(not process.stderr.read(), 'LSP wrote unexpected stderr')
    finally:
        if process.poll() is None:
            process.kill()
            process.wait()
        process.stdout.close()
        process.stderr.close()
        if not process.stdin.closed:
            process.stdin.close()
        reader.join(timeout=1)
    require(isinstance(responses[1], dict), 'LSP initialize result is not an object')
    capabilities = responses[1].get('capabilities', {})
    require(capabilities.get('hoverProvider'), "LSP did not advertise hover")
    require(responses[2] and 'length' in json.dumps(responses[2]),
            "LSP builtin hover did not return embedded documentation")
    require(isinstance(responses[3], list) and responses[3], "LSP symbols are empty")
    require(responses[4] is None, "LSP shutdown did not return null")


def smoke(quint, lsp, timeout):
    # Copy only the two executables. Neither their old siblings, source trees,
    # user configuration nor an ambient module/tool path is available here.
    with tempfile.TemporaryDirectory(prefix='quint-native-smoke-') as temporary:
        directory = Path(temporary)
        binaries = directory / 'bin'
        binaries.mkdir()
        suffix = '.exe' if os.name == 'nt' else ''
        command = binaries / ('quint' + suffix)
        server = binaries / ('quint-lsp' + suffix)
        shutil.copy2(quint, command)
        shutil.copy2(lsp, server)
        work = directory / 'work'
        work.mkdir()
        home = directory / 'home'
        home.mkdir()
        environment = {'PATH': '/nonexistent', 'HOME': str(home), 'TMPDIR': str(work),
                       'TMP': str(work), 'TEMP': str(work), 'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8'}
        (work / 'Smoke.qnt').write_text(MODEL, encoding='utf-8')

        def run(*arguments, expected=0, stdin=b''):
            return invoke(command, list(arguments), work, environment, timeout, expected, stdin)

        version = run('--version').stdout.decode('utf-8').strip()
        require(bool(version), 'empty version')
        run('parse', 'Smoke.qnt', '--out', 'parse.json', '--source-map', 'sources.json')
        require(load_stage(work, 'parse.json', 'parsing').get('modules'), 'parse modules are empty')
        require(bool(json.loads((work / 'sources.json').read_text())), 'source map is empty')
        run('typecheck', 'Smoke.qnt', '--out', 'types.json')
        require(load_stage(work, 'types.json', 'typechecking').get('types'), 'inferred types are empty')
        run('compile', 'Smoke.qnt', '--out', 'compile.json')
        require(load_stage(work, 'compile.json', 'compiling').get('main') == 'Smoke', 'wrong compiled main')
        tla = run('compile', 'Smoke.qnt', '--target', 'tlaplus').stdout
        require(b'MODULE Smoke' in tla and b'VARIABLE x' in tla, 'missing TLA+ output')
        (work / 'Smoke.tla').write_bytes(tla)
        run('run', 'Smoke.qnt', '--seed', '42', '--max-samples', '1', '--max-steps', '2',
            '--invariant', 'safe', '--out', 'run.json')
        require(len(load_stage(work, 'run.json', status='ok').get('trace', [])) == 3,
                'simulation did not produce three states')
        run('test', 'Smoke.qnt', '--seed', '42', '--out', 'test.json')
        tests = load_stage(work, 'test.json')
        require(tests.get('passed') == ['smokeTest'] and not tests.get('failed') and not tests.get('ignored'),
                'named test did not pass')
        for backend in ('apalache', 'tlc-go'):
            options = [] if backend == 'apalache' else ['--backend', backend]
            bounds = ['--max-steps', '2'] if backend == 'apalache' else []
            run('verify', 'Smoke.qnt', *options, *bounds, '--invariant', 'safe', '--out', backend + '.json')
            load_stage(work, backend + '.json', status='ok')
            run('verify', 'Smoke.qnt', *options, *bounds, '--invariant', 'broken',
                '--out', backend + '-bad.json', '--out-itf', backend + '.itf.json', expected=1)
            load_stage(work, backend + '-bad.json', status='violation')
            trace = json.loads((work / (backend + '.itf.json')).read_text())
            require(trace.get('vars') == ['x'] and len(trace.get('states', [])) == 2,
                    backend + ': missing two-state ITF counterexample')
        documentation = run('docs', 'Smoke.qnt', '--out', 'docs.json').stdout
        require(b'A finite counter' in documentation, 'docstring missing from Markdown')
        require(load_stage(work, 'docs.json', 'documentation').get('documentation'), 'documentation stage is empty')
        for arguments in (['repl'], []):
            result = run(*arguments, '--seed', '42', '--verbosity', '0', stdin=b'1 + 2\n.exit\n')
            require(result.stdout == b'3\n' and not result.stderr, 'REPL stream result differs')
        check_lsp(server, work, environment, timeout)
        return {'version': version, 'path': environment['PATH'], 'checks': [
            'parse+source-map', 'typecheck', 'compile-json', 'compile-tlaplus', 'run', 'test',
            'verify-apalache', 'verify-tlc-go', 'counterexamples+itf', 'docs', 'repl',
            'default-repl', 'lsp-lifecycle+builtin-hover+symbols']}


def main(arguments=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--quint', required=True, type=executable, help='built native Quint executable')
    parser.add_argument('--lsp', required=True, type=executable, help='built native Quint language server')
    parser.add_argument('--timeout', type=positive_seconds, default=30, help='seconds per process (default: 30)')
    options = parser.parse_args(arguments)
    try:
        report = smoke(options.quint, options.lsp, options.timeout)
    except (SmokeError, OSError, ValueError) as error:
        parser.exit(1, f'native smoke: {error}\n')
    print(json.dumps(report, sort_keys=True))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
