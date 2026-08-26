import { execFileSync } from 'node:child_process'
import { cpSync, existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

/**
 * Guards the packaging contract, which is the part of a source-only package that
 * silently depends on install order.
 *
 * This package exports raw TypeScript, so every consumer compiles it, and
 * `markdown-it` is resolved from *this* directory rather than from the consumer's
 * `node_modules`. A consumer therefore cannot satisfy the dependency by declaring
 * it themselves — it has to be reachable from here.
 *
 * That makes `packages/markdown/npm install` a required setup step rather than a
 * convenience, and nothing in either app's lockfile performs it: npm does not
 * install a `file:` link target's dependencies. Without this test the failure is
 * invisible on any machine where the directory happens to exist, and appears only
 * on a fresh clone or in CI.
 */
const packageRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')

describe('packaging', () => {
  it('resolves its own runtime dependency from its own tree', () => {
    // import.meta.resolve would answer from the test runner's perspective. What
    // matters is what a compiler walking up from this directory finds.
    expect(existsSync(join(packageRoot, 'node_modules', 'markdown-it'))).toBe(true)
  })

  it('resolves the types a consumer needs to compile this source', () => {
    // markdown-it ships no .d.ts of its own, so @types/markdown-it is load-bearing
    // for every consumer's type-check -- not merely a local dev convenience.
    expect(existsSync(join(packageRoot, 'node_modules', '@types', 'markdown-it'))).toBe(true)
  })

  it('declares markdown-it as a runtime dependency, not a dev one', () => {
    // A consumer's production build compiles this file, so a devDependency here
    // would be absent from any install that skips dev trees.
    const manifest = JSON.parse(readFileSync(join(packageRoot, 'package.json'), 'utf8')) as {
      dependencies?: Record<string, string>
      devDependencies?: Record<string, string>
    }

    expect(manifest.dependencies?.['markdown-it']).toBe('14.1.0')
    expect(manifest.devDependencies?.['@types/markdown-it']).toBeDefined()
  })

  it('type-checks standalone, without a consumer providing anything', () => {
    // The real assertion behind the two existence checks above: tsc run from this
    // directory alone must succeed. If it needs something only admin/ or frontend/
    // provides, the package is not self-contained and the next fresh clone breaks.
    const workdir = mkdtempSync(join(tmpdir(), 'alive-md-resolution-'))
    try {
      cpSync(join(packageRoot, 'src'), join(workdir, 'src'), { recursive: true })
      cpSync(join(packageRoot, 'node_modules'), join(workdir, 'node_modules'), {
        recursive: true,
        // Preserve the symlinks npm created rather than copying through them.
        verbatimSymlinks: true,
      })

      const output = execFileSync(
        'node',
        [
          join(packageRoot, 'node_modules', 'typescript', 'bin', 'tsc'),
          '--noEmit',
          '--strict',
          '--module',
          'preserve',
          '--moduleResolution',
          'bundler',
          '--target',
          'es2022',
          join(workdir, 'src', 'index.ts'),
        ],
        { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] },
      )

      expect(output.trim()).toBe('')
    } finally {
      rmSync(workdir, { recursive: true, force: true })
    }
  })
})
