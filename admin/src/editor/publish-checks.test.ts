import { describe, expect, it } from 'vitest'

import { getPublishChecks } from './publish-checks'

const complete = {
  title: '标题', slug: 'title', content_md: '正文', summary: '摘要', category_id: 1,
  cover_url: 'https://example.com/cover.jpg', happened_at: '2026-08-27T00:00:00Z',
}

describe('getPublishChecks', () => {
  it('blocks publication when the title, slug, or body is empty', () => {
    const checks = getPublishChecks({ ...complete, title: ' ', slug: '', content_md: '' })
    expect(checks.blockers.map((check) => check.field)).toEqual(['title', 'slug', 'content_md'])
  })

  it('reports optional omissions as ordered reminders', () => {
    const checks = getPublishChecks({ ...complete, summary: '', category_id: 0, cover_url: '', happened_at: null })
    expect(checks.reminders.map((check) => check.field)).toEqual(['summary', 'category_id', 'cover_url', 'happened_at'])
  })
})
