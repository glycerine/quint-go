# Emacs support

We have 2 packages for enabling support for Quint in Emacs.

1. `quint-mode` is a major mode that enables simple syntax highlighting
2. `lsp-quint` is a client for `lsp-mode`, using the Go `quint-lsp` server by default.

Build `quint-lsp` using the [Go package instructions](../../pkg/README.md), and
place it on Emacs's `exec-path`. The server speaks stdio directly and requires
no Node, Java, Scala, or downloaded language-server package.

These packages are not published anywhere for the moment. You can clone this git repo and add a configuration like the following (which uses `use-package`):

```elisp
(load-file "<path-to-quint-repo>/editor-plugins/emacs/quint-mode.el")
(load-file "<path-to-quint-repo>/editor-plugins/emacs/lsp-quint.el")
(require 'quint-mode)
(add-to-list 'auto-mode-alist '("\\.qnt" . quint-mode))
(use-package lsp-quint
  :ensure nil
  :hook (quint-mode . lsp))
```

For an executable outside `exec-path`, configure its command before opening a
Quint file. Each argument is a separate list element, so paths may contain spaces:

```elisp
(setq lsp-quint-go-server-command '("/absolute/path/to/quint-lsp"))
```

The Go selection reports a missing executable; it never installs or launches
TypeScript as a fallback. To explicitly use the existing TypeScript server:

```elisp
(setq lsp-quint-server-backend 'typescript)
```

That selection uses the existing `quint-language-server` system/npm dependency
and its `--stdio` argument. Restart the language-server workspace after changing
the backend. `LOG_LEVEL=DEBUG` or `LOG_LEVEL=TRACE` enables optional server logs;
see [logging and protocol details](../../pkg/lsp/README.md).

Client configuration tests run without downloaded packages or a live editor server:

```sh
emacs -Q --batch -l editor-plugins/emacs/lsp-quint-tests.el
```

These tests exercise backend/command selection and installation callbacks. A real
Emacs `lsp-mode` session remains separate host validation.
