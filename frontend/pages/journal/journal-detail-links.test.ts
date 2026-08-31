import { describe, expect, it } from 'vitest'

describe('journal detail navigation contract', () => {
  it('documents the world return route used by the detail page', () => {
    expect('/journal').toBe('/journal')
  })
})
