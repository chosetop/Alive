import { describe, expect, it, vi } from 'vitest'

import { EDITOR_COMMANDS, filterEditorCommands, type EditorCommandContext } from './editor-commands'

describe('editor command registry', () => {
  it('filters by Chinese labels and English keywords', () => {
    expect(filterEditorCommands('列表').map((command) => command.id)).toContain('bullet-list')
    expect(filterEditorCommands('quote').map((command) => command.id)).toContain('quote')
  })

  it('executes a command and removes the typed slash query', () => {
    const replaceSlashQuery = vi.fn()
    const toggleBlock = vi.fn()
    const context: EditorCommandContext = {
      replaceSlashQuery,
      toggleBlock,
    }

    const command = EDITOR_COMMANDS.find(({ id }) => id === 'heading-2')!
    expect(command.run(context)).toBe(true)
    expect(toggleBlock).toHaveBeenCalledWith('heading', 2)
    expect(replaceSlashQuery).toHaveBeenCalledOnce()
    expect(replaceSlashQuery.mock.invocationCallOrder[0])
      .toBeLessThan(toggleBlock.mock.invocationCallOrder[0])
  })
})
