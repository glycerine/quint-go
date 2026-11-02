import { expect } from 'chai'
import { EventEmitter } from 'events'
import { readFileSync } from 'fs'
import { join } from 'path'
import * as ts from 'typescript'
import { runInNewContext } from 'vm'

function loadLogger(level?: string, failSynchronously = false) {
  const writes: string[] = []
  const stream = new EventEmitter() as EventEmitter & { write: (message: string) => void }
  stream.write = message => writes.push(message)
  let opened = 0
  const source = readFileSync(join(__dirname, '../src/logger.ts'), 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  })
  const output = { exports: {} }
  runInNewContext(compiled.outputText, {
    exports: output.exports,
    module: output,
    process: { env: { LOG_LEVEL: level, HOME: '/not-writable' } },
    console: {},
    require: (name: string) =>
      name === 'fs'
        ? {
            createWriteStream: () => {
              opened++
              if (failSynchronously) throw new Error('read-only log directory')
              return stream
            },
          }
        : require(name),
  })
  const result = output.exports as {
    logger: { info: (...args: any[]) => void; debug: (...args: any[]) => void; error: (...args: any[]) => void }
    overrideConsole: () => void
  }
  return { ...result, opened: () => opened, writes, stream }
}

describe('logger filesystem ownership', () => {
  it('does not open a log with disabled or unknown log levels', () => {
    for (const level of [undefined, 'NONE', 'not-a-level']) {
      const log = loadLogger(level)
      log.logger.info('hidden')
      log.logger.error('also hidden')
      expect(log.opened()).to.equal(0)
      expect(log.writes).to.deep.equal([])
    }
  })

  it('opens lazily on the first enabled message and preserves formatting/filtering', () => {
    const log = loadLogger('INFO')
    expect(log.opened()).to.equal(0)
    log.logger.debug('hidden')
    expect(log.opened()).to.equal(0)
    log.logger.info('value %d', 7)
    log.logger.error('failure %s', 'details')
    expect(log.opened()).to.equal(1)
    expect(log.writes).to.have.length(2)
    expect(log.writes[0]).to.match(/\[INFO\] value 7\n$/)
    expect(log.writes[1]).to.match(/\[ERROR\] failure details\n$/)
  })

  it('keeps an asynchronous log error from crashing the language server', () => {
    const log = loadLogger('DEBUG')
    log.logger.info('first message')
    expect(() => log.stream.emit('error', new Error('read-only log directory'))).not.to.throw()
    log.logger.error('later message')
    expect(log.opened()).to.equal(1)
    expect(log.writes).to.have.length(1)
  })

  it('does not repeatedly retry a synchronous log-open failure', () => {
    const log = loadLogger('INFO', true)
    expect(() => log.logger.info('first message')).not.to.throw()
    expect(() => log.logger.error('later message')).not.to.throw()
    expect(log.opened()).to.equal(1)
    expect(log.writes).to.have.length(0)
  })
})
