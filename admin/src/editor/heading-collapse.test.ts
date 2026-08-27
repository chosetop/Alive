import { describe, expect, it } from 'vitest'

import { toggleHeadingSection } from './heading-collapse'

describe('heading section collapse', () => {
  it('hides a heading section through the next heading of the same level', () => {
    const root = document.createElement('div')
    root.innerHTML = '<h2>一</h2><p>甲</p><h3>一之一</h3><p>乙</p><h2>二</h2><p>丙</p>'
    const heading = root.querySelector('h2') as HTMLHeadingElement

    toggleHeadingSection(heading)

    expect(root.children[1].hasAttribute('hidden')).toBe(true)
    expect(root.children[2].hasAttribute('hidden')).toBe(true)
    expect(root.children[3].hasAttribute('hidden')).toBe(true)
    expect(root.children[5].hasAttribute('hidden')).toBe(false)
    expect(heading.dataset.headingCollapsed).toBe('true')
  })

  it('restores a collapsed section without changing its content', () => {
    const root = document.createElement('div')
    root.innerHTML = '<h2>一</h2><p>甲</p><h2>二</h2><p>乙</p>'
    const heading = root.querySelector('h2') as HTMLHeadingElement

    toggleHeadingSection(heading)
    toggleHeadingSection(heading)

    expect(root.textContent).toBe('一甲二乙')
    expect([...root.children].some((child) => child.hasAttribute('hidden'))).toBe(false)
    expect(heading.dataset.headingCollapsed).toBeUndefined()
  })
})
