import { describe, expect, it } from 'vitest'

import VideoHomePreview from './VideoHomePreview.vue'
import { renderComponent } from '~/test-support/render'

describe('VideoHomePreview', () => {
  it('shows the latest three videos and a browse-all link', async () => {
    const items = Array.from({ length: 4 }, (_, index) => ({
      world: 'video' as const,
      kind: '',
      title: `影像 ${index + 1}`,
      slug: `video-${index + 1}`,
      summary: '',
      cover_url: '',
      meta: {},
      word_count: 0,
      category: null,
      happened_at: null,
      published_at: null,
    }))

    const html = await renderComponent(VideoHomePreview, { items })

    expect(html).toContain('影像')
    expect(html).toContain('href="/videos"')
    expect(html).toContain('href="/videos/video-1"')
    expect(html).toContain('href="/videos/video-2"')
    expect(html).toContain('href="/videos/video-3"')
    expect(html).not.toContain('href="/videos/video-4"')
  })
})
