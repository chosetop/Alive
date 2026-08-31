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

  it('shows the empty-state copy when there is no recent entry', () => {
    const wrapper = mount(WorldDeskCard, {
      props: {
        definition: ADMIN_WORLD_REGISTRY[2],
        snapshot: snapshot({
          setting: { ...setting(), world: 'video', nav_label: '影像' },
          recentEntry: null,
          entryCount: 0,
          categoryCount: 1,
        }),
      },
    })

    expect(wrapper.text()).toContain('还没有影像。选择一段视频开始。')
  })

  it('uses a stretched enter surface over the card while keeping the settings action separate', () => {
    const source = readFileSync(
      resolve(dirname(fileURLToPath(import.meta.url)), 'WorldDeskCard.vue'),
      'utf8',
    )

    expect(source).toMatch(/\.world-desk__enter-surface\s*\{[\s\S]*position:\s*absolute/)
    expect(source).toMatch(/\.world-desk__enter-surface\s*\{[\s\S]*inset:\s*0/)
    expect(source).toMatch(/\.world-desk__settings\s*\{[\s\S]*z-index:\s*2/)
  })
})
