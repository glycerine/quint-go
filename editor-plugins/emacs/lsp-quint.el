;;; lsp-quint.el --- Quint LSP Client settings  -*- lexical-binding: t; -*-
;;;
;; Copyright (C) 2023 Gabriela Moreira

;; Author: Gabriela Moreira (gabriela@informal.systems)
;; URL: https://github.com/informalsystems/quint
;; Version: 1.0.0
;; Created: 28 Aug 2023
;; Updated 21 Aug 2023
;; Keywords: languages

;; This file is not part of GNU Emacs.

;;; Commentary:

;; A client for lsp-mode for the Quint Specification Language, using the quint language server.

;;; Code:

(require 'lsp-mode)
(require 'lsp-completion)
(require 'cl-lib)

(defgroup lsp-quint nil
  "LSP support for the Quint Specification Language, using the quint language server."
  :link '(url-link "https://github.com/informalsystems/quint")
  :group 'lsp-mode
  :package-version '(lsp-mode . "6.3.2"))

(defcustom lsp-quint-server-backend 'go
  "Quint language-server implementation.
Go uses `lsp-quint-go-server-command' without downloading another server.
TypeScript explicitly enables the existing npm language-server dependency."
  :type '(choice (const :tag "Go" go)
                 (const :tag "TypeScript" typescript))
  :group 'lsp-quint)

(defcustom lsp-quint-go-server-command '("quint-lsp")
  "Command and arguments for the Go Quint language server.
Set the first element to an absolute executable path when quint-lsp is not
on `exec-path'.  The Go executable uses stdio without a --stdio argument."
  :type '(repeat string)
  :group 'lsp-quint)

(lsp-dependency 'quint-language-server
                '(:system "quint-language-server")
                '(:npm :package "@informalsystems/quint-language-server"
                       :path "quint-language-server"))

(defun lsp-quint--server-command ()
  "Return the selected server command without changing backend on failure."
  (pcase lsp-quint-server-backend
    ('go
     (unless (and (consp lsp-quint-go-server-command)
                  (cl-every #'stringp lsp-quint-go-server-command)
                  (> (length (car lsp-quint-go-server-command)) 0))
       (user-error "Set lsp-quint-go-server-command to a nonempty command list"))
     (copy-sequence lsp-quint-go-server-command))
    ('typescript (list (lsp-package-path 'quint-language-server) "--stdio"))
    (_ (user-error "Unknown Quint server backend: %s" lsp-quint-server-backend))))

(defun lsp-quint--ensure-server (_client callback error-callback _update?)
  "Resolve the chosen server, installing npm only for explicit TypeScript."
  (if (eq lsp-quint-server-backend 'go)
      (let ((command (car (lsp-quint--server-command))))
        (if (executable-find command)
            (funcall callback)
          (funcall error-callback
                   (format "Install Go quint-lsp or customize lsp-quint-go-server-command (%s)"
                           command))))
    (if (eq lsp-quint-server-backend 'typescript)
        (lsp-package-ensure
         'quint-language-server
         (-partial #'lsp-package-ensure
                   'quint-language-server
                   callback
                   error-callback)
         error-callback)
      (user-error "Unknown Quint server backend: %s" lsp-quint-server-backend))))

(add-to-list 'lsp-language-id-configuration '(quint-mode . "quint"))
(lsp-register-client
 (make-lsp-client :new-connection (lsp-stdio-connection
                                   #'lsp-quint--server-command)
                  :major-modes '(quint-mode)
                  :activation-fn (lsp-activate-on "quint")
                  :language-id "quint"
                  :priority 0
                  :server-id 'quint-language-server
                  :completion-in-comments? t
                  :download-server-fn #'lsp-quint--ensure-server))

(lsp-consistency-check lsp-quint)

(provide 'lsp-quint)
;;; lsp-quint.el ends here
