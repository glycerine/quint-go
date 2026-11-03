# Vim support

This guide will help you set up Vim for Quint, including syntax highlighting and an integrated language server. Follow these easy steps for a smooth configuration.

## Syntax Highlighting

1. If you are using vim, copy the [quint.vim] file to `~/.vim/syntax/`.
1. If you are using neovim, copy the [quint.vim] file to `~/.config/nvim/syntax`.
2. Choose one of the following options to enable syntax highlighting for Quint:

   - **Option A (Manual):** Open a Quint file in Vim and manually set the syntax with `:set syntax=quint`.

   - **Option B (Automatic):** Add the following line to your `~/.vimrc` file:

     ```vim
     au BufNewFile,BufReadPost *.qnt runtime syntax/quint.vim
     ```

   - **Option C (Modelines):** Make sure you have modelines enabled and add the following line to the end of your Quint file:

     ```quint
     // vim: syntax=quint
     ```

## Language Server

1. Build `quint-lsp` using the [Go package instructions](../../pkg/README.md),
   and put the executable on `PATH`. The Go server uses stdio directly, without
   `--stdio` or a Node, Java, or Scala runtime.

### Neovim

2. Enable language server integration by adding the following lines to your `~/.config/nvim/init.vim`:

```vim-script
autocmd FileType quint lua vim.lsp.start({name = 'quint', cmd = {'quint-lsp'}, root_dir = vim.fs.dirname(vim.api.nvim_buf_get_name(0))})
au BufRead,BufNewFile *.qnt  setfiletype quint
```

### Vim

2. Install [`prabirshrestha/vim-lsp`](https://github.com/prabirshrestha/vim-lsp) (e.g., via vim-plug):

```vim-script
Plug 'prabirshrestha/vim-lsp'
```

3. Enable LSP for Quint

```vim-script
au BufRead,BufNewFile *.qnt setfiletype quint

let g:quint_lsp_command = get(g:, 'quint_lsp_command', ['quint-lsp'])
if executable(g:quint_lsp_command[0])
  au User lsp_setup call lsp#register_server({
    \ 'name': 'quint',
    \ 'cmd': {server_info->copy(g:quint_lsp_command)},
    \ 'allowlist': ['quint'],
    \ })
endif

function! s:on_lsp_buffer_enabled() abort
  setlocal omnifunc=lsp#complete
  setlocal signcolumn=yes
  if exists('+tagfunc') | setlocal tagfunc=lsp#tagfunc | endif
  nmap <buffer> gd <plug>(lsp-definition)
  nmap <buffer> gs <plug>(lsp-document-symbol-search)
  nmap <buffer> gr <plug>(lsp-references)
  nmap <buffer> <leader>rn <plug>(lsp-rename)
  nmap <buffer> [g <plug>(lsp-previous-diagnostic)
  nmap <buffer> ]g <plug>(lsp-next-diagnostic)
  nmap <buffer> K <plug>(lsp-hover)
  nnoremap <buffer> <expr><c-f> lsp#scroll(+4)
  nnoremap <buffer> <expr><c-d> lsp#scroll(-4)
endfunction

augroup lsp_install
  au!
  autocmd User lsp_buffer_enabled call s:on_lsp_buffer_enabled()
augroup END
```

To select an absolute executable path, set `g:quint_lsp_command` before this
configuration. Paths with spaces remain one list element:

```vim-script
let g:quint_lsp_command = ['/absolute/path/to/quint-lsp']
```

The snippets above do not launch another server if Go is unavailable. To
explicitly use [quint-language-server][] instead, install that npm package and
replace Neovim's `cmd` with `{'quint-language-server', '--stdio'}` or set Vim's
`g:quint_lsp_command` to `['quint-language-server', '--stdio']`.

Optional server diagnostics use `LOG_LEVEL`; see
[logging and protocol details](../../pkg/lsp/README.md). The Go server advertises
document symbols, definitions, references, rename, hover, completion, signature
help, and code actions. Workspace symbols, implementation lookup, and type
definition lookup are not advertised, so the example maps only supported methods.

The Vim configuration can be checked without downloaded plugins or a running
server using `python3 editor-plugins/vim/test-config.py`. This runs the actual
snippet in headless Vim; full Vim/Neovim host sessions remain separate validation.

[quint.vim]: ./quint.vim
[quint-language-server]: https://www.npmjs.com/package/@informalsystems/quint-language-server
