import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it } from 'vitest'

import AdminLayout from '../layouts/AdminLayout.vue'
import WritingLayout from '../layouts/WritingLayout.vue'
import { routes } from './index'

/**
 * These assert the one thing about this route table that is not obvious from
 * reading it: two top-level records both answer to `/entries`, and which shell a
 * path lands in depends on Vue Router's ranking rather than on their order in the
 * array. That ranking is a library behaviour, so it is worth pinning rather than
 * trusting -- if it changed, the symptom would be the writing canvas appearing
 * inside the utility chrome, or the article list losing its navigation.
 *
 * A fresh router per test over a memory history: the exported singleton installs
 * a guard that resolves the auth store, and resolution does not need a session.
 */
function resolver() {
  return createRouter({ history: createMemoryHistory(), routes })
}

/** The layout component of the matched chain, i.e. which shell renders. */
function shellFor(path: string): unknown {
  return resolver().resolve(path).matched[0]?.components?.default
}

describe('the route table', () => {
  it('serves the article library from the utility shell', () => {
    const resolved = resolver().resolve('/entries')

    expect(resolved.name).toBe('entries')
    expect(shellFor('/entries')).toBe(AdminLayout)
  })

  it('keeps the utility record ahead of the writing record', () => {
    // Not cosmetic ordering. The writing record's own `/entries` path scores
    // identically to the utility child of the same path, and Vue Router breaks
    // that tie by declaration order -- so listing the writing record first makes
    // /entries resolve to its unnamed parent and render an empty canvas where the
    // article library should be. Reproduced by swapping the two records here.
    const utilityIndex = routes.findIndex((route) => route.component === AdminLayout)
    const writingIndex = routes.findIndex((route) => route.component === WritingLayout)

    expect(utilityIndex).toBeGreaterThanOrEqual(0)
    expect(writingIndex).toBeGreaterThan(utilityIndex)
  })

  it('serves the editor from the writing shell, not nested inside the utility one', () => {
    const resolved = resolver().resolve('/entries/41')

    expect(resolved.name).toBe('entry-edit')
    expect(shellFor('/entries/41')).toBe(WritingLayout)
    // One shell in the chain, not both. Nesting would put the nav rail and the
    // canvas on screen together, which is the layout this workspace replaces.
    expect(resolved.matched.map((record) => record.components?.default)).not.toContain(AdminLayout)
  })

  it('serves the blank-draft route from the writing shell', () => {
    // `new` must outrank `:id`, or /entries/new would load an article whose id is
    // the literal string "new" and 404.
    expect(resolver().resolve('/entries/new').name).toBe('entry-new')
    expect(shellFor('/entries/new')).toBe(WritingLayout)
  })

  it('passes the id to the editor as a prop', () => {
    // The editor takes its subject as a prop rather than reading the route, so
    // the record has to opt in.
    const record = resolver().resolve('/entries/41').matched.at(-1)

    expect(record?.props.default).toBe(true)
  })

  it('marks only the writing routes as the writing workspace', () => {
    const router = resolver()

    expect(router.resolve('/entries/41').meta.writingWorkspace).toBe(true)
    expect(router.resolve('/entries/new').meta.writingWorkspace).toBe(true)
    expect(router.resolve('/entries').meta.writingWorkspace).toBeUndefined()
    expect(router.resolve('/categories').meta.writingWorkspace).toBeUndefined()
  })

  it('guards both shells with the same requiresAuth metadata', () => {
    const router = resolver()

    // Not duplicated auth logic -- duplicated metadata read by one guard. If a
    // record ever lost the flag, its pages would be reachable without a session.
    for (const path of ['/dashboard', '/entries', '/categories', '/entries/new', '/entries/41']) {
      expect(router.resolve(path).meta.requiresAuth, `${path} must require auth`).toBe(true)
    }
  })

  it('keeps the editor routes out of the utility navigation targets', () => {
    // The utility shell's own children. If an editor route were re-added here it
    // would shadow the writing shell for /entries/new, and the regression would
    // look like "the immersive editor stopped being immersive".
    const utilityChildren = routes
      .filter((route) => route.component === AdminLayout)
      .flatMap((route) => route.children ?? [])
      .map((child) => child.name)

    expect(utilityChildren).not.toContain('entry-new')
    expect(utilityChildren).not.toContain('entry-edit')
  })
})
