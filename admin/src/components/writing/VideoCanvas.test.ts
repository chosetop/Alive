import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { EntryDetail } from '../../types/api'
import VideoCanvas from './VideoCanvas.vue'

vi.mock('./VideoUpload.vue', async () => {
  const { defineComponent } = await import('vue')
  return {
    default: defineComponent({
      name: 'VideoUpload',
      props: {
        entryId: { type: [Number, null], default: null },
        revision: { type: Number, required: true },
        disabled: { type: Boolean, default: false },
      },
      emits: ['revision', 'aspect'],
      template:
        '<button data-video-upload type="button" :disabled="disabled" @click="$emit(\'revision\', revision + 1)">上传</button>',
    }),
  }
})

function entry(overrides: Partial<EntryDetail> = {}): EntryDetail {
  return {
    id: 21,
    revision: 4,
    world: 'video',
    kind: 'entry',
    type: 'journal',
    title: '落日片段',
    slug: 'sunset-clip',
    summary: '海风把最后一点光吹散了。',
    content_md: '',
    cover_url: '',
    meta: {},
    word_count: 0,
    category_id: 0,
    category: null,
    happened_at: null,
    published_at: null,
    status: 'draft',
    visibility: 'private',
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-30T08:00:00Z',
    ...overrides,
  }
}

describe('VideoCanvas', () => {
  it('renders the viewfinder with title and summary fields', () => {
    const wrapper = mount(VideoCanvas, {
      props: {
        entry: entry(),
        title: '落日片段',
        summary: '海风把最后一点光吹散了。',
      },
    })

    expect(wrapper.get('[data-video-viewfinder]').element).toBeTruthy()
    expect(Number(wrapper.get('[data-video-viewfinder]').attributes('data-aspect'))).toBeCloseTo(16 / 9)
    expect(wrapper.get('[data-video-title]').element).toBeTruthy()
    expect(wrapper.get('[data-video-summary]').element).toBeTruthy()
  })

  it('shows an upload control for an unsaved entry', () => {
    const wrapper = mount(VideoCanvas, {
      props: {
        entry: null,
        title: '',
        summary: '',
        ensureEntry: async () => 21,
      },
    })

    expect(wrapper.get('[data-video-upload]').text()).toContain('上传')
    expect(wrapper.findComponent({ name: 'VideoUpload' }).exists()).toBe(true)
  })

  it('forwards summary edits and media revision events', async () => {
    const wrapper = mount(VideoCanvas, {
      props: {
        entry: entry(),
        title: '落日片段',
        summary: '海风把最后一点光吹散了。',
      },
    })

    await wrapper.get('[data-video-summary]').setValue('新的说明')
    expect(wrapper.emitted('update:summary')).toEqual([['新的说明']])

    await wrapper.get('[data-video-upload]').trigger('click')

    expect(wrapper.emitted('revision')).toEqual([[5]])
  })

  it('passes the disabled state through to the video controls', () => {
    const wrapper = mount(VideoCanvas, {
      props: {
        entry: entry(),
        title: '落日片段',
        summary: '海风把最后一点光吹散了。',
        disabled: true,
      },
    })

    expect(wrapper.get('[data-video-title]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-video-summary]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-video-upload]').attributes('disabled')).toBeDefined()
  })

  it('starts at 16:9 and lets video metadata control the viewfinder ratio', async () => {
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'VideoCanvas.vue'), 'utf8')

    const wrapper = mount(VideoCanvas, { props: { entry: entry(), title: '落日片段', summary: '' } })
    wrapper.getComponent({ name: 'VideoUpload' }).vm.$emit('aspect', 9 / 16)
    await wrapper.vm.$nextTick()

    expect(Number(wrapper.get('[data-video-viewfinder]').attributes('data-aspect'))).toBeCloseTo(9 / 16)
    expect(source).toContain("aspectRatio: String(videoAspect)")
  })

  it('uses a desktop split with a stable media column and a mobile fallback', () => {
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'VideoCanvas.vue'), 'utf8')

    expect(source).toContain('data-media-layout')
    expect(source).toMatch(/\.video-canvas__layout\s*\{[\s\S]*grid-template-columns:/)
    expect(source).toMatch(/@media \(max-width: 48rem\)[\s\S]*\.video-canvas__layout\s*\{[\s\S]*grid-template-columns:\s*1fr/)
  })

  it('keeps the mobile viewfinder within the canvas width', () => {
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'VideoCanvas.vue'), 'utf8')

    expect(source).toMatch(/@media \(max-width: 48rem\)[\s\S]*\.video-canvas__viewfinder\s*\{[\s\S]*width:\s*100%[\s\S]*min-height:\s*0/)
  })
})
