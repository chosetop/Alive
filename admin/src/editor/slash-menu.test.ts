import { describe, expect, it } from 'vitest'

import { slashCommands, slashQuery } from './slash-menu'

describe('slash menu', () => {
  it('reads only a slash query at the end of the current paragraph', () => {
    expect(slashQuery('写一段 /列表')).toBe('列表')
    expect(slashQuery('写一段 /列表 后面')).toBeNull()
  })

  it('filters commands from the current slash query', () => {
    expect(slashCommands('/quote').map((command) => command.id)).toEqual(['quote'])
  })
})
