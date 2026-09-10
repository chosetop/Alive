import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import VideoUpload from './VideoUpload.vue'

vi.mock('../../media', () => ({
  uploadMedia: vi.fn(),
}))

vi.mock('../../api', () => ({
  mediaApi: { setPrimaryVideo: vi.fn() },
}))

describe('VideoUpload', () => {
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
})
