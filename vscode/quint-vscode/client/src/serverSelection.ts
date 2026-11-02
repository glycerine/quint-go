/* ----------------------------------------------------------------------------------
 * Licensed under the Apache License, Version 2.0.
 * See LICENSE in the project root for license information.
 * --------------------------------------------------------------------------------- */

import { existsSync } from 'fs'
import * as path from 'path'

export interface ServerSettings {
  backend: string
  executable: string
}

export type ServerSelection = { backend: 'go'; command: string } | { backend: 'typescript'; module: string }

// Selection is explicit. A missing Go executable is a startup error, never a
// reason to launch the TypeScript server. The command is passed to spawn
// directly, so paths containing spaces do not need shell quoting.
export function selectLanguageServer(
  settings: ServerSettings,
  extensionPath: string,
  platform: string,
  fileExists: (file: string) => boolean = existsSync,
  developmentOverride?: string
): ServerSelection {
  const backend = developmentOverride ?? settings.backend
  if (backend === 'typescript') {
    return { backend, module: path.join(extensionPath, 'server', 'out', 'src', 'server.js') }
  }
  if (backend !== 'go') {
    throw new Error(`Unsupported Quint language-server backend '${backend}'. Choose 'go' or 'typescript'.`)
  }
  const bundled = path.join(extensionPath, 'bin', platform === 'win32' ? 'quint-lsp.exe' : 'quint-lsp')
  return {
    backend,
    command: settings.executable || (fileExists(bundled) ? bundled : 'quint-lsp'),
  }
}
