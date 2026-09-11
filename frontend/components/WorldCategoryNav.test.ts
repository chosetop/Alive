import { describe, expect, it } from 'vitest'

import { renderComponent } from '~/test-support/render'
import type { Category } from '~/types'
import WorldCategoryNav from './WorldCategoryNav.vue'
import navigationSource from './WorldCategoryNav.vue?raw'
import layoutSource from './WorldBrowseLayout.vue?raw'

const categories: Category[] = [
  { id: 1, world: 'saying', name: '日常', slug: 'daily', description: '', sort_order: 0, entry_count: 4 },
  { id: 2, world: 'saying', name: '空白', slug: 'empty', description: '', sort_order: 1, entry_count: 0 },
]

describe('WorldCategoryNav', () => {
  it('keeps category navigation inside the current content world', async () => {
    const html = await renderComponent(WorldCategoryNav, { world: 'saying', categories })

    expect(html).not.toContain('>全部<')
    expect(html).not.toContain('href="/sayings"')
    expect(html).toContain('href="/sayings/categories/daily"')
    expect(html).not.toContain('/journal/categories/daily')
    expect(html).toContain('href="/sayings/categories/empty"')
    expect(html).toContain('空白')
  })

  it('keeps the active category visible and marks it as the current page', async () => {
    const html = await renderComponent(WorldCategoryNav, { world: 'saying', categories, activeSlug: 'empty' })

    expect(html).toContain('href="/sayings/categories/empty"')
    expect(html).toContain('aria-current="page"')
  })

  it('reads as a sticky contents rail without a redundant total heading', async () => {
    const html = await renderComponent(WorldCategoryNav, { world: 'saying', categories })

    expect(html).not.toContain('分类索引')
    expect(html).not.toContain('共 7 则')
    expect(html).not.toContain('分类</strong>')
    expect(navigationSource).toContain('position: sticky')
    expect(navigationSource).toContain('flex-direction: column')
  })

  it('reclaims the sidebar column when there is no navigation', () => {
    expect(layoutSource).toContain('world-browse__rail--empty')
    expect(layoutSource).toContain('grid-column: 1 / -1')
  })

  it('keeps the category title and navigation together in a sticky rail', () => {
    expect(layoutSource).toContain('class="world-browse__rail"')
    expect(layoutSource).toContain('position: sticky')
    expect(layoutSource).toContain('top: calc(var(--alive-masthead-height')
  })
})
