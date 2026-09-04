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
        entryId: { type: Number, required: true },
        revision: { type: Number, required: true },
        disabled: { type: Boolean, default: false },
      },
      emits: ['revision'],
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
    expect(wrapper.get('[data-video-viewfinder]').attributes('data-aspect')).toBe('16:9')
    expect(wrapper.get('[data-video-title]').element).toBeTruthy()
    expect(wrapper.get('[data-video-summary]').element).toBeTruthy()
  })

  it('shows client-first copy for an unsaved entry and does not mount upload controls', () => {
    const wrapper = mount(VideoCanvas, {
      props: {
        entry: null,
        title: '',
        summary: '',
      },
    })

    expect(wrapper.text()).toContain('主视频会出现在这里')
    expect(wrapper.findComponent({ name: 'VideoUpload' }).exists()).toBe(false)
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

  it('pins the viewfinder to a 16:9 aspect ratio in source as well as DOM metadata', () => {
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'VideoCanvas.vue'), 'utf8')

    expect(source).toMatch(/\.video-canvas__viewfinder\s*\{[\s\S]*aspect-ratio:\s*16\s*\/\s*9/)
  })

  it('uses a desktop split with a stable media column and a mobile fallback', () => {
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'VideoCanvas.vue'), 'utf8')

    expect(source).toContain('data-media-layout')
    expect(source).toMatch(/\.video-canvas__layout\s*\{[\s\S]*grid-template-columns:/)
    expect(source).toMatch(/@media \(max-width: 48rem\)[\s\S]*\.video-canvas__layout\s*\{[\s\S]*grid-template-columns:\s*1fr/)
  })
})
