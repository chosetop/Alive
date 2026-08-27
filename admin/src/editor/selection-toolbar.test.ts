import { describe, expect, it } from 'vitest'

import { SELECTION_TOOLBAR_ACTIONS, shouldShowSelectionToolbar } from './selection-toolbar'

describe('selection toolbar', () => {
  it('has the five contextual formatting actions', () => {
    expect(SELECTION_TOOLBAR_ACTIONS.map((action) => action.id)).toEqual([
      'bold', 'emphasis', 'strike', 'inline-code', 'link',
    ])
  })

  it('only shows for a non-empty selection', () => {
    expect(shouldShowSelectionToolbar(3, 3)).toBe(false)
    expect(shouldShowSelectionToolbar(3, 8)).toBe(true)
  })
})
