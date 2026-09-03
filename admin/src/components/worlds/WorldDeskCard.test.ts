import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import { ADMIN_WORLD_REGISTRY } from '../../content-worlds/registry'
import type { AdminWorldSetting, EntryListItem } from '../../types/api'
import WorldDeskCard, { type WorldDeskSnapshot } from './WorldDeskCard.vue'

function setting(): AdminWorldSetting {
  return {
    world: 'journal',
    status: 'open',
    nav_label: '日志',
    sort_order: 10,
    default_view: '',
    revision: 1,
    updated_at: '2026-08-29T09:00:00Z',
  }
}

function recentEntry(): EntryListItem {
  return {
    id: 11,
    world: 'journal',
    kind: 'entry',
    title: '一场缓慢的夏雨',
    slug: 'slow-summer-rain',
    summary: '傍晚后的雷声没有追上雨。',
    cover_url: '',
    meta: {},
    word_count: 728,
    category: null,
    happened_at: '2026-08-12T10:00:00Z',
    published_at: null,
    status: 'draft',
    visibility: 'private',
    created_at: '2026-08-12T10:00:00Z',
    updated_at: '2026-08-30T08:00:00Z',
  }
}

function snapshot(overrides: Partial<WorldDeskSnapshot> = {}): WorldDeskSnapshot {
  return {
    setting: setting(),
    recentEntry: recentEntry(),
    recentContentMd: '雨落在窗台。\n\n屋里的人没有急着关窗。',
    entryCount: 12,
    categoryCount: 4,
    error: null,
    canEnter: true,
    canRetry: false,
    ...overrides,
  }
}

describe('WorldDeskCard', () => {
  it('renders the recent draft summary and emits enter/settings actions', async () => {
    const wrapper = mount(WorldDeskCard, {
      props: {
        definition: ADMIN_WORLD_REGISTRY[0],
        snapshot: snapshot(),
      },
    })

    expect(wrapper.get('[data-world-desk="journal"]').text()).toContain('一场缓慢的夏雨')
    expect(wrapper.get('[data-world-status]').text()).toBe('已开放')

    await wrapper.get('[data-world-enter]').trigger('click')
    expect(wrapper.emitted('enter')).toEqual([['journal']])

    await wrapper.get('[data-world-settings]').trigger('click')
    expect(wrapper.emitted('settings')).toEqual([['journal']])
  })

  it('uses only the author-defined world name inside the card', () => {
    const wrapper = mount(WorldDeskCard, {
      props: {
        definition: ADMIN_WORLD_REGISTRY[0],
        snapshot: snapshot({
          setting: { ...setting(), nav_label: '随手记' },
          entryCount: 12,
        }),
      },
    })

    expect(wrapper.get('h2').text()).toBe('随手记')
    expect(wrapper.text()).toContain('12 篇内容')
    expect(wrapper.text()).not.toContain('日志')
  })

  it('keeps settings and enter as separate focusable buttons in visual order', () => {
    const wrapper = mount(WorldDeskCard, {
      props: {
        definition: ADMIN_WORLD_REGISTRY[0],
        snapshot: snapshot(),
      },
    })

    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(2)
    expect(buttons.map((button) => button.text().trim())).toEqual(['设置', '进入最近编辑'])
    expect(buttons[0].attributes('data-world-settings')).toBeDefined()
    expect(buttons[1].attributes('data-world-enter')).toBeDefined()
  })

  it('treats visible card content as an enter target while keeping settings separate', async () => {
    const wrapper = mount(WorldDeskCard, {
      props: {
        definition: ADMIN_WORLD_REGISTRY[0],
        snapshot: snapshot(),
      },
    })

    await wrapper.get('[data-world-preview]').trigger('click')
    expect(wrapper.emitted('enter')).toEqual([['journal']])

    await wrapper.get('[data-world-settings]').trigger('click')
    expect(wrapper.emitted('enter')).toEqual([['journal']])
    expect(wrapper.emitted('settings')).toEqual([['journal']])
  })

  it('renders recent journal Markdown as a read-only preview without the world description', () => {
    const wrapper = mount(WorldDeskCard, {
      props: {
        definition: ADMIN_WORLD_REGISTRY[0],
        snapshot: snapshot(),
      },
    })

    expect(wrapper.get('[data-world-preview]').text()).toContain('屋里的人没有急着关窗。')
    expect(wrapper.text()).not.toContain(ADMIN_WORLD_REGISTRY[0].description)
  })

  it('blocks entry on a recent-load failure and offers an inline retry', async () => {
    const wrapper = mount(WorldDeskCard, {
      props: {
        definition: ADMIN_WORLD_REGISTRY[0],
        snapshot: snapshot({
          recentEntry: null,
          error: '暂时无法读取最近编辑，请先重试。',
          canEnter: false,
          canRetry: true,
        }),
      },
    })

    expect(wrapper.get('[data-world-enter]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-world-retry]').text()).toContain('重试')

    await wrapper.get('[data-world-enter]').trigger('click')
    expect(wrapper.emitted('enter')).toBeUndefined()

    await wrapper.get('[data-world-retry]').trigger('click')
    expect(wrapper.emitted('retry')).toEqual([['journal']])
  })

  it('uses a world-neutral empty state beneath every author-defined name', () => {
    const worlds = [
      ['journal', '手记'],
      ['saying', '闪念'],
      ['video', '镜头'],
    ] as const

    for (const [world, label] of worlds) {
      const definition = ADMIN_WORLD_REGISTRY.find((item) => item.key === world)
      expect(definition).toBeTruthy()
      const wrapper = mount(WorldDeskCard, {
        props: {
          definition: definition!,
          snapshot: snapshot({
            setting: { ...setting(), world, nav_label: label },
            recentEntry: null,
            entryCount: 0,
            categoryCount: world === 'video' ? 1 : 0,
          }),
        },
      })

      expect(wrapper.get('h2').text()).toBe(label)
      expect(wrapper.text()).toContain('还没有内容。开始第一篇。')
      expect(wrapper.text()).not.toContain(ADMIN_WORLD_REGISTRY.find((item) => item.key === world)!.label)
      wrapper.unmount()
    }
  })

  it('uses a stretched enter surface over the card while keeping the settings action separate', () => {
    const source = readFileSync(
      resolve(dirname(fileURLToPath(import.meta.url)), 'WorldDeskCard.vue'),
      'utf8',
    )

    expect(source).toMatch(/\.world-desk__enter-surface\s*\{[\s\S]*position:\s*absolute/)
    expect(source).toMatch(/\.world-desk__enter-surface\s*\{[\s\S]*inset:\s*0/)
    expect(source).toMatch(/\.world-desk__settings\s*\{[\s\S]*z-index:\s*2/)
    expect(source).toMatch(/@media \(max-width:\s*48rem\)[\s\S]*\.world-desk\s*\{[\s\S]*min-height:\s*auto/)
  })
})
