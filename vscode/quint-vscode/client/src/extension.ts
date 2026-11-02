/* ----------------------------------------------------------------------------------
 * Copyright (c) Informal Systems.
 * Licensed under the Apache License, Version 2.0.
 * See LICENSE in the project root for license information.
 * --------------------------------------------------------------------------------- */

import { ExtensionContext, ExtensionMode, window, workspace } from 'vscode'

import { LanguageClient, LanguageClientOptions, ServerOptions, TransportKind } from 'vscode-languageclient/node'
import { ServerSelection, selectLanguageServer } from './serverSelection'

let client: LanguageClient

export function activate(context: ExtensionContext) {
  const configuration = workspace.getConfiguration('quint.languageServer')
  let selection: ServerSelection
  try {
    selection = selectLanguageServer(
      { backend: configuration.get<string>('backend', 'go'), executable: configuration.get<string>('path', '') },
      context.extensionPath,
      process.platform,
      undefined,
      context.extensionMode === ExtensionMode.Development ? process.env.QUINT_VSCODE_SERVER_BACKEND : undefined
    )
  } catch (error) {
    void window.showErrorMessage(String(error))
    return
  }
  const serverOptions: ServerOptions =
    selection.backend === 'go'
      ? {
          run: { command: selection.command, transport: TransportKind.stdio },
          debug: { command: selection.command, transport: TransportKind.stdio },
        }
      : {
          run: { module: selection.module, transport: TransportKind.ipc },
          debug: {
            module: selection.module,
            transport: TransportKind.ipc,
            options: { execArgv: ['--nolazy', '--inspect=6009'] },
          },
        }

  const watcher = workspace.createFileSystemWatcher('**/*.qnt')
  context.subscriptions.push(watcher)

  // Options to control the language client
  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { scheme: 'file', language: 'quint' },
      { scheme: 'untitled', language: 'quint' },
    ],
    synchronize: {
      // watch for changes in Quint files
      fileEvents: watcher,
    },
  }

  // Create the language client and start the client.
  client = new LanguageClient('quintLspClient', 'Quint Language Server', serverOptions, clientOptions)

  // Start the client. This will also launch the server
  context.subscriptions.push(client.start())
  void client.onReady().catch(error => {
    const executable = selection.backend === 'go' ? ` (${selection.command})` : ''
    const hint =
      selection.backend === 'go'
        ? ' Install quint-lsp or set quint.languageServer.path, then reload the window.'
        : ' Compile the TypeScript server, then reload the window.'
    void window.showErrorMessage(
      `Quint ${selection.backend} language server${executable} failed to start: ${String(error)}.${hint}`
    )
  })
}

export function deactivate(): Promise<void> | undefined {
  if (!client) {
    return undefined
  }
  return client.stop()
}
