# Go parser generation

The Go port uses ANTLR-generated parsers for the repository's Quint and Effect
syntax. `adapt.py` translates the Quint grammar's TypeScript actions and parameter
syntax to Go; it fails if the expected action sites change. Do not edit the
adapted grammar or generated parser files directly.

Prerequisites: Python 3, Java, Go (`gofmt`), and ANTLR **4.13.2**. Obtain the tool
jar from https://www.antlr.org/download/antlr-4.13.2-complete.jar. Its pinned SHA-256
is `eae2dfa119a64327444672aff63e9ec35a20180dc5b8090b7a6ab85125df4d76`.
The generator verifies this checksum before invoking Java and never downloads
anything. Keep the jar outside the repository.

From the repository root:

```sh
python3 pkg/grammar/generate.py --jar /path/to/antlr-4.13.2-complete.jar
python3 pkg/grammar/generate.py --jar /path/to/antlr-4.13.2-complete.jar --check
python3 -m unittest discover -s pkg/grammar -p 'test_*.py'
go test ./pkg/... -count=1
```

Generation runs in a temporary directory using fixed relative input paths so
ANTLR's generated headers are reproducible. Both parsers are generated and
formatted successfully before any repository file is changed. `--check` compares
all generated outputs without writing to the repository. Generation takes several
seconds and is a maintenance check, not part of the fast default Go test suite.
The generator replaces ANTLR's unused-label jump after each rule's return with
a constant-false jump before the return. This preserves rule behavior and Go's
label-use requirement while allowing `go vet` to check the generated packages
without disabling unreachable-code analysis. The transformation fails if the
pinned generator's rule-tail template changes.
The Python unit tests verify adaptation drift detection, rule-tail conversion,
and jar verification;
Go parser tests exercise the generated grammar and translated diagnostic actions.
