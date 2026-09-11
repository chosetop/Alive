import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import VideoUpload from './VideoUpload.vue'

const { uploadMedia, listEntryMedia, setPrimaryVideo } = vi.hoisted(() => ({
  uploadMedia: vi.fn(),
  listEntryMedia: vi.fn(),
  setPrimaryVideo: vi.fn(),
}))

vi.mock('../../media', () => ({
  uploadMedia,
}))

vi.mock('../../api', () => ({
  mediaApi: { listEntryMedia, setPrimaryVideo },
}))

describe('VideoUpload', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listEntryMedia.mockResolvedValue([])
  })

  it('makes the whole picker surface a file chooser target', () => {
    const wrapper = mount(VideoUpload, {
      props: { entryId: 21, revision: 4 },
    })

    const picker = wrapper.get('[data-video-picker]')
    const input = wrapper.get('input[type="file"]')

    expect(picker.element.tagName).toBe('LABEL')
    expect(picker.attributes('for')).toBe(input.attributes('id'))
    expect(input.attributes('accept')).toBe('video/mp4')
  })

  it('restores the saved primary video when an entry is reopened', async () => {
    listEntryMedia.mockResolvedValue([
      { id: 8, url: 'https://example.test/saved.mp4', mime_type: 'video/mp4' },
    ])
    const wrapper = mount(VideoUpload, { props: { entryId: 21, revision: 4 } })

    await flushPromises()

    expect(listEntryMedia).toHaveBeenCalledWith(21)
    expect(wrapper.get('[data-video-preview]').attributes('src')).toBe('https://example.test/saved.mp4')
    expect(wrapper.get('[data-video-picker]').text()).toContain('更换视频')
  })

  it('shows the uploaded video immediately after it is bound to the entry', async () => {
    uploadMedia.mockResolvedValue({ id: 9, url: 'https://example.test/new.mp4' })
    setPrimaryVideo.mockResolvedValue({ revision: 5, media_id: 9 })
    const wrapper = mount(VideoUpload, { props: { entryId: 21, revision: 4 } })
    const input = wrapper.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [new File(['video'], 'new.mp4', { type: 'video/mp4' })] })

    await input.trigger('change')
    await flushPromises()

    expect(setPrimaryVideo).toHaveBeenCalledWith(21, { media_id: 9, revision: 4 })
    expect(wrapper.get('[data-video-preview]').attributes('src')).toBe('https://example.test/new.mp4')
    expect(wrapper.emitted('revision')).toEqual([[5]])
    expect(uploadMedia).toHaveBeenCalledOnce()
  })
})
